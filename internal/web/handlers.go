package web

import (
	"fmt"
	"net/http"
	"strconv"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *AdminServer) handleDashboard(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	
	totalRequests := 0
	successRequests := 0
	activeEndpoints := 0
	
	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}
	
	endpointStats := make([]EndpointStats, 0)
	
	for _, ep := range endpoints {
		totalRequests += ep.TotalRequests
		successRequests += ep.SuccessRequests
		if ep.Status == endpoint.StatusActive {
			activeEndpoints++
		}
		
		successRate := "N/A"
		if ep.TotalRequests > 0 {
			rate := float64(ep.SuccessRequests) / float64(ep.TotalRequests) * 100.0
			successRate = fmt.Sprintf("%.1f%%", rate)
		}
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	overallSuccessRate := "N/A"
	if totalRequests > 0 {
		rate := float64(successRequests) / float64(totalRequests) * 100.0
		overallSuccessRate = fmt.Sprintf("%.1f%%", rate)
	}
	
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title":             "Claude Proxy Dashboard",
		"TotalEndpoints":    len(endpoints),
		"ActiveEndpoints":   activeEndpoints,
		"TotalRequests":     totalRequests,
		"SuccessRequests":   successRequests,
		"OverallSuccessRate": overallSuccessRate,
		"Endpoints":         endpointStats,
	})
}

func (s *AdminServer) handleEndpointsPage(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	
	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}
	
	endpointStats := make([]EndpointStats, 0)
	
	for _, ep := range endpoints {
		successRate := "N/A"
		if ep.TotalRequests > 0 {
			rate := float64(ep.SuccessRequests) / float64(ep.TotalRequests) * 100.0
			successRate = fmt.Sprintf("%.1f%%", rate)
		}
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	c.HTML(http.StatusOK, "endpoints.html", gin.H{
		"Title":     "Endpoints Configuration",
		"Endpoints": endpointStats,
	})
}

func (s *AdminServer) handleLogsPage(c *gin.Context) {
	// 获取参数
	pageStr := c.DefaultQuery("page", "1")
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)
	
	// 每页100条记录
	limit := 100
	offset := (page - 1) * limit
	
	logs, total, _ := s.logger.GetLogs(limit, offset, failedOnly)
	
	// 计算分页信息
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	
	// 生成分页数组
	var pages []int
	startPage := page - 5
	if startPage < 1 {
		startPage = 1
	}
	endPage := startPage + 9
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 9
		if startPage < 1 {
			startPage = 1
		}
	}
	
	for i := startPage; i <= endPage; i++ {
		pages = append(pages, i)
	}
	
	c.HTML(http.StatusOK, "logs.html", gin.H{
		"Title":       "Request Logs",
		"Logs":        logs,
		"Total":       total,
		"FailedOnly":  failedOnly,
		"Page":        page,
		"TotalPages":  totalPages,
		"Pages":       pages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
		"Limit":       limit,
	})
}

func (s *AdminServer) handleSettingsPage(c *gin.Context) {
	// 计算启用的端点数量
	enabledCount := 0
	for _, ep := range s.config.Endpoints {
		if ep.Enabled {
			enabledCount++
		}
	}
	
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"Title":        "Settings",
		"Config":       s.config,
		"EnabledCount": enabledCount,
	})
}

func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	c.JSON(http.StatusOK, gin.H{
		"endpoints": endpoints,
	})
}

func (s *AdminServer) handleUpdateEndpoints(c *gin.Context) {
	var request struct {
		Endpoints []config.EndpointConfig `json:"endpoints"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 创建新配置，只更新端点部分
	newConfig := *s.config
	newConfig.Endpoints = request.Endpoints

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update endpoints: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after endpoint update", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig
	} else {
		// 回退到旧的更新方式（如果没有热更新处理器）
		s.endpointManager.UpdateEndpoints(request.Endpoints)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints updated successfully"})
}

func (s *AdminServer) handleGetLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	requestIDStr := c.DefaultQuery("request_id", "")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)

	if requestIDStr != "" {
		// 如果指定了request_id，返回该请求的所有尝试记录
		allLogs, _ := s.logger.GetAllLogsByRequestID(requestIDStr)
		c.JSON(http.StatusOK, gin.H{
			"logs":  allLogs,
			"total": len(allLogs),
		})
		return
	}

	logs, total, err := s.logger.GetLogs(limit, offset, failedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// saveEndpointsToConfig 将端点配置保存到配置文件
func (s *AdminServer) saveEndpointsToConfig(endpointConfigs []config.EndpointConfig) error {
	// 更新配置
	s.config.Endpoints = endpointConfigs
	
	// 保存到文件
	return config.SaveConfig(s.config, s.configFilePath)
}

// createEndpointConfigFromRequest 从请求创建端点配置，自动设置优先级
func createEndpointConfigFromRequest(name, url, pathPrefix, authType, authValue string, enabled bool, priority int, tags []string) config.EndpointConfig {
	return config.EndpointConfig{
		Name:       name,
		URL:        url,
		PathPrefix: pathPrefix,
		AuthType:   authType,
		AuthValue:  authValue,
		Enabled:    enabled,
		Priority:   priority,
		Tags:       tags,
	}
}

// handleCreateEndpoint 创建新端点
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	var request struct {
		Name       string   `json:"name" binding:"required"`
		URL        string   `json:"url" binding:"required"`
		PathPrefix string   `json:"path_prefix"`
		AuthType   string   `json:"auth_type" binding:"required"`
		AuthValue  string   `json:"auth_value" binding:"required"`
		Enabled    bool     `json:"enabled"`
		Tags       []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 验证auth_type
	if request.AuthType != "api_key" && request.AuthType != "auth_token" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key' or 'auth_token'"})
		return
	}

	// 设置默认值 - 移除timeout相关逻辑

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints

	// 新端点的优先级为当前最大优先级+1
	maxPriority := 0
	for _, ep := range currentEndpoints {
		if ep.Priority > maxPriority {
			maxPriority = ep.Priority
		}
	}

	// 创建新端点配置
	newEndpoint := createEndpointConfigFromRequest(
		request.Name, request.URL, request.PathPrefix, 
		request.AuthType, request.AuthValue, 
		request.Enabled, maxPriority+1, request.Tags)
	currentEndpoints = append(currentEndpoints, newEndpoint)

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		// 创建新配置，只更新端点部分
		newConfig := *s.config
		newConfig.Endpoints = currentEndpoints

		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create endpoint: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after endpoint creation", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig
	} else {
		// 回退到旧方式
		if err := s.saveEndpointsToConfig(currentEndpoints); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Endpoint created successfully",
		"endpoint": newEndpoint,
	})
}

// handleUpdateEndpoint 更新特定端点
func (s *AdminServer) handleUpdateEndpoint(c *gin.Context) {
	endpointName := c.Param("id") // 使用名称作为ID

	var request struct {
		Name       string   `json:"name"`
		URL        string   `json:"url"`
		PathPrefix string   `json:"path_prefix"`
		AuthType   string   `json:"auth_type"`
		AuthValue  string   `json:"auth_value"`
		Enabled    bool     `json:"enabled"`
		Tags       []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	found := false

	for i, ep := range currentEndpoints {
		if ep.Name == endpointName {
			// 更新端点，保持原有优先级
			if request.Name != "" {
				currentEndpoints[i].Name = request.Name
			}
			if request.URL != "" {
				currentEndpoints[i].URL = request.URL
			}
			if request.PathPrefix != "" {
				currentEndpoints[i].PathPrefix = request.PathPrefix
			}
			if request.AuthType != "" {
				if request.AuthType != "api_key" && request.AuthType != "auth_token" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key' or 'auth_token'"})
					return
				}
				currentEndpoints[i].AuthType = request.AuthType
			}
			if request.AuthValue != "" {
				currentEndpoints[i].AuthValue = request.AuthValue
			}
			currentEndpoints[i].Enabled = request.Enabled
			
			// 更新tags字段
			currentEndpoints[i].Tags = request.Tags
			
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		// 创建新配置，只更新端点部分
		newConfig := *s.config
		newConfig.Endpoints = currentEndpoints

		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update endpoint: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after endpoint update", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig
	} else {
		// 回退到旧方式
		if err := s.saveEndpointsToConfig(currentEndpoints); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint updated successfully"})
}

// handleDeleteEndpoint 删除端点
func (s *AdminServer) handleDeleteEndpoint(c *gin.Context) {
	endpointName := c.Param("id") // 使用名称作为ID

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	newEndpoints := make([]config.EndpointConfig, 0)
	found := false

	for _, ep := range currentEndpoints {
		if ep.Name != endpointName {
			newEndpoints = append(newEndpoints, ep)
		} else {
			found = true
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 重新计算优先级（按数组顺序）
	for i := range newEndpoints {
		newEndpoints[i].Priority = i + 1
	}

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		// 创建新配置，只更新端点部分
		newConfig := *s.config
		newConfig.Endpoints = newEndpoints

		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete endpoint: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after endpoint deletion", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig
	} else {
		// 回退到旧方式
		if err := s.saveEndpointsToConfig(newEndpoints); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint deleted successfully"})
}

// handleReorderEndpoints 重新排序端点
func (s *AdminServer) handleReorderEndpoints(c *gin.Context) {
	var request struct {
		OrderedNames []string `json:"ordered_names" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	
	// 创建按名称索引的map
	endpointMap := make(map[string]config.EndpointConfig)
	for _, ep := range currentEndpoints {
		endpointMap[ep.Name] = ep
	}

	// 按新顺序重新排列
	newEndpoints := make([]config.EndpointConfig, 0, len(request.OrderedNames))
	for i, name := range request.OrderedNames {
		if ep, exists := endpointMap[name]; exists {
			ep.Priority = i + 1 // 优先级从1开始
			newEndpoints = append(newEndpoints, ep)
		}
	}

	// 检查是否所有端点都被包含
	if len(newEndpoints) != len(currentEndpoints) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ordered names must include all existing endpoints"})
		return
	}

	// 使用热更新机制
	if s.hotUpdateHandler != nil {
		// 创建新配置，只更新端点部分
		newConfig := *s.config
		newConfig.Endpoints = newEndpoints

		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to reorder endpoints: " + err.Error(),
			})
			return
		}

		// 保存配置到文件
		if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
			s.logger.Error("Failed to save configuration file after endpoint reorder", err)
			// 不返回错误，因为内存更新已成功
		}

		// 更新本地配置引用
		s.config = &newConfig
	} else {
		// 回退到旧方式
		if err := s.saveEndpointsToConfig(newEndpoints); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints reordered successfully"})
}

// handleGetLogStats 获取日志统计信息
func (s *AdminServer) handleGetLogStats(c *gin.Context) {
	// SQLite存储提供基本统计信息
	stats := map[string]interface{}{
		"storage_type": "sqlite",
		"message": "SQLite storage active with automatic cleanup (30 days retention)",
		"features": []string{
			"Automatic cleanup of logs older than 30 days",
			"Indexed queries for better performance", 
			"Memory efficient storage",
			"ACID transactions",
		},
	}
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// handleGetConfig 获取当前配置
func (s *AdminServer) handleGetConfig(c *gin.Context) {
	// 返回当前配置，但隐藏敏感信息
	configCopy := *s.config
	
	// 隐藏认证信息的敏感部分
	for i := range configCopy.Endpoints {
		if configCopy.Endpoints[i].AuthValue != "" {
			// 只显示前4个和后4个字符
			authValue := configCopy.Endpoints[i].AuthValue
			if len(authValue) > 8 {
				configCopy.Endpoints[i].AuthValue = authValue[:4] + "****" + authValue[len(authValue)-4:]
			} else {
				configCopy.Endpoints[i].AuthValue = "****"
			}
		}
	}
	
	// 隐藏服务器认证token
	if configCopy.Server.AuthToken != "" {
		token := configCopy.Server.AuthToken
		if len(token) > 8 {
			configCopy.Server.AuthToken = token[:4] + "****" + token[len(token)-4:]
		} else {
			configCopy.Server.AuthToken = "****"
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"config": configCopy,
	})
}

// handleHotUpdateConfig 热更新配置
func (s *AdminServer) handleHotUpdateConfig(c *gin.Context) {
	var request struct {
		Config config.Config `json:"config"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	// 验证新配置
	newConfig := request.Config
	if err := s.validateConfigUpdate(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration validation failed: " + err.Error(),
		})
		return
	}

	// 保存配置到文件
	if err := config.SaveConfig(&newConfig, s.configFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save configuration file: " + err.Error(),
		})
		return
	}

	// 如果有热更新处理器，执行热更新
	if s.hotUpdateHandler != nil {
		if err := s.hotUpdateHandler.HotUpdateConfig(&newConfig); err != nil {
			// 热更新失败，记录错误但不回滚文件（文件已保存成功）
			s.logger.Error("Hot update failed, configuration file saved but runtime not updated", err)
			c.JSON(http.StatusPartialContent, gin.H{
				"warning": "Configuration file saved successfully, but hot update failed: " + err.Error(),
				"message": "Server restart may be required for some changes to take effect",
			})
			return
		}
	}

	// 更新本地配置引用
	s.config = &newConfig

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully via hot update",
	})
}

// validateConfigUpdate validates the configuration update using unified validation
func (s *AdminServer) validateConfigUpdate(newConfig *config.Config) error {
	// 使用统一的服务器配置验证
	if err := utils.ValidateServerConfig(newConfig.Server.Host, newConfig.Server.Port, newConfig.Server.AuthToken); err != nil {
		return err
	}

	// 转换为接口类型进行统一验证
	validator := utils.NewEndpointConfigValidator()
	endpointInterfaces := make([]utils.EndpointConfig, len(newConfig.Endpoints))
	for i, ep := range newConfig.Endpoints {
		endpointInterfaces[i] = ep
	}

	return validator.ValidateEndpoints(endpointInterfaces)
}