package web

import (
	"fmt"
	"net/http"

	"claude-proxy/internal/config"

	"github.com/gin-gonic/gin"
)

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
	if err := s.hotUpdateEndpoints(request.Endpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update endpoints: " + err.Error(),
		})
		return
	}

	// 如果没有热更新处理器，则使用旧方式
	if s.hotUpdateHandler == nil {
		s.endpointManager.UpdateEndpoints(request.Endpoints)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints updated successfully"})
}

// saveEndpointsToConfig 将端点配置保存到配置文件
func (s *AdminServer) saveEndpointsToConfig(endpointConfigs []config.EndpointConfig) error {
	// 更新配置
	s.config.Endpoints = endpointConfigs
	
	// 保存到文件
	return config.SaveConfig(s.config, s.configFilePath)
}

// createEndpointConfigFromRequest 从请求创建端点配置，自动设置优先级
func createEndpointConfigFromRequest(name, url, endpointType, pathPrefix, authType, authValue string, enabled bool, priority int, tags []string, proxy *config.ProxyConfig, oauthConfig *config.OAuthConfig) config.EndpointConfig {
	// 如果没有指定endpoint_type，默认为anthropic（向后兼容）
	if endpointType == "" {
		endpointType = "anthropic"
	}
	
	return config.EndpointConfig{
		Name:         name,
		URL:          url,
		EndpointType: endpointType,
		PathPrefix:   pathPrefix, // 新增：支持路径前缀
		AuthType:     authType,
		AuthValue:    authValue,
		Enabled:      enabled,
		Priority:     priority,
		Tags:         tags,
		Proxy:        proxy, // 新增：支持代理配置
		OAuthConfig:  oauthConfig, // 新增：支持OAuth配置
	}
}

// handleCreateEndpoint 创建新端点
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	var request struct {
		Name         string               `json:"name" binding:"required"`
		URL          string               `json:"url" binding:"required"`
		EndpointType string               `json:"endpoint_type"` // "anthropic" | "openai"
		PathPrefix   string               `json:"path_prefix"`   // OpenAI 端点的路径前缀
		AuthType     string               `json:"auth_type" binding:"required"`
		AuthValue    string               `json:"auth_value"`    // OAuth时不需要
		Enabled      bool                 `json:"enabled"`
		Tags         []string             `json:"tags"`
		Proxy        *config.ProxyConfig  `json:"proxy,omitempty"` // 新增：代理配置
		OAuthConfig  *config.OAuthConfig  `json:"oauth_config,omitempty"` // 新增：OAuth配置
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 验证auth_type
	if request.AuthType != "api_key" && request.AuthType != "auth_token" && request.AuthType != "oauth" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key', 'auth_token', or 'oauth'"})
		return
	}
	
	// 验证 OAuth 或传统认证配置
	if request.AuthType == "oauth" {
		if request.OAuthConfig == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "oauth_config is required when auth_type is 'oauth'"})
			return
		}
		// 验证OAuth配置
		if err := config.ValidateOAuthConfig(request.OAuthConfig, fmt.Sprintf("endpoint '%s'", request.Name)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid oauth config: " + err.Error()})
			return
		}
	} else {
		// 非 OAuth 认证需要 auth_value
		if request.AuthValue == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth_value is required for non-oauth authentication"})
			return
		}
	}

	// 验证代理配置（如果提供）
	if request.Proxy != nil {
		if err := config.ValidateProxyConfig(request.Proxy, fmt.Sprintf("endpoint '%s'", request.Name)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy config: " + err.Error()})
			return
		}
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
		request.Name, request.URL, request.EndpointType, request.PathPrefix,
		request.AuthType, request.AuthValue, 
		request.Enabled, maxPriority+1, request.Tags, request.Proxy, request.OAuthConfig)
	currentEndpoints = append(currentEndpoints, newEndpoint)

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create endpoint: " + err.Error(),
		})
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
		Name         string               `json:"name"`
		URL          string               `json:"url"`
		EndpointType string               `json:"endpoint_type"`
		PathPrefix   string               `json:"path_prefix"` // OpenAI 端点的路径前缀
		AuthType     string               `json:"auth_type"`
		AuthValue    string               `json:"auth_value"`
		Enabled      bool                 `json:"enabled"`
		Tags         []string             `json:"tags"`
		Proxy        *config.ProxyConfig  `json:"proxy,omitempty"` // 新增：代理配置
		OAuthConfig  *config.OAuthConfig  `json:"oauth_config,omitempty"` // 新增：OAuth配置
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 验证代理配置（如果提供）
	if request.Proxy != nil {
		if err := config.ValidateProxyConfig(request.Proxy, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy config: " + err.Error()})
			return
		}
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
			if request.EndpointType != "" {
				currentEndpoints[i].EndpointType = request.EndpointType
			}
			// 处理 PathPrefix 字段，允许设置空值（对于 Anthropic 端点）
			currentEndpoints[i].PathPrefix = request.PathPrefix
			if request.AuthType != "" {
				if request.AuthType != "api_key" && request.AuthType != "auth_token" && request.AuthType != "oauth" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key', 'auth_token', or 'oauth'"})
					return
				}
				
				// 验证 OAuth 或传统认证配置
				if request.AuthType == "oauth" {
					if request.OAuthConfig == nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "oauth_config is required when auth_type is 'oauth'"})
						return
					}
					// 验证OAuth配置
					if err := config.ValidateOAuthConfig(request.OAuthConfig, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid oauth config: " + err.Error()})
						return
					}
					
					// 检查内存中是否已有更新的 OAuth token（防止覆盖已刷新的token）
					if currentEndpoints[i].AuthType == "oauth" && currentEndpoints[i].OAuthConfig != nil {
						currentExpiresAt := currentEndpoints[i].OAuthConfig.ExpiresAt
						requestExpiresAt := request.OAuthConfig.ExpiresAt
						
						// 如果内存中的过期时间比 WebUI 发送的更大，说明后台已刷新token，拒绝更新
						if currentExpiresAt > requestExpiresAt && requestExpiresAt > 0 {
							c.JSON(http.StatusConflict, gin.H{
								"error": "Cannot update OAuth config: token has been refreshed in background. Please reload the page to get the latest configuration.",
								"current_expires_at": currentExpiresAt,
								"request_expires_at": requestExpiresAt,
							})
							return
						}
					}
					
					// 设置OAuth配置，清空auth_value
					currentEndpoints[i].OAuthConfig = request.OAuthConfig
					currentEndpoints[i].AuthValue = ""
				} else {
					// 非 OAuth 认证，清空OAuth配置
					currentEndpoints[i].OAuthConfig = nil
					if request.AuthValue != "" {
						currentEndpoints[i].AuthValue = request.AuthValue
					}
				}
				currentEndpoints[i].AuthType = request.AuthType
			}
			currentEndpoints[i].Enabled = request.Enabled
			
			// 更新tags字段
			currentEndpoints[i].Tags = request.Tags
			
			// 更新代理配置
			currentEndpoints[i].Proxy = request.Proxy
			
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update endpoint: " + err.Error(),
		})
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

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(newEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete endpoint: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint deleted successfully"})
}

// generateUniqueEndpointName 生成唯一的端点名称，如果存在重名则添加数字后缀
func (s *AdminServer) generateUniqueEndpointName(baseName string) string {
	currentEndpoints := s.config.Endpoints
	
	// 检查基础名称是否已存在
	nameExists := func(name string) bool {
		for _, ep := range currentEndpoints {
			if ep.Name == name {
				return true
			}
		}
		return false
	}
	
	// 如果基础名称不存在，直接返回
	if !nameExists(baseName) {
		return baseName
	}
	
	// 如果存在，添加数字后缀
	counter := 1
	for {
		newName := fmt.Sprintf("%s (%d)", baseName, counter)
		if !nameExists(newName) {
			return newName
		}
		counter++
	}
}

// handleToggleEndpoint 切换端点启用/禁用状态
func (s *AdminServer) handleToggleEndpoint(c *gin.Context) {
	endpointName := c.Param("id") // 端点名称

	var request struct {
		Enabled bool `json:"enabled"`
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
			// 更新enabled状态
			currentEndpoints[i].Enabled = request.Enabled
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to toggle endpoint: " + err.Error(),
		})
		return
	}

	actionText := "enabled"
	if !request.Enabled {
		actionText = "disabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Endpoint '%s' has been %s successfully", endpointName, actionText),
	})
}

// handleCopyEndpoint 复制端点
func (s *AdminServer) handleCopyEndpoint(c *gin.Context) {
	endpointName := c.Param("id") // 要复制的端点名称

	// 查找源端点
	var sourceEndpoint *config.EndpointConfig
	for _, ep := range s.config.Endpoints {
		if ep.Name == endpointName {
			sourceEndpoint = &ep
			break
		}
	}

	if sourceEndpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 生成唯一的新名称
	newName := s.generateUniqueEndpointName(sourceEndpoint.Name)

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints

	// 计算新端点的优先级
	maxPriority := 0
	for _, ep := range currentEndpoints {
		if ep.Priority > maxPriority {
			maxPriority = ep.Priority
		}
	}

	// 创建新端点（复制所有属性，除了名称和优先级）
	newEndpoint := config.EndpointConfig{
		Name:         newName,
		URL:          sourceEndpoint.URL,
		EndpointType: sourceEndpoint.EndpointType,
		PathPrefix:   sourceEndpoint.PathPrefix,
		AuthType:     sourceEndpoint.AuthType,
		AuthValue:    sourceEndpoint.AuthValue,
		Enabled:      sourceEndpoint.Enabled,
		Priority:     maxPriority + 1,
		Tags:         make([]string, len(sourceEndpoint.Tags)), // 复制tags
	}

	// 深度复制Tags切片
	copy(newEndpoint.Tags, sourceEndpoint.Tags)

	// 深度复制ModelRewrite配置
	if sourceEndpoint.ModelRewrite != nil {
		newEndpoint.ModelRewrite = &config.ModelRewriteConfig{
			Enabled: sourceEndpoint.ModelRewrite.Enabled,
			Rules:   make([]config.ModelRewriteRule, len(sourceEndpoint.ModelRewrite.Rules)),
		}
		copy(newEndpoint.ModelRewrite.Rules, sourceEndpoint.ModelRewrite.Rules)
	}

	// 深度复制Proxy配置
	if sourceEndpoint.Proxy != nil {
		newEndpoint.Proxy = &config.ProxyConfig{
			Type:     sourceEndpoint.Proxy.Type,
			Address:  sourceEndpoint.Proxy.Address,
			Username: sourceEndpoint.Proxy.Username,
			Password: sourceEndpoint.Proxy.Password,
		}
	}

	// 添加到端点列表
	currentEndpoints = append(currentEndpoints, newEndpoint)

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to copy endpoint: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Endpoint copied successfully",
		"endpoint": newEndpoint,
	})
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
	if err := s.hotUpdateEndpoints(newEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to reorder endpoints: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints reordered successfully"})
}