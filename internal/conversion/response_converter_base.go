package conversion

import (
	"claude-code-companion/internal/logger"
)

// ResponseConverter 响应转换器 - 基于参考实现
type ResponseConverter struct {
	logger    *logger.Logger
	sseParser *SSEParser
}

// NewResponseConverter 创建响应转换器
func NewResponseConverter(logger *logger.Logger) *ResponseConverter {
	return &ResponseConverter{
		logger:    logger,
		sseParser: NewSSEParser(logger),
	}
}