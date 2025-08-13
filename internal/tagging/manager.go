package tagging

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"claude-proxy/internal/config"
	"claude-proxy/internal/taggers/builtin"
	"claude-proxy/internal/taggers/starlark"
)

// Manager 管理整个tagging系统
type Manager struct {
	registry *TagRegistry
	pipeline *TaggerPipeline
	factory  *builtin.BuiltinTaggerFactory
	enabled  bool
}

// NewManager 创建tagging系统管理器
func NewManager() *Manager {
	return &Manager{
		registry: NewTagRegistry(),
		pipeline: NewTaggerPipeline(5 * time.Second), // 默认5秒超时
		factory:  builtin.NewBuiltinTaggerFactory(),
		enabled:  true, // tagging系统永远启用
	}
}

// Initialize 根据配置初始化tagging系统
func (m *Manager) Initialize(config *config.TaggingConfig) error {
	if config == nil {
		// 即使没有配置，tagging系统也保持启用状态，只是没有taggers
		m.enabled = true
		return nil
	}

	m.enabled = true // tagging系统永远启用

	// 清理之前的注册信息
	m.registry.Clear()

	// 设置pipeline超时时间
	timeout, err := time.ParseDuration(config.PipelineTimeout)
	if err != nil {
		return fmt.Errorf("invalid pipeline timeout: %v", err)
	}
	m.pipeline.SetTimeout(timeout)

	// 创建并注册所有tagger
	var taggers []Tagger
	for _, taggerConfig := range config.Taggers {
		if !taggerConfig.Enabled {
			continue // 跳过禁用的tagger
		}

		var tagger Tagger
		if taggerConfig.Type == "builtin" {
			tagger, err = m.factory.CreateTagger(
				taggerConfig.BuiltinType,
				taggerConfig.Name,
				taggerConfig.Tag,
				taggerConfig.Config,
			)
			if err != nil {
				return fmt.Errorf("failed to create builtin tagger '%s': %v", taggerConfig.Name, err)
			}
		} else if taggerConfig.Type == "starlark" {
			// 创建Starlark tagger
			var script string
			
			// 支持两种方式：script_file 或 script
			if scriptFile, ok := taggerConfig.Config["script_file"].(string); ok && scriptFile != "" {
				// 从文件读取脚本
				scriptBytes, readErr := os.ReadFile(scriptFile)
				if readErr != nil {
					return fmt.Errorf("starlark tagger '%s': failed to read script file '%s': %v", 
						taggerConfig.Name, scriptFile, readErr)
				}
				script = string(scriptBytes)
			} else if inlineScript, ok := taggerConfig.Config["script"].(string); ok && inlineScript != "" {
				// 使用内联脚本
				script = inlineScript
			} else {
				return fmt.Errorf("starlark tagger '%s': missing script or script_file config", taggerConfig.Name)
			}
			
			tagger = starlark.NewTagger(taggerConfig.Name, taggerConfig.Tag, script, timeout)
		} else {
			return fmt.Errorf("unknown tagger type: %s", taggerConfig.Type)
		}

		// 注册tagger到registry
		if err := m.registry.RegisterTagger(tagger); err != nil {
			return fmt.Errorf("failed to register tagger '%s': %v", taggerConfig.Name, err)
		}

		taggers = append(taggers, tagger)
	}

	// 设置pipeline中的tagger
	m.pipeline.SetTaggers(taggers)

	return nil
}

// ProcessRequest 处理HTTP请求，进行tag标记
func (m *Manager) ProcessRequest(req *http.Request) (*TaggedRequest, error) {
	if !m.enabled {
		// tagging系统被禁用，返回无tag的请求
		return &TaggedRequest{
			OriginalRequest: req,
			Tags:           []string{},
			TaggingTime:    time.Now(),
			TaggerResults:  []TaggerResult{},
		}, nil
	}

	return m.pipeline.ProcessRequest(req)
}

// IsEnabled 返回tagging系统是否启用
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// SetEnabled 设置tagging系统启用状态
func (m *Manager) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// GetRegistry 获取tag注册表
func (m *Manager) GetRegistry() *TagRegistry {
	return m.registry
}

// GetPipeline 获取tagger管道
func (m *Manager) GetPipeline() *TaggerPipeline {
	return m.pipeline
}

// GetFactory 获取内置tagger工厂
func (m *Manager) GetFactory() *builtin.BuiltinTaggerFactory {
	return m.factory
}

// ValidateTaggedEndpoints 验证带tag的endpoint配置
func (m *Manager) ValidateTaggedEndpoints(endpoints []TaggedEndpoint) error {
	if !m.enabled {
		return nil // tagging系统禁用时不验证
	}

	for i, endpoint := range endpoints {
		for j, tag := range endpoint.Tags {
			if !m.registry.ValidateTag(tag) {
				return fmt.Errorf("endpoint[%d] '%s': unknown tag '%s' at index %d", 
					i, endpoint.Name, tag, j)
			}
		}
	}

	return nil
}