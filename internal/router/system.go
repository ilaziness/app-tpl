package router

import (
	httphandler "github.com/example/app-tpl/internal/handler/http"
	"github.com/gin-gonic/gin"
)

func registerSystemRoutes(engine *gin.Engine, health *httphandler.HealthHandler) {
	engine.GET(PathHealth, health.Health)
	engine.GET(PathReadiness, health.Readiness)
	engine.GET(PathLiveness, health.Liveness)
	engine.GET(PathVersion, health.Version)
}
