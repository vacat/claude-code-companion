package conversion

import (
	"claude-code-companion/internal/logger"
)

// AggregatedMessage represents a complete message after aggregating all OpenAI SSE chunks
type AggregatedMessage struct {
	ID           string                  `json:"id"`
	Model        string                  `json:"model"`
	TextContent  string                  `json:"text_content"`
	ToolCalls    []AggregatedToolCall    `json:"tool_calls"`
	FinishReason string                  `json:"finish_reason"`
	Usage        *OpenAIUsage            `json:"usage,omitempty"`
}

// AggregatedToolCall represents a complete tool call after aggregation
type AggregatedToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // Complete JSON string
}

// ConversionResult contains the converted event sequence
type ConversionResult struct {
	Events   []AnthropicSSEEvent   `json:"events"`
	Metadata ConversionMetadata    `json:"metadata"`
}

// AnthropicSSEEvent represents a single Anthropic SSE event
type AnthropicSSEEvent struct {
	Type string      `json:"type"` // "message_start", "content_block_start", etc.
	Data interface{} `json:"data"` // Specific event data
}

// ConversionMetadata contains metadata about the conversion process
type ConversionMetadata struct {
	OriginalChunkCount int    `json:"original_chunk_count"`
	ProcessingNotes    string `json:"processing_notes,omitempty"`
}

// MessageAggregator aggregates OpenAI chunks into a complete message
type MessageAggregator struct {
	logger      *logger.Logger
	pythonFixer *PythonJSONFixer
}

// NewMessageAggregator creates a new MessageAggregator
func NewMessageAggregator(logger *logger.Logger) *MessageAggregator {
	return &MessageAggregator{
		logger:      logger,
		pythonFixer: NewPythonJSONFixer(logger),
	}
}

// UnifiedConverter converts aggregated messages to Anthropic event sequences
type UnifiedConverter struct {
	logger *logger.Logger
}

// NewUnifiedConverter creates a new UnifiedConverter
func NewUnifiedConverter(logger *logger.Logger) *UnifiedConverter {
	return &UnifiedConverter{
		logger: logger,
	}
}