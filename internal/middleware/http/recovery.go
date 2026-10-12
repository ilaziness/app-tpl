package http

import (
	"runtime/debug"

	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	errcode "github.com/ilaziness/app-tpl/internal/errcode"
	"github.com/ilaziness/app-tpl/internal/response"
	"go.uber.org/zap"
)

// Recovery returns a middleware that recovers from panics, logs them with stack,
// and returns a unified InternalError response instead of an empty 200 body.
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return ginzap.CustomRecoveryWithZap(logger, true, func(c *gin.Context, recovered any) {
		logger.Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("request_id", requestid.Get(c)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.ByteString("stack", debug.Stack()),
		)
		response.Error(c, errcode.InternalError)
		c.Abort()
	})
}
