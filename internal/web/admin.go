package web

import (
	"fmt"

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
	buildVersion      string
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, taggingManager *tagging.Manager, log *logger.Logger, configFilePath string, buildVersion string) *AdminServer {
	return &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		taggingManager:  taggingManager,
		logger:          log,
		configFilePath:  configFilePath,
		buildVersion:    buildVersion,
	}
}

// SetHotUpdateHandler sets the hot update handler
func (s *AdminServer) SetHotUpdateHandler(handler HotUpdateHandler) {
	s.hotUpdateHandler = handler
}

// getBaseTemplateData returns common template data for all pages
func (s *AdminServer) getBaseTemplateData(currentPage string) map[string]interface{} {
	return map[string]interface{}{
		"BuildVersion": s.buildVersion,
		"CurrentPage":  currentPage,
	}
}

// mergeTemplateData merges base template data with page-specific data
func (s *AdminServer) mergeTemplateData(currentPage string, pageData map[string]interface{}) map[string]interface{} {
	baseData := s.getBaseTemplateData(currentPage)
	for key, value := range pageData {
		baseData[key] = value
	}
	return baseData
}

// calculateSuccessRate calculates success rate as a formatted percentage string
func calculateSuccessRate(successRequests, totalRequests int) string {
	if totalRequests == 0 {
		return "N/A"
	}
	rate := float64(successRequests) / float64(totalRequests) * 100.0
	return fmt.Sprintf("%.1f%%", rate)
}

// hotUpdateEndpoints performs hot update of endpoints configuration
func (s *AdminServer) hotUpdateEndpoints(endpoints []config.EndpointConfig) error {
	if s.hotUpdateHandler == nil {
		// 回退到旧的更新方式
		return s.saveEndpointsToConfig(endpoints)
	}

	// 创建新配置，只更新端点部分
	newConfig := *s.config
	newConfig.Endpoints = endpoints

	// 验证完整的配置
	if err := config.ValidateConfig(&newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
		return fmt.Errorf("failed to hot update: %v", err)
	}

	// 保存配置到文件
	if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
		s.logger.Error("Failed to save configuration file after endpoint update", err)
		// 不返回错误，因为内存更新已成功
	}

	// 更新本地配置引用
	s.config = &newConfig
	return nil
}

// updateConfigWithRollback 执行配置更新，失败时自动回滚
func (s *AdminServer) updateConfigWithRollback(updateFunc func() error, rollbackFunc func() error) error {
	if err := updateFunc(); err != nil {
		return err
	}
	
	// 保存配置到文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
		// 保存失败，尝试回滚
		if rollbackErr := rollbackFunc(); rollbackErr != nil {
			s.logger.Error("Failed to rollback after save error", rollbackErr)
		}
		return fmt.Errorf("failed to save configuration: %v", err)
	}
	
	return nil
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
		api.PUT("/endpoints/:id/model-rewrite", s.handleUpdateEndpointModelRewrite)
		api.POST("/endpoints/:id/test-model-rewrite", s.handleTestModelRewrite)
		api.DELETE("/endpoints/:id", s.handleDeleteEndpoint)
		api.POST("/endpoints/:id/copy", s.handleCopyEndpoint)
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
		api.PUT("/settings", s.handleUpdateSettings)
	}
}

