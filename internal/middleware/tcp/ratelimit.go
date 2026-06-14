// Package tcp provides TCP-specific middleware.
package tcp

import (
	"golang.org/x/time/rate"

	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// RateLimitMiddleware provides rate limiting for TCP connections.
type RateLimitMiddleware struct {
	logger  *zap.Logger
	limiter *rate.Limiter
	enabled bool
}

// NewRateLimitMiddleware creates a new rate limiting middleware.
func NewRateLimitMiddleware(logger *zap.Logger, rps int, enabled bool) *RateLimitMiddleware {
	var limiter *rate.Limiter
	if enabled && rps > 0 {
		limiter = rate.NewLimiter(rate.Limit(rps), rps)
	}

	return &RateLimitMiddleware{
		logger:  logger,
		limiter: limiter,
		enabled: enabled,
	}
}

// Allow checks if a request is allowed under the rate limit.
func (m *RateLimitMiddleware) Allow(conn *types.Connection) bool {
	if !m.enabled || m.limiter == nil {
		return true
	}

	allowed := m.limiter.Allow()
	if !allowed {
		m.logger.Warn("Rate limit exceeded",
			zap.String("conn_id", conn.ID),
			zap.String("remote_addr", conn.RemoteAddr))
	}
	return allowed
}
