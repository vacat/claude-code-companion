package conversion

// Convert 转换 OpenAI 响应为 Anthropic 格式
func (c *ResponseConverter) Convert(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error) {
	if isStreaming {
		return c.convertStreamingResponse(openaiResp, ctx)
	}
	return c.convertNonStreamingResponse(openaiResp, ctx)
}