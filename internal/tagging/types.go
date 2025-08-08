package tagging

import (
	"claude-proxy/internal/interfaces"
)

// 重新导出接口以保持向后兼容
type Tagger = interfaces.Tagger
type Tag = interfaces.Tag
type TaggedRequest = interfaces.TaggedRequest
type TaggedEndpoint = interfaces.TaggedEndpoint
type TaggerResult = interfaces.TaggerResult

// TagMatcher 负责根据请求tags匹配合适的endpoint
type TagMatcher interface {
	MatchEndpoints(requestTags []string, endpoints []TaggedEndpoint) []TaggedEndpoint
}