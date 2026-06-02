// Package udp provides UDP-specific middleware.
package udp

import (
	"go.uber.org/zap"
)

// AuthMiddleware provides packet authentication.
type AuthMiddleware struct {
	logger *zap.Logger
	token  string
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(logger *zap.Logger, token string) *AuthMiddleware {
	return &AuthMiddleware{
		logger: logger,
		token:  token,
	}
}

// Authenticate checks if a packet is authenticated.
func (m *AuthMiddleware) Authenticate(remoteAddr string, authToken string) bool {
	if m.token == "" {
		// No authentication required
		return true
	}

	if authToken == m.token {
		return true
	}

	m.logger.Warn("Authentication failed",
		zap.String("remote_addr", remoteAddr))
	return false
}
