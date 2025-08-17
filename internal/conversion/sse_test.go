package conversion

import (
	"strings"
	"testing"
)

func TestSSEStreamConversion(t *testing.T) {
	// 使用nil logger来避免配置问题
	converter := NewResponseConverter(nil)

	// 模拟 OpenAI 流式响应
	openaiSSE := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-3.5-turbo","choices":[{"delta":{"content":"Hello"},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-3.5-turbo","choices":[{"delta":{"content":" World"},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-3.5-turbo","choices":[{"delta":{},"index":0,"finish_reason":"stop"}]}

data: [DONE]

`

	// 创建转换上下文
	ctx := &ConversionContext{
		EndpointType: "openai",
		StreamState: nil,
	}

	// 执行转换
	result, err := converter.convertStreamingResponse([]byte(openaiSSE), ctx)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	resultStr := string(result)
	t.Logf("Conversion result:\n%s", resultStr)

	// 验证转换结果
	expectedEvents := []string{
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_stop",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(resultStr, expected) {
			t.Errorf("Expected event '%s' not found in result", expected)
		}
	}

	// 验证结果是SSE格式
	if !strings.Contains(resultStr, "data:") {
		t.Error("Result should be in SSE format with 'data:' prefixes")
	}

	// 验证包含结束标记
	if !strings.Contains(resultStr, "[DONE]") {
		t.Error("Result should contain [DONE] marker")
	}
}

func TestSSEParser(t *testing.T) {
	// 使用nil logger来避免配置问题
	parser := NewSSEParser(nil)

	// 测试SSE格式验证
	validSSE := `data: {"test": "value"}

data: [DONE]

`
	if !parser.ValidateSSEFormat([]byte(validSSE)) {
		t.Error("Valid SSE format should be recognized")
	}

	invalidSSE := `{"test": "value"}`
	if parser.ValidateSSEFormat([]byte(invalidSSE)) {
		t.Error("Invalid SSE format should not be recognized")
	}

	// 测试SSE解析
	openaiSSE := `data: {"id":"test","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"test","object":"chat.completion.chunk","choices":[{"delta":{"content":" World"},"index":0}]}

data: [DONE]

`
	chunks, err := parser.ParseSSEStream([]byte(openaiSSE))
	if err != nil {
		t.Fatalf("Failed to parse SSE stream: %v", err)
	}

	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}

	// 验证第一个chunk
	if chunks[0].Choices[0].Delta.Content != "Hello" {
		t.Errorf("First chunk content mismatch: expected 'Hello', got '%v'", chunks[0].Choices[0].Delta.Content)
	}

	// 验证第二个chunk
	if chunks[1].Choices[0].Delta.Content != " World" {
		t.Errorf("Second chunk content mismatch: expected ' World', got '%v'", chunks[1].Choices[0].Delta.Content)
	}
}