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
	// Complete patterns that indicate Python-style syntax
	completePatterns := []string{
		`{'[^']*':\s*'[^']*'}`,           // Single key-value pair: {'key': 'value'}
		`'[^']*':\s*'[^']*'`,             // Key-value fragment: 'key': 'value'
		`\[{'[^']*':\s*'[^']*'`,          // Array start: [{'key': 'value'
		`{'[^']*':\s*'[^']*',`,           // Multiple keys start: {'key': 'value',
		`',\s*'[^']*':\s*'[^']*'`,        // Middle key-value: , 'key': 'value'
	}

	// Check complete patterns first
	for _, pattern := range completePatterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return true
		}
	}

	// SSE Stream fragment patterns - for handling split content across multiple chunks
	streamPatterns := []string{
		`^{'$`,                          // Opening dict: {'
		`^'[^']*':\s*'[^']*$`,          // Key with incomplete value: 'key': 'val
		`^[^']*',\s*'[^']*$`,           // Value end with new key start: ue', 'newkey
		`^':\s*'[^']*$`,                // Continuation after key: ': 'value
		`^'[^']*':\s*'$`,               // Key with colon: 'key': '
		`^'[^']*'},\s*{'$`,             // Object transition: 'value'}, {'
		`'},\s*{'[^']*$`,               // Object end to start: }, {'key
		`^[^']*'},\s*{'$`,              // Value end to new object: alue'}, {'
		`^'[^']*':\s*'$`,               // Key with start of value: 'key': '
		`':\s*'[^']*',?\s*$`,           // Key-value completion: ': 'value',
		`^[^']*',\s*'$`,                // Value end, new key start: value', '
		`^\s*'[^']*':\s*'$`,            // Key with colon space: 'status': '
		`^'[^']*'}\s*$`,                // Key with object end: 'value'}
		`^\s*'[^']*':\s*$`,             // Key with colon: 'status':
		`'}\s*,?\s*$`,                  // Object closing: '}
		`^'\s*$`,                       // Just a quote: '
		`[a-zA-Z0-9_]+'\s*:\s*'[a-zA-Z0-9]`,  // Simple key-value pattern: key': 'val
	}

	// Check stream fragment patterns
	for _, pattern := range streamPatterns {
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

	// Enhanced heuristic for SSE stream fragments and complete structures
	
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
	
	// Special case: beginning of input (common in SSE fragments)
	if prevNonSpace == -1 {
		return true
	}
	
	// Check preceding context
	if prevNonSpace >= 0 {
		prevChar := runes[prevNonSpace]
		if prevChar == '{' || prevChar == '[' || prevChar == ',' || prevChar == ':' {
			return true
		}
	}
	
	// Check following context
	if nextNonSpace >= 0 {
		nextChar := runes[nextNonSpace]
		if nextChar == ':' || nextChar == ',' || nextChar == '}' || nextChar == ']' {
			return true
		}
	}
	
	// Special case: end of input (common in SSE fragments)
	if nextNonSpace == -1 {
		return true
	}
	
	// Enhanced heuristics for SSE stream fragments
	// If we have very limited context, be more permissive
	inputLength := len(runes)
	
	// For very short fragments (likely SSE chunks), assume structural if it contains typical patterns
	if inputLength <= 5 {
		return true
	}
	
	// Look for common SSE fragment patterns around the quote
	startIdx := pos - 2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := pos + 3
	if endIdx > len(runes) {
		endIdx = len(runes)
	}
	
	context := string(runes[startIdx:endIdx])
	
	// Common SSE fragment patterns that indicate structural quotes
	fragmentPatterns := []string{
		"{'",     // Start of dict
		"':",     // Key separator  
		"',",     // Value separator
		"'}",     // End of dict entry
		"' ",     // Quote with space (often structural)
	}
	
	for _, pattern := range fragmentPatterns {
		if strings.Contains(context, pattern) {
			return true
		}
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