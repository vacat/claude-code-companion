package web

import (
	"fmt"
	"net/http"

	"claude-code-companion/internal/config"

	"github.com/gin-gonic/gin"
)

// deepCopyConfig 执行Config的深拷贝，避免指针字段共享
func deepCopyConfig(src *config.Config) config.Config {
	// 拷贝基本字段
	dst := config.Config{
		Server:      src.Server,
		Logging:     src.Logging,
		Validation:  src.Validation,
		Timeouts:    src.Timeouts, // 新的TimeoutConfig是值类型，可以直接赋值
		I18n:        src.I18n,
	}
	
	// 深拷贝 Tagging.Taggers slice
	dst.Tagging = src.Tagging
	if src.Tagging.Taggers != nil {
		dst.Tagging.Taggers = make([]config.TaggerConfig, len(src.Tagging.Taggers))
		copy(dst.Tagging.Taggers, src.Tagging.Taggers)
	}
	
	// 深拷贝 Endpoints slice
	dst.Endpoints = make([]config.EndpointConfig, len(src.Endpoints))
	for i, ep := range src.Endpoints {
		dst.Endpoints[i] = ep
		
		// 深拷贝指针字段
		if ep.ModelRewrite != nil {
			modelRewrite := *ep.ModelRewrite
			// 深拷贝 Rules slice
			if ep.ModelRewrite.Rules != nil {
				modelRewrite.Rules = make([]config.ModelRewriteRule, len(ep.ModelRewrite.Rules))
				copy(modelRewrite.Rules, ep.ModelRewrite.Rules)
			}
			dst.Endpoints[i].ModelRewrite = &modelRewrite
		}
		
		if ep.Proxy != nil {
			proxy := *ep.Proxy
			dst.Endpoints[i].Proxy = &proxy
		}
		
		if ep.OAuthConfig != nil {
			oauth := *ep.OAuthConfig
			// 深拷贝 Scopes slice
			if ep.OAuthConfig.Scopes != nil {
				oauth.Scopes = make([]string, len(ep.OAuthConfig.Scopes))
				copy(oauth.Scopes, ep.OAuthConfig.Scopes)
			}
			dst.Endpoints[i].OAuthConfig = &oauth
		}
		
		if ep.OverrideMaxTokens != nil {
			maxTokens := *ep.OverrideMaxTokens
			dst.Endpoints[i].OverrideMaxTokens = &maxTokens
		}
		
		// 深拷贝 Tags slice
		if ep.Tags != nil {
			dst.Endpoints[i].Tags = make([]string, len(ep.Tags))
			copy(dst.Endpoints[i].Tags, ep.Tags)
		}
	}
	
	return dst
}

func (s *AdminServer) handleSettingsPage(c *gin.Context) {
	// 计算启用的端点数量
	enabledCount := 0
	for _, ep := range s.config.Endpoints {
		if ep.Enabled {
			enabledCount++
		}
	}
	
	data := s.mergeTemplateData(c, "settings", map[string]interface{}{
		"Title":        "Settings",
		"Config":       s.config,
		"EnabledCount": enabledCount,
	})
	s.renderHTML(c, "settings.html", data)
}

// handleUpdateSettings handles updating server settings
func (s *AdminServer) handleUpdateSettings(c *gin.Context) {
	// 定义请求结构
	type SettingsRequest struct {
		Server     config.ServerConfig         `json:"server"`
		Logging    config.LoggingConfig        `json:"logging"`
		Validation config.ValidationConfig    `json:"validation"`
		Timeouts   config.TimeoutConfig        `json:"timeouts"`
	}

	var request SettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	// 创建新的配置，保持现有的端点和其他配置不变
	// 使用深拷贝避免共享指针字段
	newConfig := deepCopyConfig(s.config)
	newConfig.Server = request.Server
	newConfig.Logging = request.Logging
	newConfig.Validation = request.Validation
	newConfig.Timeouts = request.Timeouts

	// 验证新配置
	if err := config.ValidateConfig(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration validation failed: " + err.Error(),
		})
		return
	}

	// 保存配置到文件
	if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
		s.logger.Error("Failed to save configuration file", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save configuration file: " + err.Error(),
		})
		return
	}

	// 更新内存中的配置
	s.config = &newConfig

	s.logger.Info("Settings updated successfully")
	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully",
	})
}

// handleHelpPage 处理帮助页面
func (s *AdminServer) handleHelpPage(c *gin.Context) {
	// 获取基础 URL（从请求中推断）
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	
	data := s.mergeTemplateData(c, "help", map[string]interface{}{
		"Title":   "Claude Code Setup Guide",
		"BaseURL": baseURL,
	})
	s.renderHTML(c, "help.html", data)
}