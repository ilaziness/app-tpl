// Package server provides TCP server implementation.
// This is an optional module for TCP service support.
package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/ilaziness/app-tpl/internal/protocol"
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// TCPServer handles TCP connections.
type TCPServer struct {
	listener       net.Listener
	logger         *zap.Logger
	cfg            *config.Config
	enabled        bool
	connections    map[string]*types.Connection
	connMutex      sync.RWMutex
	connIDCounter  uint64
	protocol       *protocol.CustomProtocol
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	wg             sync.WaitGroup
	handlers       map[uint8]types.TCPHandler
	handlerMutex   sync.RWMutex
}

// NewTCPServer creates a new TCP server instance.
func NewTCPServer(cfg *config.Config, logger *zap.Logger, echoHandler types.TCPHandler) *TCPServer {
	tcpServer := &TCPServer{
		enabled:     cfg.TCP.Enabled,
		logger:      logger,
		cfg:         cfg,
		connections: make(map[string]*types.Connection),
		handlers:    make(map[uint8]types.TCPHandler),
	}

	if !tcpServer.enabled {
		logger.Info("TCP server is disabled")
		return tcpServer
	}

	// Initialize protocol codec
	var codec protocol.Codec
	if cfg.TCP.Codec == "binary" {
		codec = protocol.NewBinaryCodec()
	} else {
		codec = protocol.NewJSONCodec()
	}
	tcpServer.protocol = protocol.NewCustomProtocol(codec)

	// Initialize shutdown context
	tcpServer.shutdownCtx, tcpServer.shutdownCancel = context.WithCancel(context.Background())

	// Register default echo handler for all message types
	tcpServer.RegisterHandler(protocol.MessageTypeRequest, echoHandler)
	tcpServer.RegisterHandler(protocol.MessageTypeResponse, echoHandler)
	tcpServer.RegisterHandler(protocol.MessageTypeHeartbeat, echoHandler)

	return tcpServer
}

// Start starts the TCP server.
func (s *TCPServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.TCP.Host, s.cfg.TCP.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.logger.Error("Failed to start TCP server", zap.Error(err))
		return err
	}

	s.listener = listener

	s.logger.Info("TCP server started", zap.String("addr", addr))

	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// Stop stops the TCP server gracefully.
func (s *TCPServer) Stop(ctx context.Context) error {
	if !s.enabled || s.listener == nil {
		return nil
	}

	s.logger.Info("Stopping TCP server")

	// Close listener to stop accepting new connections
	if err := s.listener.Close(); err != nil {
		s.logger.Error("Error closing TCP listener", zap.Error(err))
	}

	// Cancel shutdown context to signal all connections to close
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}

	// Wait for all connections to close with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	timeout := time.Duration(s.cfg.TCP.ShutdownTimeout) * time.Second
	if err := waitShutdown(ctx, timeout, done); err != nil {
		s.logger.Warn("TCP server shutdown timeout, forcing close", zap.Error(err))
		return err
	}
	s.logger.Info("TCP server stopped gracefully")
	return nil
}

// acceptConnections accepts incoming TCP connections.
func (s *TCPServer) acceptConnections() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownCtx.Done():
				// Server is shutting down
				return
			default:
				s.logger.Error("Error accepting connection", zap.Error(err))
				continue
			}
		}

		// Check connection limit
		s.connMutex.RLock()
		connCount := len(s.connections)
		s.connMutex.RUnlock()

		if connCount >= s.cfg.TCP.MaxConnections {
			s.logger.Warn("Connection limit reached, closing connection",
				zap.String("remote_addr", conn.RemoteAddr().String()))
			closeTCPConn(conn)
			continue
		}

		// Handle connection
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single TCP connection.
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer closeTCPConn(conn)

	// Generate connection ID
	connID := fmt.Sprintf("%d", atomic.AddUint64(&s.connIDCounter, 1))

	// Set read/write timeouts
	readTimeout := time.Duration(s.cfg.TCP.ReadTimeout) * time.Second
	writeTimeout := time.Duration(s.cfg.TCP.WriteTimeout) * time.Second
	applyConnDeadlines(conn, readTimeout, writeTimeout)

	// Create connection object
	connection := &types.Connection{
		ID:           connID,
		Conn:         conn,
		RemoteAddr:   conn.RemoteAddr().String(),
		LastActive:   time.Now(),
		CreatedAt:    time.Now(),
		TimeoutCount: 0,
		WriteTimeout: writeTimeout,
	}

	// Add to connections map
	s.connMutex.Lock()
	s.connections[connID] = connection
	s.connMutex.Unlock()

	s.logger.Info("New connection established",
		zap.String("conn_id", connID),
		zap.String("remote_addr", connection.RemoteAddr))

	// Start heartbeat checker
	heartbeatDone := make(chan struct{})
	go s.heartbeatChecker(connection, heartbeatDone)

	// Handle connection data
	defer func() {
		close(heartbeatDone)
		s.connMutex.Lock()
		delete(s.connections, connID)
		s.connMutex.Unlock()
		s.logger.Info("Connection closed",
			zap.String("conn_id", connID),
			zap.String("remote_addr", connection.RemoteAddr))
	}()

	// Read and process messages
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		default:
			n, err := conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					s.connMutex.Lock()
					connection.TimeoutCount++
					timeoutCount := connection.TimeoutCount
					s.connMutex.Unlock()

					s.logger.Debug("Connection read timeout",
						zap.String("conn_id", connID),
						zap.Int("timeout_count", timeoutCount))

					// Close connection after max consecutive timeouts
					maxTimeoutCount := s.cfg.TCP.MaxTimeoutCount
					if maxTimeoutCount <= 0 {
						maxTimeoutCount = 10 // default
					}
					if timeoutCount >= maxTimeoutCount {
						s.logger.Warn("Connection closed due to excessive timeouts",
							zap.String("conn_id", connID),
							zap.Int("timeout_count", timeoutCount))
						return
					}
					continue
				}
				s.logger.Debug("Connection read error",
					zap.String("conn_id", connID),
					zap.Error(err))
				return
			}

			// Update last active time and reset timeout count
			s.connMutex.Lock()
			connection.LastActive = time.Now()
			connection.TimeoutCount = 0
			s.connMutex.Unlock()

			// Reset read/write deadlines before processing
			writeTimeout := connection.WriteTimeout
			if writeTimeout <= 0 {
				writeTimeout = readTimeout
			}
			applyConnDeadlines(conn, readTimeout, writeTimeout)

			// Check minimum protocol header size
			if n < protocol.ProtocolHeaderSize {
				s.logger.Warn("Received data too small for protocol header",
					zap.String("conn_id", connID),
					zap.Int("bytes", n))
				continue
			}

			// Parse protocol header to get message type
			header, err := protocol.ParseHeader(buf[:n])
			if err != nil {
				s.logger.Error("Failed to parse protocol header",
					zap.String("conn_id", connID),
					zap.Error(err))
				continue
			}

			// Get handler based on message type
			s.handlerMutex.RLock()
			handler, ok := s.handlers[header.Type]
			s.handlerMutex.RUnlock()

			if !ok {
				s.logger.Debug("No handler registered for message type",
					zap.String("conn_id", connID),
					zap.Uint8("message_type", header.Type))
			}

			if handler == nil {
				s.logger.Warn("No handler available for message type",
					zap.String("conn_id", connID),
					zap.Uint8("message_type", header.Type))
				continue
			}

			// Call handler
			responseData, err := handler.Handle(connection, buf[:n])
			if err != nil {
				s.logger.Error("Handler error",
					zap.String("conn_id", connID),
					zap.Error(err))
				continue
			}

			// Send response via connection.Write to serialize with concurrent writers (e.g. broadcastLoop)
			if len(responseData) > 0 {
				_, err = connection.Write(responseData)
				if err != nil {
					s.logger.Error("Failed to write response",
						zap.String("conn_id", connID),
						zap.Error(err))
					return
				}
			}
		}
	}
}

// heartbeatChecker checks heartbeat for a connection.
func (s *TCPServer) heartbeatChecker(conn *types.Connection, done <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(s.cfg.TCP.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-s.shutdownCtx.Done():
			return
		case <-ticker.C:
			s.connMutex.RLock()
			lastActive := conn.LastActive
			s.connMutex.RUnlock()

			heartbeatTimeout := time.Duration(s.cfg.TCP.HeartbeatTimeout) * time.Second
			if time.Since(lastActive) > heartbeatTimeout {
				s.logger.Warn("Connection heartbeat timeout, closing",
					zap.String("conn_id", conn.ID),
					zap.String("remote_addr", conn.RemoteAddr))
				closeTCPConn(conn.Conn)
				return
			}
		}
	}
}

// Enabled returns whether the TCP server is enabled.
func (s *TCPServer) Enabled() bool {
	return s.enabled
}

// Addr returns the server address.
func (s *TCPServer) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// GetConnectionCount returns the current number of active connections.
func (s *TCPServer) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return len(s.connections)
}

// GetConnection returns a connection by ID.
func (s *TCPServer) GetConnection(connID string) (*types.Connection, bool) {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	conn, ok := s.connections[connID]
	return conn, ok
}

// RegisterHandler registers a handler for a specific message type.
func (s *TCPServer) RegisterHandler(messageType uint8, handler types.TCPHandler) {
	s.handlerMutex.Lock()
	defer s.handlerMutex.Unlock()
	s.handlers[messageType] = handler
	s.logger.Info("TCP handler registered", zap.Uint8("message_type", messageType))
}

func closeTCPConn(conn net.Conn) {
	if conn == nil {
		return
	}
	_ = conn.Close()
}

func applyConnDeadlines(conn net.Conn, readTimeout, writeTimeout time.Duration) {
	if conn == nil {
		return
	}
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
}
