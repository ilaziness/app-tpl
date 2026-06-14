// Package tcp provides TCP-specific middleware.
package tcp

import (
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// AuthMiddleware provides connection authentication.
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

// Authenticate checks if a connection is authenticated.
func (m *AuthMiddleware) Authenticate(conn *types.Connection, authToken string) bool {
	if m.token == "" {
		// No authentication required
		return true
	}

	if authToken == m.token {
		return true
	}

	m.logger.Warn("Authentication failed",
		zap.String("conn_id", conn.ID),
		zap.String("remote_addr", conn.RemoteAddr))
	return false
}
