package conversion

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"claude-proxy/internal/config"
	"claude-proxy/internal/logger"
)

// PythonJSONFixer handles the conversion of Python-style dictionary syntax to valid JSON
type PythonJSONFixer struct {
	logger *logger.Logger
	config config.PythonJSONFixingConfig
}

// NewPythonJSONFixer creates a new PythonJSONFixer instance
func NewPythonJSONFixer(log *logger.Logger) *PythonJSONFixer {
	// Default configuration
	defaultConfig := config.PythonJSONFixingConfig{
		Enabled:      true,
		TargetTools:  []string{"TodoWrite"},
		DebugLogging: false,
		MaxAttempts:  3,
	}
	
	return &PythonJSONFixer{
		logger: log,
		config: defaultConfig,
	}
}

// NewPythonJSONFixerWithConfig creates a new PythonJSONFixer instance with custom configuration
func NewPythonJSONFixerWithConfig(log *logger.Logger, cfg config.PythonJSONFixingConfig) *PythonJSONFixer {
	return &PythonJSONFixer{
		logger: log,
		config: cfg,
	}
}

// FixPythonStyleJSON attempts to fix Python-style JSON syntax and returns the fixed string
// along with a boolean indicating whether any fixes were applied
func (f *PythonJSONFixer) FixPythonStyleJSON(input string) (string, bool) {
	if !f.DetectPythonStyle(input) {
		return input, false
	}

	if f.config.DebugLogging {
		f.logger.Debug("Detected Python-style JSON, attempting to fix", map[string]interface{}{
			"original": input,
		})
	}

	fixed := f.convertPythonQuotes(input)
	
	// Validate the fixed JSON
	if f.isValidJSON(fixed) {
		if f.config.DebugLogging {
			f.logger.Debug("Successfully fixed Python-style JSON", map[string]interface{}{
				"original": input,
				"fixed":    fixed,
			})
		}
		return fixed, true
	}

	if f.config.DebugLogging {
		f.logger.Debug("Failed to fix Python-style JSON - result is not valid JSON", map[string]interface{}{
			"original": input,
			"fixed":    fixed,
		})
	}
	
	return input, false
}

// DetectPythonStyle checks if the input contains Python-style dictionary syntax
func (f *PythonJSONFixer) DetectPythonStyle(input string) bool {
	// Common patterns that indicate Python-style syntax
	patterns := []string{
		`{'[^']*':\s*'[^']*'}`,           // Single key-value pair: {'key': 'value'}
		`'[^']*':\s*'[^']*'`,             // Key-value fragment: 'key': 'value'
		`\[{'[^']*':\s*'[^']*'`,          // Array start: [{'key': 'value'
		`{'[^']*':\s*'[^']*',`,           // Multiple keys start: {'key': 'value',
		`',\s*'[^']*':\s*'[^']*'`,        // Middle key-value: , 'key': 'value'
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return true
		}
	}

	return false
}

// convertPythonQuotes converts Python-style single quotes to JSON double quotes
func (f *PythonJSONFixer) convertPythonQuotes(input string) string {
	runes := []rune(input)
	result := make([]rune, 0, len(runes))
	
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\'' && f.isStructuralQuote(runes, i) {
			// Convert structural single quotes to double quotes
			result = append(result, '"')
		} else {
			result = append(result, runes[i])
		}
	}
	
	return string(result)
}

// isStructuralQuote determines if a quote at the given position is structural (part of JSON syntax)
// rather than content within a string value
func (f *PythonJSONFixer) isStructuralQuote(runes []rune, pos int) bool {
	if pos >= len(runes) || runes[pos] != '\'' {
		return false
	}

	// Look at the context around the quote to determine if it's structural
	// This is a simplified heuristic that works for TodoWrite tool format
	
	// Find the preceding non-whitespace character
	prevNonSpace := -1
	for i := pos - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			prevNonSpace = i
			break
		}
	}
	
	// Find the following non-whitespace character
	nextNonSpace := -1
	for i := pos + 1; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) {
			nextNonSpace = i
			break
		}
	}
	
	// Structural quotes typically appear:
	// 1. After {, [, or , (start of key or value)
	// 2. Before :, ,, }, or ] (end of key or value)
	// 3. At the beginning of input (pos == 0)
	// 4. At the end of input
	
	// Special case: beginning of input or after whitespace from beginning
	if prevNonSpace == -1 {
		return true
	}
	
	if prevNonSpace >= 0 {
		prevChar := runes[prevNonSpace]
		if prevChar == '{' || prevChar == '[' || prevChar == ',' || prevChar == ':' {
			return true
		}
	}
	
	if nextNonSpace >= 0 {
		nextChar := runes[nextNonSpace]
		if nextChar == ':' || nextChar == ',' || nextChar == '}' || nextChar == ']' {
			return true
		}
	}
	
	// Special case: end of input
	if nextNonSpace == -1 {
		return true
	}
	
	return false
}

// isValidJSON checks if the given string is valid JSON
func (f *PythonJSONFixer) isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// ShouldApplyFix determines if the fix should be applied based on tool name and other criteria
func (f *PythonJSONFixer) ShouldApplyFix(toolName string, content string) bool {
	// Check if fixing is enabled
	if !f.config.Enabled {
		return false
	}
	
	// Check if the tool is in the target tools list
	for _, targetTool := range f.config.TargetTools {
		if targetTool == toolName {
			// Only apply if we detect Python-style syntax
			return f.DetectPythonStyle(content)
		}
	}
	
	return false
}