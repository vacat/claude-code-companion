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
	data := s.mergeTemplateData(c, "taggers", map[string]interface{}{
		"Title": "Tagger Management",
	})
	s.renderHTML(c, "taggers.html", data)
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

	// 使用公共的配置更新函数
	err := s.updateConfigWithRollback(
		// 更新函数
		func() error {
			s.config.Tagging.Taggers = append(s.config.Tagging.Taggers, newTagger)
			return nil
		},
		// 回滚函数
		func() error {
			if len(s.config.Tagging.Taggers) > 0 {
				s.config.Tagging.Taggers = s.config.Tagging.Taggers[:len(s.config.Tagging.Taggers)-1]
			}
			return nil
		},
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	var found bool
	var originalConfig config.TaggerConfig
	
	// 使用公共的配置更新函数
	err := s.updateConfigWithRollback(
		// 更新函数
		func() error {
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
						return fmt.Errorf("invalid tagger configuration: %v", err)
					}
					
					// 更新配置
					s.config.Tagging.Taggers[i] = newTaggerConfig
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("tagger not found")
			}
			return nil
		},
		// 回滚函数
		func() error {
			for i, tagger := range s.config.Tagging.Taggers {
				if tagger.Name == req.Name {
					s.config.Tagging.Taggers[i] = originalConfig
					break
				}
			}
			return nil
		},
	)

	if err != nil {
		if err.Error() == "tagger not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tagger not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
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

	var found bool
	var deletedTagger config.TaggerConfig
	var deletedIndex int
	
	// 使用公共的配置更新函数
	err := s.updateConfigWithRollback(
		// 更新函数
		func() error {
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
				return fmt.Errorf("tagger not found")
			}
			return nil
		},
		// 回滚函数
		func() error {
			newTaggers := make([]config.TaggerConfig, len(s.config.Tagging.Taggers)+1)
			copy(newTaggers[:deletedIndex], s.config.Tagging.Taggers[:deletedIndex])
			newTaggers[deletedIndex] = deletedTagger
			copy(newTaggers[deletedIndex+1:], s.config.Tagging.Taggers[deletedIndex:])
			s.config.Tagging.Taggers = newTaggers
			return nil
		},
	)

	if err != nil {
		if err.Error() == "tagger not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tagger not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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