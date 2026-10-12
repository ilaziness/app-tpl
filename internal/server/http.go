package server

import (
	"context"
	"crypto/tls"
	"fmt"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	redisrate "github.com/go-redis/redis_rate/v10"
	"github.com/ilaziness/app-tpl/internal/auth"
	"github.com/ilaziness/app-tpl/internal/config"
	errcode "github.com/ilaziness/app-tpl/internal/errcode"
	"github.com/ilaziness/app-tpl/internal/metrics"
	httpmiddleware "github.com/ilaziness/app-tpl/internal/middleware/http"
	"github.com/ilaziness/app-tpl/internal/response"
	"github.com/ilaziness/app-tpl/internal/router"
	"github.com/ilaziness/app-tpl/internal/tracing"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type HTTPServer struct {
	server   *stdhttp.Server
	router   *gin.Engine
	addr     string
	enabled  bool
	tls      bool
	certFile string
	keyFile  string
}

// NewHTTPServer creates the HTTP server.
// accessLogger is used for Gin access logs (stdout-only recommended).
// logger is used for recovery / rate-limit warnings.
// redisClient is optional; when rate_limit.store=redis and client is non-nil, Redis store is used.
func NewHTTPServer(cfg *config.Config, logger *zap.Logger, accessLogger *zap.Logger, h *router.Handlers, m *metrics.Metrics, jwtMgr *auth.JWTManager, redisClient *redis.Client) (*HTTPServer, error) {
	httpServer := &HTTPServer{
		enabled: cfg.HTTP.Enabled,
	}

	if !cfg.HTTP.Enabled {
		return httpServer, nil
	}

	if accessLogger == nil {
		accessLogger = logger
	}

	ginRouter := gin.New()

	// Apply core middlewares
	ginRouter.Use(httpmiddleware.RequestID())
	ginRouter.Use(httpmiddleware.SecurityHeaders())
	ginRouter.Use(httpmiddleware.Logger(accessLogger, router.SystemPaths...))
	ginRouter.Use(httpmiddleware.Recovery(logger))
	ginRouter.Use(httpmiddleware.CORS())
	ginRouter.Use(httpmiddleware.BodySizeLimit(0)) // default 10MB

	// Register metrics endpoint before gzip middleware (metrics should not be compressed)
	if cfg.Metrics.Enabled {
		ginRouter.GET(cfg.Metrics.Path, metrics.Handler(m, cfg.Metrics.Path))
	}

	ginRouter.Use(httpmiddleware.Compress())

	// Apply tracing middleware if enabled
	if cfg.Tracing.Enabled {
		ginRouter.Use(tracing.Middleware(cfg, router.SystemPaths...))
		ginRouter.Use(httpmiddleware.InjectTraceID())
	}

	// Apply metrics middleware if enabled
	if cfg.Metrics.Enabled {
		ginRouter.Use(metrics.Middleware(m, router.SystemPaths...))
	}

	// Apply JWT middleware if manager is provided
	if jwtMgr != nil {
		skipPaths := router.MergeJWTSkipPaths(cfg.JWT.SkipPaths)
		ginRouter.Use(httpmiddleware.JWTAuth(jwtMgr, skipPaths...))
	}

	// Apply rate limit middleware if enabled (must be after JWT for per-user rate limiting)
	if cfg.RateLimit.Enabled {
		var store httpmiddleware.RateLimitStore
		if cfg.RateLimit.Store == "redis" && redisClient != nil {
			limiter := redisrate.NewLimiter(redisClient)
			store = httpmiddleware.NewRedisRateLimitStore(limiter)
			logger.Info("rate limit: using Redis store")
		} else {
			if cfg.RateLimit.Store == "redis" {
				logger.Warn("rate limit: redis store requested but redis client unavailable, falling back to memory")
			}
			store = httpmiddleware.NewMemoryRateLimitStore(10000)
		}
		rateCfg := httpmiddleware.RateLimitConfig{
			Enabled:   true,
			GlobalRPS: cfg.RateLimit.GlobalRPS,
			IPRPS:     cfg.RateLimit.IPRPS,
			UserRPS:   cfg.RateLimit.UserRPS,
			Store:     store,
			SkipPaths: router.RateLimitSkipPaths(),
		}
		ginRouter.Use(httpmiddleware.RateLimit(rateCfg, logger))
	}

	// Always-on login rate limit (IP-based) for future auth routes.
	loginStore := httpmiddleware.NewMemoryRateLimitStore(10000)
	ginRouter.Use(func(c *gin.Context) {
		if c.Request.Method == stdhttp.MethodPost && (c.Request.URL.Path == router.PathAdminV1+"/auth/login" || c.Request.URL.Path == router.PathClientV1+"/auth/login") {
			ok, _, _, err := loginStore.Allow(c.Request.Context(), "login:"+c.ClientIP(), 5)
			if err == nil && !ok {
				response.Error(c, errcode.TooManyRequests)
				c.Abort()
				return
			}
		}
		c.Next()
	})

	// Register routes
	if err := router.RegisterRoutes(ginRouter, h); err != nil {
		return nil, fmt.Errorf("register routes: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	server := &stdhttp.Server{
		Addr:         addr,
		Handler:      ginRouter,
		ReadTimeout:  time.Duration(cfg.HTTP.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTP.WriteTimeout) * time.Second,
	}

	// Configure TLS if enabled
	if cfg.HTTP.TLS.Enabled {
		minVersion := uint16(tls.VersionTLS13)
		if cfg.HTTP.TLS.MinVersion == "1.2" {
			minVersion = tls.VersionTLS12
		}
		server.TLSConfig = &tls.Config{ //nolint:gosec // MinVersion is set from config, default TLS 1.3
			MinVersion:               minVersion,
			PreferServerCipherSuites: true,
		}
		httpServer.tls = true
		httpServer.certFile = cfg.HTTP.TLS.CertFile
		httpServer.keyFile = cfg.HTTP.TLS.KeyFile
	}

	httpServer.server = server
	httpServer.router = ginRouter
	httpServer.addr = addr

	return httpServer, nil
}

func (s *HTTPServer) Addr() string {
	return s.addr
}

func (s *HTTPServer) Router() *gin.Engine {
	return s.router
}

func (s *HTTPServer) Enabled() bool {
	return s.enabled
}

// Serve starts the HTTP server. Returns when the server stops or fails.
func (s *HTTPServer) Serve() error {
	if !s.enabled || s.server == nil {
		return nil
	}
	if s.tls {
		return s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if !s.enabled || s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
