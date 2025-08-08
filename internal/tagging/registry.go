package tagging

import (
	"fmt"
	"sync"
)

// TagRegistry 管理所有注册的tag和tagger
type TagRegistry struct {
	tags     map[string]*Tag
	taggers  map[string]Tagger
	mu       sync.RWMutex
}

// NewTagRegistry 创建新的Tag注册表
func NewTagRegistry() *TagRegistry {
	return &TagRegistry{
		tags:    make(map[string]*Tag),
		taggers: make(map[string]Tagger),
	}
}

// RegisterTag 注册一个新的tag
func (tr *TagRegistry) RegisterTag(name, description string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	if _, exists := tr.tags[name]; exists {
		return fmt.Errorf("tag '%s' already registered", name)
	}
	
	tr.tags[name] = &Tag{
		Name:        name,
		Description: description,
	}
	
	return nil
}

// RegisterTagger 注册一个新的tagger
func (tr *TagRegistry) RegisterTagger(tagger Tagger) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	name := tagger.Name()
	if _, exists := tr.taggers[name]; exists {
		return fmt.Errorf("tagger '%s' already registered", name)
	}
	
	// 自动注册tagger对应的tag
	tag := tagger.Tag()
	if _, exists := tr.tags[tag]; !exists {
		tr.tags[tag] = &Tag{
			Name:        tag,
			Description: fmt.Sprintf("Tag from tagger '%s'", name),
		}
	}
	
	tr.taggers[name] = tagger
	return nil
}

// UnregisterTagger 注销一个tagger
func (tr *TagRegistry) UnregisterTagger(name string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	delete(tr.taggers, name)
}

// Clear 清理所有注册的taggers和tags
func (tr *TagRegistry) Clear() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	
	tr.tags = make(map[string]*Tag)
	tr.taggers = make(map[string]Tagger)
}

// GetTag 获取指定的tag信息
func (tr *TagRegistry) GetTag(name string) (*Tag, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	tag, exists := tr.tags[name]
	return tag, exists
}

// GetTagger 获取指定的tagger
func (tr *TagRegistry) GetTagger(name string) (Tagger, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	tagger, exists := tr.taggers[name]
	return tagger, exists
}

// ListTags 列出所有注册的tag
func (tr *TagRegistry) ListTags() []*Tag {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	tags := make([]*Tag, 0, len(tr.tags))
	for _, tag := range tr.tags {
		tags = append(tags, tag)
	}
	
	return tags
}

// ListTaggers 列出所有注册的tagger
func (tr *TagRegistry) ListTaggers() []Tagger {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	taggers := make([]Tagger, 0, len(tr.taggers))
	for _, tagger := range tr.taggers {
		taggers = append(taggers, tagger)
	}
	
	return taggers
}

// ValidateTag 验证tag名称是否存在
func (tr *TagRegistry) ValidateTag(name string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	
	_, exists := tr.tags[name]
	return exists
}