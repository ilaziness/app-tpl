// Package tcp provides TCP-specific middleware.
package tcp

import (
	"net"
	"time"

	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// TimeoutMiddleware provides timeout control for TCP connections.
type TimeoutMiddleware struct {
	logger       *zap.Logger
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewTimeoutMiddleware creates a new timeout middleware.
func NewTimeoutMiddleware(logger *zap.Logger, readTimeout, writeTimeout int) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		logger:       logger,
		readTimeout:  time.Duration(readTimeout) * time.Second,
		writeTimeout: time.Duration(writeTimeout) * time.Second,
	}
}

// SetTimeouts sets read and write timeouts for a connection.
func (m *TimeoutMiddleware) SetTimeouts(conn *types.Connection) error {
	tcpConn, ok := conn.Conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	if m.readTimeout > 0 {
		if err := tcpConn.SetReadDeadline(time.Now().Add(m.readTimeout)); err != nil {
			m.logger.Error("Failed to set read deadline",
				zap.String("conn_id", conn.ID),
				zap.Error(err))
			return err
		}
	}

	if m.writeTimeout > 0 {
		if err := tcpConn.SetWriteDeadline(time.Now().Add(m.writeTimeout)); err != nil {
			m.logger.Error("Failed to set write deadline",
				zap.String("conn_id", conn.ID),
				zap.Error(err))
			return err
		}
	}

	return nil
}
