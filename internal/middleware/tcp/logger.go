// Package tcp provides TCP-specific middleware.
package tcp

import (
	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// LoggerMiddleware provides logging for TCP connections.
type LoggerMiddleware struct {
	logger *zap.Logger
}

// NewLoggerMiddleware creates a new logging middleware.
func NewLoggerMiddleware(logger *zap.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger,
	}
}

// LogConnection logs connection events.
func (m *LoggerMiddleware) LogConnection(conn *types.Connection, event string) {
	m.logger.Info("TCP connection event",
		zap.String("event", event),
		zap.String("conn_id", conn.ID),
		zap.String("remote_addr", conn.RemoteAddr))
}

// LogMessage logs message events.
func (m *LoggerMiddleware) LogMessage(conn *types.Connection, msgType string, bytes int) {
	m.logger.Debug("TCP message",
		zap.String("conn_id", conn.ID),
		zap.String("remote_addr", conn.RemoteAddr),
		zap.String("msg_type", msgType),
		zap.Int("bytes", bytes))
}
