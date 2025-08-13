package conversion

// Converter 定义转换器接口
type Converter interface {
	// 转换请求
	ConvertRequest(anthropicReq []byte, endpointType string) ([]byte, *ConversionContext, error)
	
	// 转换响应
	ConvertResponse(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error)
	
	// 检查是否需要转换
	ShouldConvert(endpointType string) bool
}

// ConversionContext 转换上下文
type ConversionContext struct {
	EndpointType    string                 // "anthropic" | "openai"
	ToolCallIDMap   map[string]string      // 工具调用ID映射 (Anthropic ID -> OpenAI ID)
	IsStreaming     bool                   // 是否为流式请求
	RequestHeaders  map[string]string      // 原始请求头
	StreamState     *StreamState           // 流式转换状态
	StopSequences   []string               // 请求中的停止序列，用于响应时检测
	// 注意：不包含模型映射，因为转换发生在模型重写之后
}

// StreamState 流式转换状态
type StreamState struct {
	ContentBlockIndex int                         // 当前内容块索引
	ToolCallStates    map[string]*ToolCallState   // 工具调用状态
	NextBlockIndex    int                         // 下一个块索引
	TextBlockStarted  bool                        // 文本块是否已开始
	MessageStarted    bool                        // 消息是否已开始
	PingSent          bool                        // ping事件是否已发送
}

// ToolCallState 工具调用状态
type ToolCallState struct {
	BlockIndex        int    // 内容块索引
	ID               string // 工具调用ID
	Name             string // 工具名称
	ArgumentsBuffer  string // 参数缓冲区（保留以兼容现有代码）
	JSONBuffer       *SimpleJSONBuffer // 简单JSON缓冲器
	Completed        bool   // 是否已完成
	Started          bool   // 是否已发送 content_block_start
	NameReceived     bool   // 是否已收到工具名称
}

// ConversionError 转换错误
type ConversionError struct {
	Type    string // "parse_error", "unsupported_feature", "tool_conversion_error"
	Message string
	Err     error
}

func (e *ConversionError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ConversionError) Unwrap() error {
	return e.Err
}

// NewConversionError 创建转换错误
func NewConversionError(errorType, message string, err error) *ConversionError {
	return &ConversionError{
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}