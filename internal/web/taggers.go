package web

import (
	"fmt"
	"net/http"

	"claude-proxy/internal/config"

	"github.com/gin-gonic/gin"
)

// TaggerResponse API响应格式
type TaggerResponse struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Tag         string                 `json:"tag"`
	BuiltinType string                 `json:"builtin_type,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Priority    int                    `json:"priority"`
	Config      map[string]interface{} `json:"config"`
}

// TagResponse API响应格式
type TagResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InUse       bool   `json:"in_use"`
}

// handleTaggersPage 显示tagger管理页面
func (s *AdminServer) handleTaggersPage(c *gin.Context) {
	c.HTML(http.StatusOK, "taggers.html", gin.H{
		"title": "Tagger Management",
	})
}

// handleGetTaggers 获取所有tagger配置
func (s *AdminServer) handleGetTaggers(c *gin.Context) {
	if !s.taggingManager.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"taggers": []TaggerResponse{},
		})
		return
	}

	var taggers []TaggerResponse
	
	// 从配置中获取tagger信息
	for _, taggerConfig := range s.config.Tagging.Taggers {
		tagger := TaggerResponse{
			Name:        taggerConfig.Name,
			Type:        taggerConfig.Type,
			Tag:         taggerConfig.Tag,
			BuiltinType: taggerConfig.BuiltinType,
			Enabled:     taggerConfig.Enabled,
			Priority:    taggerConfig.Priority,
			Config:      taggerConfig.Config,
		}
		taggers = append(taggers, tagger)
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"timeout": s.config.Tagging.PipelineTimeout,
		"taggers": taggers,
	})
}

// handleGetTags 获取所有已注册的tag
func (s *AdminServer) handleGetTags(c *gin.Context) {
	if !s.taggingManager.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"tags": []TagResponse{},
		})
		return
	}

	registry := s.taggingManager.GetRegistry()
	allTags := registry.ListTags()
	
	var tags []TagResponse
	for _, tag := range allTags {
		// 检查tag是否被endpoint使用
		inUse := false
		for _, ep := range s.endpointManager.GetAllEndpoints() {
			for _, epTag := range ep.GetTags() {
				if epTag == tag.Name {
					inUse = true
					break
				}
			}
			if inUse {
				break
			}
		}

		tags = append(tags, TagResponse{
			Name:        tag.Name,
			Description: tag.Description,
			InUse:       inUse,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"tags": tags,
	})
}

// handleCreateTagger 创建新的tagger
func (s *AdminServer) handleCreateTagger(c *gin.Context) {
	var req TaggerResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 验证必要字段
	if req.Name == "" || req.Type == "" || req.Tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, type, and tag are required"})
		return
	}

	// 检查tagger名称是否已存在
	for _, existing := range s.config.Tagging.Taggers {
		if existing.Name == req.Name {
			c.JSON(http.StatusConflict, gin.H{"error": "Tagger with this name already exists"})
			return
		}
	}

	// 创建新的tagger配置
	newTagger := config.TaggerConfig{
		Name:        req.Name,
		Type:        req.Type,
		Tag:         req.Tag,
		BuiltinType: req.BuiltinType,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		Config:      req.Config,
	}

	// 验证配置
	if err := validateTaggerConfig(newTagger); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tagger configuration: " + err.Error()})
		return
	}

	// 添加到配置
	s.config.Tagging.Taggers = append(s.config.Tagging.Taggers, newTagger)

	// 保存配置到文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
		// 如果保存失败，回滚内存中的配置
		s.config.Tagging.Taggers = s.config.Tagging.Taggers[:len(s.config.Tagging.Taggers)-1]
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
	}

	// 重新初始化tagging系统
	if err := s.taggingManager.Initialize(&s.config.Tagging); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize tagger: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tagger created successfully"})
}

// handleUpdateTagger 更新existing tagger
func (s *AdminServer) handleUpdateTagger(c *gin.Context) {
	name := c.Param("name")
	
	var req TaggerResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 查找要更新的tagger
	found := false
	var originalConfig config.TaggerConfig
	for i, tagger := range s.config.Tagging.Taggers {
		if tagger.Name == name {
			// 保存原始配置用于回滚
			originalConfig = tagger
			// 创建新的配置
			newTaggerConfig := config.TaggerConfig{
				Name:        req.Name,
				Type:        req.Type,
				Tag:         req.Tag,
				BuiltinType: req.BuiltinType,
				Enabled:     req.Enabled,
				Priority:    req.Priority,
				Config:      req.Config,
			}
			
			// 验证新配置
			if err := validateTaggerConfig(newTaggerConfig); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tagger configuration: " + err.Error()})
				return
			}
			
			// 更新配置
			s.config.Tagging.Taggers[i] = newTaggerConfig
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tagger not found"})
		return
	}

	// 保存配置到文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
		// 如果保存失败，回滚内存中的配置
		for i, tagger := range s.config.Tagging.Taggers {
			if tagger.Name == req.Name {
				s.config.Tagging.Taggers[i] = originalConfig
				break
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
	}

	// 重新初始化tagging系统
	if err := s.taggingManager.Initialize(&s.config.Tagging); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tagger: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tagger updated successfully"})
}

// handleDeleteTagger 删除tagger
func (s *AdminServer) handleDeleteTagger(c *gin.Context) {
	name := c.Param("name")

	// 查找并删除tagger
	found := false
	var deletedTagger config.TaggerConfig
	var deletedIndex int
	for i, tagger := range s.config.Tagging.Taggers {
		if tagger.Name == name {
			// 保存被删除的tagger用于回滚
			deletedTagger = tagger
			deletedIndex = i
			// 删除tagger
			s.config.Tagging.Taggers = append(s.config.Tagging.Taggers[:i], s.config.Tagging.Taggers[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tagger not found"})
		return
	}

	// 保存配置到文件
	if err := config.SaveConfig(s.config, s.configFilePath); err != nil {
		// 如果保存失败，回滚内存中的配置
		newTaggers := make([]config.TaggerConfig, len(s.config.Tagging.Taggers)+1)
		copy(newTaggers[:deletedIndex], s.config.Tagging.Taggers[:deletedIndex])
		newTaggers[deletedIndex] = deletedTagger
		copy(newTaggers[deletedIndex+1:], s.config.Tagging.Taggers[deletedIndex:])
		s.config.Tagging.Taggers = newTaggers
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration: " + err.Error()})
		return
	}

	// 重新初始化tagging系统
	if err := s.taggingManager.Initialize(&s.config.Tagging); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tagger: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tagger deleted successfully"})
}

// validateTaggerConfig 验证tagger配置
func validateTaggerConfig(tagger config.TaggerConfig) error {
	// 基本字段验证
	if tagger.Name == "" || tagger.Type == "" || tagger.Tag == "" {
		return fmt.Errorf("name, type and tag are required")
	}
	
	if tagger.Type == "builtin" && tagger.BuiltinType == "" {
		return fmt.Errorf("builtin_type is required for builtin taggers")
	}
	
	if tagger.Type == "starlark" {
		if script, ok := tagger.Config["script"].(string); !ok || script == "" {
			if scriptFile, ok := tagger.Config["script_file"].(string); !ok || scriptFile == "" {
				return fmt.Errorf("script or script_file is required for starlark taggers")
			}
		}
	}
	
	return nil
}