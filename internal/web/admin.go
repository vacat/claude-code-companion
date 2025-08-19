package web

import (
	"fmt"
	"net/http"
	"strings"

	"claude-code-companion/internal/config"
	"claude-code-companion/internal/endpoint"
	"claude-code-companion/internal/i18n"
	"claude-code-companion/internal/logger"
	"claude-code-companion/internal/tagging"
	"claude-code-companion/internal/webres"

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
	version           string
	i18nManager       *i18n.Manager
}

func NewAdminServer(cfg *config.Config, endpointManager *endpoint.Manager, taggingManager *tagging.Manager, log *logger.Logger, configFilePath string, version string, i18nManager *i18n.Manager) *AdminServer {
	return &AdminServer{
		config:          cfg,
		endpointManager: endpointManager,
		taggingManager:  taggingManager,
		logger:          log,
		configFilePath:  configFilePath,
		version:         version,
		i18nManager:     i18nManager,
	}
}

// SetHotUpdateHandler sets the hot update handler
func (s *AdminServer) SetHotUpdateHandler(handler HotUpdateHandler) {
	s.hotUpdateHandler = handler
}

// renderHTML renders template with i18n support
func (s *AdminServer) renderHTML(c *gin.Context, templateName string, data map[string]interface{}) {
	// Always detect language fresh
	lang := s.i18nManager.GetDetector().DetectLanguage(c)
	i18n.SetLanguageToContext(c, lang)
	
	// If i18n is disabled or language is default, render normally
	if s.i18nManager == nil || !s.i18nManager.IsEnabled() || lang == s.i18nManager.GetDefaultLanguage() {
		c.HTML(200, templateName, data)
		return
	}
	
	// For non-default languages, we need to post-process
	// Create a custom writer that captures the output
	originalWriter := c.Writer
	captureWriter := &captureResponseWriter{ResponseWriter: originalWriter}
	c.Writer = captureWriter
	
	// Render template
	c.HTML(200, templateName, data)
	
	// Process the captured HTML through translator
	html := captureWriter.GetHTML()
	translator := s.i18nManager.GetTranslator()
	translatedHTML := translator.ProcessHTML(html, lang, s.i18nManager.GetTranslation)
	
	// Write the translated HTML to original writer
	c.Writer = originalWriter
	c.Writer.Write([]byte(translatedHTML))
}

// captureResponseWriter captures response for post-processing
type captureResponseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *captureResponseWriter) GetHTML() string {
	return string(w.body)
}

// getBaseTemplateData returns common template data for all pages
func (s *AdminServer) getBaseTemplateData(c *gin.Context, currentPage string) map[string]interface{} {
	lang := s.i18nManager.GetDetector().DetectLanguage(c)
	
	// Build available languages data
	availableLanguages := make([]map[string]interface{}, 0)
	for _, availableLang := range s.i18nManager.GetAvailableLanguages() {
		langInfo := s.i18nManager.GetLanguageInfo(availableLang)
		availableLanguages = append(availableLanguages, map[string]interface{}{
			"code": string(availableLang),
			"flag": langInfo["flag"],
			"name": langInfo["name"],
		})
	}
	
	return map[string]interface{}{
		"Version":            s.version,
		"CurrentPage":        currentPage,
		"CurrentLanguage":    string(lang),
		"AvailableLanguages": availableLanguages,
	}
}

// mergeTemplateData merges base template data with page-specific data
func (s *AdminServer) mergeTemplateData(c *gin.Context, currentPage string, pageData map[string]interface{}) map[string]interface{} {
	baseData := s.getBaseTemplateData(c, currentPage)
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
	// 加载嵌入的模板
	templates, err := webres.LoadTemplates()
	if err != nil {
		panic("Failed to load embedded templates: " + err.Error())
	}
	router.SetHTMLTemplate(templates)
	
	// 设置静态文件服务器（使用嵌入的文件系统）
	staticFS, err := webres.GetStaticFS()
	if err != nil {
		panic("Failed to get embedded static filesystem: " + err.Error())
	}
	router.StaticFS("/static", http.FS(staticFS))

	// 注册根目录帮助页面
	router.GET("/", s.handleHelpPage)

	// 注册页面路由
	router.GET("/admin/", s.handleDashboard)
	router.GET("/admin/endpoints", s.handleEndpointsPage)
	router.GET("/admin/taggers", s.handleTaggersPage)
	router.GET("/admin/logs", s.handleLogsPage)
	router.GET("/admin/settings", s.handleSettingsPage)

	// 注册 API 路由，添加UTF-8字符集中间件
	api := router.Group("/admin/api")
	api.Use(s.utf8JsonMiddleware()) // 添加UTF-8中间件
	{
		api.GET("/endpoints", s.handleGetEndpoints)
		api.PUT("/endpoints", s.handleUpdateEndpoints)
		api.POST("/endpoints", s.handleCreateEndpoint)
		api.PUT("/endpoints/:id", s.handleUpdateEndpoint)
		api.PUT("/endpoints/:id/model-rewrite", s.handleUpdateEndpointModelRewrite)
		api.POST("/endpoints/:id/test-model-rewrite", s.handleTestModelRewrite)
		api.DELETE("/endpoints/:id", s.handleDeleteEndpoint)
		api.POST("/endpoints/:id/copy", s.handleCopyEndpoint)
		api.POST("/endpoints/:id/toggle", s.handleToggleEndpoint)
		api.POST("/endpoints/reorder", s.handleReorderEndpoints)
		
		api.GET("/taggers", s.handleGetTaggers)
		api.POST("/taggers", s.handleCreateTagger)
		api.PUT("/taggers/:name", s.handleUpdateTagger)
		api.DELETE("/taggers/:name", s.handleDeleteTagger)
		api.GET("/tags", s.handleGetTags)
		
		api.GET("/logs", s.handleGetLogs)
		api.POST("/logs/cleanup", s.handleCleanupLogs)
		api.GET("/logs/stats", s.handleGetLogStats)
		api.PUT("/config", s.handleHotUpdateConfig)
		api.GET("/config", s.handleGetConfig)
		api.PUT("/settings", s.handleUpdateSettings)
	}
}

// utf8JsonMiddleware 确保所有JSON响应都包含UTF-8字符集声明
func (s *AdminServer) utf8JsonMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 处理请求
		c.Next()
		
		// 如果响应是JSON，确保Content-Type包含UTF-8字符集
		contentType := c.Writer.Header().Get("Content-Type")
		if contentType == "application/json" {
			c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
	})
}

// i18nMiddleware provides internationalization support
func (s *AdminServer) i18nMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if s.i18nManager == nil || !s.i18nManager.IsEnabled() {
			c.Next()
			return
		}

		// Detect user's preferred language
		lang := s.i18nManager.GetDetector().DetectLanguage(c)
		i18n.SetLanguageToContext(c, lang)

		// Only apply translation for /admin/ pages
		if strings.HasPrefix(c.Request.URL.Path, "/admin/") && 
		   !strings.HasPrefix(c.Request.URL.Path, "/admin/api/") {
			// Override HTML response to process translations
			originalWriter := c.Writer
			c.Writer = &translatingResponseWriter{
				ResponseWriter: originalWriter,
				lang:           lang,
				i18nManager:    s.i18nManager,
			}
		}

		c.Next()
	})
}

// translatingResponseWriter wraps gin.ResponseWriter to process translations
type translatingResponseWriter struct {
	gin.ResponseWriter
	lang        i18n.Language
	i18nManager *i18n.Manager
}

func (w *translatingResponseWriter) Write(data []byte) (int, error) {
	// Always process if it looks like HTML content
	html := string(data)
	if strings.Contains(html, "<!DOCTYPE html") || strings.Contains(html, "<html") {
		// Process translations
		translator := w.i18nManager.GetTranslator()
		translatedHTML := translator.ProcessHTML(html, w.lang, w.i18nManager.GetTranslation)
		return w.ResponseWriter.Write([]byte(translatedHTML))
	}

	return w.ResponseWriter.Write(data)
}

