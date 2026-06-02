package router

import (
	httphandler "github.com/example/app-tpl/internal/handler/http"
	httpmiddleware "github.com/example/app-tpl/internal/middleware/http"
	"github.com/gin-gonic/gin"
)

func registerInternalRoutes(engine *gin.Engine, h *Handlers) {
	v1 := engine.Group(PathInternalV1)
	v1.Use(httpmiddleware.InternalServiceAuth(h.InternalServiceKey))

	registerInternalUserRoutes(v1, h.User)
}

func registerInternalUserRoutes(v1 *gin.RouterGroup, user *httphandler.UserHandler) {
	users := v1.Group("/users")
	users.GET("/:id", user.GetUser)
}
