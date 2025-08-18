package conversion

import (
	"testing"

	"claude-proxy/internal/config"
	"claude-proxy/internal/logger"
)

// Helper function to create a test logger
func createTestLogger(t *testing.T) *logger.Logger {
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none", // Use "none" to disable file logging
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return log
}

func TestPythonJSONFixer_DetectPythonStyle(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Simple Python dict",
			input:    "{'content': 'test', 'id': '1', 'status': 'pending'}",
			expected: true,
		},
		{
			name:     "TodoWrite format",
			input:    "{'content': '创建项目结构和主程序文件', 'id': '1', 'status': 'in_progress'}",
			expected: true,
		},
		{
			name:     "Array with Python dict",
			input:    "[{'content': 'test', 'id': '1'}]",
			expected: true,
		},
		{
			name:     "Partial Python dict",
			input:    "'content': 'test',",
			expected: true,
		},
		{
			name:     "Valid JSON",
			input:    `{"content": "test", "id": "1", "status": "pending"}`,
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Simple string",
			input:    "test",
			expected: false,
		},
		{
			name:     "Number",
			input:    "123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.DetectPythonStyle(tt.input)
			if result != tt.expected {
				t.Errorf("DetectPythonStyle() = %v, want %v for input: %s", result, tt.expected, tt.input)
			}
		})
	}
}

func TestPythonJSONFixer_FixPythonStyleJSON(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name          string
		input         string
		expectedFixed string
		expectedBool  bool
	}{
		{
			name:          "Simple Python dict",
			input:         "{'content': 'test', 'id': '1'}",
			expectedFixed: `{"content": "test", "id": "1"}`,
			expectedBool:  true,
		},
		{
			name:          "TodoWrite format",
			input:         "{'content': '创建项目结构和主程序文件', 'id': '1', 'status': 'in_progress'}",
			expectedFixed: `{"content": "创建项目结构和主程序文件", "id": "1", "status": "in_progress"}`,
			expectedBool:  true,
		},
		{
			name:          "Array with Python dict",
			input:         "[{'content': 'test', 'id': '1'}]",
			expectedFixed: `[{"content": "test", "id": "1"}]`,
			expectedBool:  true,
		},
		{
			name:          "Complex nested structure",
			input:         "{'todos': [{'content': 'task1', 'id': '1'}, {'content': 'task2', 'id': '2'}]}",
			expectedFixed: `{"todos": [{"content": "task1", "id": "1"}, {"content": "task2", "id": "2"}]}`,
			expectedBool:  true,
		},
		{
			name:          "Already valid JSON",
			input:         `{"content": "test", "id": "1"}`,
			expectedFixed: `{"content": "test", "id": "1"}`,
			expectedBool:  false,
		},
		{
			name:          "Empty string",
			input:         "",
			expectedFixed: "",
			expectedBool:  false,
		},
		{
			name:          "Invalid after conversion",
			input:         "{'unclosed': 'value",
			expectedFixed: "{'unclosed': 'value",
			expectedBool:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, wasFixed := fixer.FixPythonStyleJSON(tt.input)
			if wasFixed != tt.expectedBool {
				t.Errorf("FixPythonStyleJSON() wasFixed = %v, want %v", wasFixed, tt.expectedBool)
			}
			if fixed != tt.expectedFixed {
				t.Errorf("FixPythonStyleJSON() fixed = %v, want %v", fixed, tt.expectedFixed)
			}
		})
	}
}

func TestPythonJSONFixer_ShouldApplyFix(t *testing.T) {
	tests := []struct {
		name     string
		config   config.PythonJSONFixingConfig
		toolName string
		content  string
		expected bool
	}{
		{
			name: "Enabled for TodoWrite with Python content",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: true,
		},
		{
			name: "Disabled globally",
			config: config.PythonJSONFixingConfig{
				Enabled:     false,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: false,
		},
		{
			name: "Tool not in target list",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"OtherTool"},
			},
			toolName: "TodoWrite",
			content:  "{'content': 'test'}",
			expected: false,
		},
		{
			name: "Valid JSON content",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite"},
			},
			toolName: "TodoWrite",
			content:  `{"content": "test"}`,
			expected: false,
		},
		{
			name: "Multiple target tools",
			config: config.PythonJSONFixingConfig{
				Enabled:     true,
				TargetTools: []string{"TodoWrite", "OtherTool"},
			},
			toolName: "OtherTool",
			content:  "{'content': 'test'}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewPythonJSONFixerWithConfig(createTestLogger(t), tt.config)
			result := fixer.ShouldApplyFix(tt.toolName, tt.content)
			if result != tt.expected {
				t.Errorf("ShouldApplyFix() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPythonJSONFixer_isStructuralQuote(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		pos      int
		expected bool
	}{
		{
			name:     "Quote after opening brace",
			input:    "{'key'",
			pos:      1,
			expected: true,
		},
		{
			name:     "Quote before colon",
			input:    "'key':",
			pos:      4,
			expected: true,
		},
		{
			name:     "Quote after colon",
			input:    ": 'value'",
			pos:      2,
			expected: true,
		},
		{
			name:     "Quote before comma",
			input:    "'value',",
			pos:      6,
			expected: true,
		},
		{
			name:     "Quote at beginning of string",
			input:    "'content'",
			pos:      0,
			expected: true,
		},
		{
			name:     "Non-quote character",
			input:    "{'key'",
			pos:      0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			if tt.pos >= len(runes) {
				t.Errorf("Test setup error: pos %d >= len(runes) %d", tt.pos, len(runes))
				return
			}
			result := fixer.isStructuralQuote(runes, tt.pos)
			if result != tt.expected {
				t.Errorf("isStructuralQuote() = %v, want %v for input '%s' at pos %d", result, tt.expected, tt.input, tt.pos)
			}
		})
	}
}

func TestPythonJSONFixer_convertPythonQuotes(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple conversion",
			input:    "{'key': 'value'}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "Multiple keys",
			input:    "{'key1': 'value1', 'key2': 'value2'}",
			expected: `{"key1": "value1", "key2": "value2"}`,
		},
		{
			name:     "Array",
			input:    "['item1', 'item2']",
			expected: `["item1", "item2"]`,
		},
		{
			name:     "Nested structure",
			input:    "{'outer': {'inner': 'value'}}",
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "No quotes to convert",
			input:    `{"already": "valid"}`,
			expected: `{"already": "valid"}`,
		},
		{
			name:     "Mixed content",
			input:    "{'string': 'text', 'number': 123, 'bool': true}",
			expected: `{"string": "text", "number": 123, "bool": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.convertPythonQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("convertPythonQuotes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPythonJSONFixer_isValidJSON(t *testing.T) {
	fixer := NewPythonJSONFixer(createTestLogger(t))

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid object",
			input:    `{"key": "value"}`,
			expected: true,
		},
		{
			name:     "Valid array",
			input:    `["item1", "item2"]`,
			expected: true,
		},
		{
			name:     "Valid complex structure",
			input:    `{"todos": [{"content": "test", "id": "1"}]}`,
			expected: true,
		},
		{
			name:     "Invalid JSON - Python style",
			input:    "{'key': 'value'}",
			expected: false,
		},
		{
			name:     "Invalid JSON - malformed",
			input:    `{"key": "value"`,
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Simple string",
			input:    "test",
			expected: false,
		},
		{
			name:     "Valid number",
			input:    "123",
			expected: true,
		},
		{
			name:     "Valid boolean",
			input:    "true",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixer.isValidJSON(tt.input)
			if result != tt.expected {
				t.Errorf("isValidJSON() = %v, want %v for input: %s", result, tt.expected, tt.input)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkPythonJSONFixer_DetectPythonStyle(b *testing.B) {
	logConfig := logger.LogConfig{Level: "error", LogRequestTypes: "none", LogRequestBody: "none", LogResponseBody: "none", LogDirectory: "none"}
	log, _ := logger.NewLogger(logConfig)
	fixer := NewPythonJSONFixer(log)
	input := "{'content': 'test content with some length', 'id': '1', 'status': 'in_progress'}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fixer.DetectPythonStyle(input)
	}
}

func BenchmarkPythonJSONFixer_FixPythonStyleJSON(b *testing.B) {
	logConfig := logger.LogConfig{Level: "error", LogRequestTypes: "none", LogRequestBody: "none", LogResponseBody: "none", LogDirectory: "none"}
	log, _ := logger.NewLogger(logConfig)
	fixer := NewPythonJSONFixer(log)
	input := "{'todos': [{'content': 'task1', 'id': '1', 'status': 'pending'}, {'content': 'task2', 'id': '2', 'status': 'in_progress'}]}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fixer.FixPythonStyleJSON(input)
	}
}