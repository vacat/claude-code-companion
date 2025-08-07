package web

import (
	"fmt"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/logger"

	"github.com/gin-gonic/gin"
)

type AdminServer struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	router          *gin.Engine
	configFilePath  string
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, log *logger.Logger, configFilePath string) *AdminServer {
	server := &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		configFilePath:  configFilePath,
	}

	server.setupRoutes()
	return server
}

func (s *AdminServer) setupRoutes() {
	gin.SetMode(gin.ReleaseMode)
	s.router = gin.New()
	s.router.Use(gin.Recovery())

	s.router.LoadHTMLGlob("web/templates/*")
	s.router.Static("/static", "web/static")

	s.router.GET("/admin/", s.handleDashboard)
	s.router.GET("/admin/endpoints", s.handleEndpointsPage)
	s.router.GET("/admin/logs", s.handleLogsPage)
	s.router.GET("/admin/settings", s.handleSettingsPage)

	api := s.router.Group("/admin/api")
	{
		api.GET("/endpoints", s.handleGetEndpoints)
		api.PUT("/endpoints", s.handleUpdateEndpoints)
		api.POST("/endpoints", s.handleCreateEndpoint)
		api.PUT("/endpoints/:id", s.handleUpdateEndpoint)
		api.DELETE("/endpoints/:id", s.handleDeleteEndpoint)
		api.POST("/endpoints/reorder", s.handleReorderEndpoints)
		api.GET("/logs", s.handleGetLogs)
	}
}

func (s *AdminServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.WebAdmin.Host, s.config.WebAdmin.Port)
	s.logger.Info(fmt.Sprintf("Starting admin server on %s", addr))
	return s.router.Run(addr)
}

func (s *AdminServer) GetRouter() *gin.Engine {
	return s.router
}