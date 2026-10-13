package router

import (
	"github.com/gin-gonic/gin"
	httphandler "github.com/ilaziness/app-tpl/internal/handler/http"
)

func registerClientRoutes(engine *gin.Engine, h *Handlers) {
	v1 := engine.Group(PathClientV1)
	registerClientUserRoutes(v1, h.User)

	// v2 reserved for breaking API changes. Prefer one active version in the sample;
	// use httpmiddleware.Deprecated when retiring a v1 route.
	_ = engine.Group(PathClientV2)
}

func registerClientUserRoutes(v1 *gin.RouterGroup, user *httphandler.UserHandler) {
	users := v1.Group("/users")
	{
		users.GET("/:id", user.GetUser)
		users.POST("", user.CreateUser)
		users.PUT("/:id", user.UpdateUser)
		users.DELETE("/:id", user.DeleteUser)
		users.GET("", user.ListUsers)
	}
}
