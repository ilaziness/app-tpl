// Package server provides UDP server implementation.
// This is an optional module for UDP service support.
package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/ilaziness/app-tpl/internal/protocol"
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// UDPServer handles UDP packets.
type UDPServer struct {
	conn           *net.UDPConn
	logger         *zap.Logger
	cfg            *config.Config
	enabled        bool
	sessions       map[string]*types.Session
	sessionMutex   sync.RWMutex
	protocol       *protocol.CustomProtocol
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	wg             sync.WaitGroup
	packetChan     chan *types.UDPPacket
	sessionCleanup *time.Ticker
	handlers       map[uint8]types.UDPHandler
	handlerMutex   sync.RWMutex
}

// NewUDPServer creates a new UDP server instance.
func NewUDPServer(cfg *config.Config, logger *zap.Logger, handler types.UDPHandler) *UDPServer {
	udpServer := &UDPServer{
		enabled:    cfg.UDP.Enabled,
		logger:     logger,
		cfg:        cfg,
		sessions:   make(map[string]*types.Session),
		packetChan: make(chan *types.UDPPacket, 1000),
		handlers:   make(map[uint8]types.UDPHandler),
	}

	if !udpServer.enabled {
		logger.Info("UDP server is disabled")
		return udpServer
	}

	// Initialize protocol codec
	var codec protocol.Codec
	if cfg.UDP.Codec == "binary" {
		codec = protocol.NewBinaryCodec()
	} else {
		codec = protocol.NewJSONCodec()
	}
	udpServer.protocol = protocol.NewCustomProtocol(codec)

	// Initialize shutdown context
	udpServer.shutdownCtx, udpServer.shutdownCancel = context.WithCancel(context.Background())

	// Register application handler for all protocol message types
	udpServer.RegisterHandler(protocol.MessageTypeRequest, handler)
	udpServer.RegisterHandler(protocol.MessageTypeResponse, handler)
	udpServer.RegisterHandler(protocol.MessageTypeHeartbeat, handler)

	return udpServer
}

// Start starts the UDP server.
func (s *UDPServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.UDP.Host, s.cfg.UDP.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		s.logger.Error("Failed to resolve UDP address", zap.Error(err))
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		s.logger.Error("Failed to start UDP server", zap.Error(err))
		return err
	}

	s.conn = conn

	s.logger.Info("UDP server started", zap.String("addr", addr))

	// Start worker pool
	workerPoolSize := s.cfg.UDP.WorkerPoolSize
	if workerPoolSize <= 0 {
		workerPoolSize = 4 // default worker pool size
		s.logger.Info("Using default worker pool size", zap.Int("workers", workerPoolSize))
	}
	for i := 0; i < workerPoolSize; i++ {
		s.wg.Add(1)
		go s.packetWorker(i)
	}

	// Start packet receiver
	s.wg.Add(1)
	go s.receivePackets()

	// Start session cleanup goroutine
	cleanupInterval := s.cfg.UDP.SessionCleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 300 // default 5 minutes in seconds
	}
	s.sessionCleanup = time.NewTicker(time.Duration(cleanupInterval) * time.Second)
	s.wg.Add(1)
	go s.sessionCleanupLoop()

	return nil
}

// Stop stops the UDP server gracefully.
func (s *UDPServer) Stop(ctx context.Context) error {
	if !s.enabled || s.conn == nil {
		return nil
	}

	s.logger.Info("Stopping UDP server")

	// Cancel shutdown context to signal all workers to stop
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}

	// Close the UDP connection to unblock receivePackets which may be blocked on ReadFromUDP.
	// receivePackets and all packetWorkers observe shutdownCtx.Done() and will exit cleanly.
	if err := s.conn.Close(); err != nil {
		s.logger.Error("Error closing UDP connection during stop", zap.Error(err))
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	timeout := time.Duration(s.cfg.UDP.ShutdownTimeout) * time.Second
	if err := waitShutdown(ctx, timeout, done); err != nil {
		s.logger.Warn("UDP server shutdown timeout, forcing close", zap.Error(err))
		return err
	}
	s.logger.Info("UDP server stopped gracefully")
	return nil
}

// receivePackets receives UDP packets.
func (s *UDPServer) receivePackets() {
	defer s.wg.Done()

	readBufferSize := s.cfg.UDP.ReadBufferSize
	if readBufferSize <= 0 {
		readBufferSize = 4096
	}
	buf := make([]byte, readBufferSize)

	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		default:
			n, remoteAddr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				// Check shutdown first: conn.Close() in Stop() causes a read error
				// that should be treated as a clean exit, not a real error.
				select {
				case <-s.shutdownCtx.Done():
					return
				default:
				}
				s.logger.Error("Error reading UDP packet", zap.Error(err))
				continue
			}

			// Copy data to avoid buffer reuse issues
			data := make([]byte, n)
			copy(data, buf[:n])

			// Create packet
			packet := &types.UDPPacket{
				Data:       data,
				RemoteAddr: remoteAddr,
				LocalAddr:  s.conn.LocalAddr(),
				Timestamp:  time.Now(),
			}

			// Update or create session
			s.updateSession(remoteAddr.String())

			// Send to worker pool
			select {
			case s.packetChan <- packet:
			default:
				s.logger.Warn("Packet channel full, dropping packet",
					zap.String("remote_addr", remoteAddr.String()))
			}
		}
	}
}

// packetWorker processes packets from the channel.
func (s *UDPServer) packetWorker(workerID int) {
	defer s.wg.Done()

	s.logger.Debug("UDP packet worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		case packet, ok := <-s.packetChan: //nolint:staticcheck
			if !ok {
				return
			}

			// Check minimum protocol header size
			if len(packet.Data) < protocol.ProtocolHeaderSize {
				s.logger.Warn("Received data too small for protocol header",
					zap.String("remote_addr", packet.RemoteAddr.String()),
					zap.Int("bytes", len(packet.Data)))
				continue
			}

			// Parse protocol header to get message type
			header, err := protocol.ParseHeader(packet.Data)
			if err != nil {
				s.logger.Error("Failed to parse protocol header",
					zap.String("remote_addr", packet.RemoteAddr.String()),
					zap.Error(err))
				continue
			}

			// Get handler based on message type
			s.handlerMutex.RLock()
			handler, ok := s.handlers[header.Type]
			s.handlerMutex.RUnlock()

			if !ok {
				s.logger.Debug("No handler registered for message type",
					zap.String("remote_addr", packet.RemoteAddr.String()),
					zap.Uint8("message_type", header.Type))
			}

			if handler == nil {
				s.logger.Warn("No handler available for message type",
					zap.String("remote_addr", packet.RemoteAddr.String()),
					zap.Uint8("message_type", header.Type))
				continue
			}

			// Call handler
			responseData, err := handler.Handle(packet)
			if err != nil {
				s.logger.Error("Handler error",
					zap.String("remote_addr", packet.RemoteAddr.String()),
					zap.Error(err))
				continue
			}

			// Send response
			if len(responseData) > 0 {
				if udpAddr, ok := packet.RemoteAddr.(*net.UDPAddr); ok {
					_ = s.SendPacket(responseData, udpAddr)
				} else {
					s.logger.Warn("Failed to send UDP response: invalid address type",
						zap.String("remote_addr", packet.RemoteAddr.String()))
				}
			}
		}
	}
}

// updateSession updates or creates a session for a remote address.
func (s *UDPServer) updateSession(remoteAddr string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[remoteAddr]; exists {
		session.LastActive = time.Now()
	} else {
		s.sessions[remoteAddr] = &types.Session{
			RemoteAddr: remoteAddr,
			LastActive: time.Now(),
			CreatedAt:  time.Now(),
		}
	}
}

// sessionCleanupLoop periodically removes inactive sessions.
func (s *UDPServer) sessionCleanupLoop() {
	defer s.wg.Done()
	defer s.sessionCleanup.Stop()

	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		case <-s.sessionCleanup.C:
			s.cleanupInactiveSessions()
		}
	}
}

// cleanupInactiveSessions removes sessions inactive for more than threshold.
func (s *UDPServer) cleanupInactiveSessions() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	inactiveThresholdSeconds := s.cfg.UDP.SessionInactiveThreshold
	if inactiveThresholdSeconds <= 0 {
		inactiveThresholdSeconds = 600 // default 10 minutes in seconds
	}
	inactiveThreshold := time.Duration(inactiveThresholdSeconds) * time.Second
	now := time.Now()

	for addr, session := range s.sessions {
		if now.Sub(session.LastActive) > inactiveThreshold {
			delete(s.sessions, addr)
			s.logger.Debug("Removed inactive session",
				zap.String("remote_addr", addr),
				zap.Duration("inactive_duration", now.Sub(session.LastActive)))
		}
	}
}

// GetSessionCount returns the current number of active sessions.
func (s *UDPServer) GetSessionCount() int {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	return len(s.sessions)
}

// GetSession returns a session by remote address.
func (s *UDPServer) GetSession(remoteAddr string) (*types.Session, bool) {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	session, ok := s.sessions[remoteAddr]
	return session, ok
}

// Enabled returns whether the UDP server is enabled.
func (s *UDPServer) Enabled() bool {
	return s.enabled
}

// Addr returns the server address.
func (s *UDPServer) Addr() string {
	if s.conn == nil {
		return ""
	}
	return s.conn.LocalAddr().String()
}

// SendPacket sends a packet to a remote address.
func (s *UDPServer) SendPacket(data []byte, remoteAddr *net.UDPAddr) error {
	if s.conn == nil {
		return fmt.Errorf("UDP connection not initialized")
	}

	_, err := s.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		s.logger.Error("Error sending UDP packet",
			zap.String("remote_addr", remoteAddr.String()),
			zap.Error(err))
		return err
	}

	return nil
}

// RegisterHandler registers a handler for a specific message type.
func (s *UDPServer) RegisterHandler(messageType uint8, handler types.UDPHandler) {
	s.handlerMutex.Lock()
	defer s.handlerMutex.Unlock()
	s.handlers[messageType] = handler
	s.logger.Info("UDP handler registered", zap.Uint8("message_type", messageType))
}
