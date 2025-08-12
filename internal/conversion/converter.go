package conversion

import (
	"claude-proxy/internal/logger"
)

// DefaultConverter 默认转换器实现
type DefaultConverter struct {
	logger           *logger.Logger
	requestConverter *RequestConverter
	responseConverter *ResponseConverter
}

// NewConverter 创建新的转换器
func NewConverter(logger *logger.Logger) Converter {
	return &DefaultConverter{
		logger:            logger,
		requestConverter:  NewRequestConverter(logger),
		responseConverter: NewResponseConverter(logger),
	}
}

// ShouldConvert 检查是否需要转换
func (c *DefaultConverter) ShouldConvert(endpointType string) bool {
	return endpointType == "openai"
}

// ConvertRequest 转换请求
func (c *DefaultConverter) ConvertRequest(anthropicReq []byte, endpointType string) ([]byte, *ConversionContext, error) {
	if !c.ShouldConvert(endpointType) {
		return anthropicReq, nil, nil
	}

	c.logger.Debug("Starting request conversion for OpenAI endpoint")
	
	convertedReq, ctx, err := c.requestConverter.Convert(anthropicReq)
	if err != nil {
		c.logger.Error("Request conversion failed", err)
		return nil, nil, err
	}
	
	ctx.EndpointType = endpointType
	c.logger.Debug("Request conversion completed successfully")
	
	return convertedReq, ctx, nil
}

// ConvertResponse 转换响应
func (c *DefaultConverter) ConvertResponse(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error) {
	if ctx == nil || !c.ShouldConvert(ctx.EndpointType) {
		return openaiResp, nil
	}

	c.logger.Debug("Starting response conversion from OpenAI format")
	
	convertedResp, err := c.responseConverter.Convert(openaiResp, ctx, isStreaming)
	if err != nil {
		c.logger.Error("Response conversion failed", err)
		return nil, err
	}
	
	c.logger.Debug("Response conversion completed successfully")
	
	return convertedResp, nil
}