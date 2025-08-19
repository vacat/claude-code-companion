package web

import (
	"fmt"
	"net/http"

	"claude-code-companion/internal/config"

	"github.com/gin-gonic/gin"
)

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
	newConfig := *s.config
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