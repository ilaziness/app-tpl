package router

import (
	"github.com/gin-gonic/gin"
	httpmiddleware "github.com/ilaziness/app-tpl/internal/middleware/http"
)

func registerAdminRoutes(engine *gin.Engine, h *Handlers) {
	v1 := engine.Group(PathAdminV1)
	if h.RequireAdminAuth {
		v1.Use(httpmiddleware.RequireAuth())
	}

	// Sample user CRUD is registered on the client API surface only.
	// Add admin-specific handlers here when extending by API face (admin/client).
	_ = v1
}
