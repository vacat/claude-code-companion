package web

import (
	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/logger"
	"claude-proxy/internal/tagging"

	"github.com/gin-gonic/gin"
)

// HotUpdateHandler defines the interface for hot config updates
type HotUpdateHandler interface {
	HotUpdateConfig(newConfig *config.Config) error
}

type AdminServer struct {
	config            *config.Config
	endpointManager   *endpoint.Manager
	taggingManager    *tagging.Manager
	logger            *logger.Logger
	configFilePath    string
	hotUpdateHandler  HotUpdateHandler
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, taggingManager *tagging.Manager, log *logger.Logger, configFilePath string) *AdminServer {
	return &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		taggingManager:  taggingManager,
		logger:          log,
		configFilePath:  configFilePath,
	}
}

// SetHotUpdateHandler sets the hot update handler
func (s *AdminServer) SetHotUpdateHandler(handler HotUpdateHandler) {
	s.hotUpdateHandler = handler
}

// RegisterRoutes 注册管理界面路由到指定的 router
func (s *AdminServer) RegisterRoutes(router *gin.Engine) {
	// 加载模板和静态文件
	router.LoadHTMLGlob("web/templates/*")
	router.Static("/static", "web/static")

	// 注册页面路由
	router.GET("/admin/", s.handleDashboard)
	router.GET("/admin/endpoints", s.handleEndpointsPage)
	router.GET("/admin/taggers", s.handleTaggersPage)
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
		
		api.GET("/taggers", s.handleGetTaggers)
		api.POST("/taggers", s.handleCreateTagger)
		api.PUT("/taggers/:name", s.handleUpdateTagger)
		api.DELETE("/taggers/:name", s.handleDeleteTagger)
		api.GET("/tags", s.handleGetTags)
		
		api.GET("/logs", s.handleGetLogs)
		api.GET("/logs/stats", s.handleGetLogStats)
		api.PUT("/config", s.handleHotUpdateConfig)
		api.GET("/config", s.handleGetConfig)
	}
}

