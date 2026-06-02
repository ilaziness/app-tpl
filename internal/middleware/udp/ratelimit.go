// Package udp provides UDP-specific middleware.
package udp

import (
	"golang.org/x/time/rate"

	"go.uber.org/zap"
)

// RateLimitMiddleware provides rate limiting for UDP packets.
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

// Allow checks if a packet is allowed under the rate limit.
func (m *RateLimitMiddleware) Allow(remoteAddr string) bool {
	if !m.enabled || m.limiter == nil {
		return true
	}

	allowed := m.limiter.Allow()
	if !allowed {
		m.logger.Warn("Rate limit exceeded",
			zap.String("remote_addr", remoteAddr))
	}
	return allowed
}
