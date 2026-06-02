// Package udp provides UDP-specific middleware.
package udp

import (
	"go.uber.org/zap"
)

// LoggerMiddleware provides logging for UDP packets.
type LoggerMiddleware struct {
	logger *zap.Logger
}

// NewLoggerMiddleware creates a new logging middleware.
func NewLoggerMiddleware(logger *zap.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger,
	}
}

// LogPacket logs packet events.
func (m *LoggerMiddleware) LogPacket(remoteAddr string, msgType string, bytes int) {
	m.logger.Debug("UDP packet",
		zap.String("remote_addr", remoteAddr),
		zap.String("msg_type", msgType),
		zap.Int("bytes", bytes))
}
