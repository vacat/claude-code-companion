package web

import (
	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/logger"

	"github.com/gin-gonic/gin"
)

type AdminServer struct {
	config          *config.Config
	endpointManager *endpoint.Manager
	logger          *logger.Logger
	configFilePath  string
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, log *logger.Logger, configFilePath string) *AdminServer {
	return &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		logger:          log,
		configFilePath:  configFilePath,
	}
}

// RegisterRoutes 注册管理界面路由到指定的 router
func (s *AdminServer) RegisterRoutes(router *gin.Engine) {
	// 加载模板和静态文件
	router.LoadHTMLGlob("web/templates/*")
	router.Static("/static", "web/static")

	// 注册页面路由
	router.GET("/admin/", s.handleDashboard)
	router.GET("/admin/endpoints", s.handleEndpointsPage)
	router.GET("/admin/logs", s.handleLogsPage)
	router.GET("/admin/settings", s.handleSettingsPage)

	// 注册 API 路由
	api := router.Group("/admin/api")
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

