package web

import (
	"fmt"
	"net/http"
	"strconv"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"

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
	// 获取failed_only参数
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)
	
	logs, total, _ := s.logger.GetLogs(50, 0, failedOnly)
	c.HTML(http.StatusOK, "logs.html", gin.H{
		"Title":     "Request Logs",
		"Logs":      logs,
		"Total":     total,
		"FailedOnly": failedOnly,
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

	s.endpointManager.UpdateEndpoints(request.Endpoints)
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
func createEndpointConfigFromRequest(name, url, pathPrefix, authType, authValue string, enabled bool, priority int) config.EndpointConfig {
	return config.EndpointConfig{
		Name:       name,
		URL:        url,
		PathPrefix: pathPrefix,
		AuthType:   authType,
		AuthValue:  authValue,
		Enabled:    enabled,
		Priority:   priority,
	}
}

// handleCreateEndpoint 创建新端点
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	var request struct {
		Name       string `json:"name" binding:"required"`
		URL        string `json:"url" binding:"required"`
		PathPrefix string `json:"path_prefix"`
		AuthType   string `json:"auth_type" binding:"required"`
		AuthValue  string `json:"auth_value" binding:"required"`
		Enabled    bool   `json:"enabled"`
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
		request.Enabled, maxPriority+1)
	currentEndpoints = append(currentEndpoints, newEndpoint)

	// 保存到配置文件
	if err := s.saveEndpointsToConfig(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
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
		Name       string `json:"name"`
		URL        string `json:"url"`
		PathPrefix string `json:"path_prefix"`
		AuthType   string `json:"auth_type"`
		AuthValue  string `json:"auth_value"`
		Enabled    bool   `json:"enabled"`
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
			
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 保存到配置文件
	if err := s.saveEndpointsToConfig(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
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

	// 保存到配置文件
	if err := s.saveEndpointsToConfig(newEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
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

	// 保存到配置文件
	if err := s.saveEndpointsToConfig(newEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints reordered successfully"})
}