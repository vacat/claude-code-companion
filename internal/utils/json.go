package utils

import (
	"encoding/json"
)

// ExtractStringField extracts a string field from JSON data
func ExtractStringField(data []byte, field string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}

	if value, ok := parsed[field].(string); ok {
		return value, nil
	}

	return "", nil
}

// ExtractNestedStringField extracts a nested string field from JSON data
// path should be like ["metadata", "user_id"]
func ExtractNestedStringField(data []byte, path []string) (string, error) {
	if len(data) == 0 || len(path) == 0 {
		return "", nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}

	current := parsed
	for _, key := range path[:len(path)-1] {
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return "", nil
		}
	}

	finalKey := path[len(path)-1]
	if value, ok := current[finalKey].(string); ok {
		return value, nil
	}

	return "", nil
}

// ExtractModelFromRequestBody extracts the model name from request body JSON
func ExtractModelFromRequestBody(body string) string {
	if body == "" {
		return ""
	}
	
	model, _ := ExtractStringField([]byte(body), "model")
	return model
}


// TruncateBody truncates body content to specified length
func TruncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "... [truncated]"
}

// ThinkingInfo contains extracted thinking mode information
type ThinkingInfo struct {
	Enabled     bool `json:"enabled"`
	BudgetTokens int  `json:"budget_tokens"`
}

// ExtractThinkingInfo extracts thinking mode information from request body
func ExtractThinkingInfo(body string) (*ThinkingInfo, error) {
	if body == "" {
		return nil, nil
	}
	
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	
	thinkingField, exists := parsed["thinking"]
	if !exists {
		return nil, nil
	}
	
	thinkingMap, ok := thinkingField.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	
	info := &ThinkingInfo{}
	
	// Check if thinking is enabled
	if typeValue, ok := thinkingMap["type"].(string); ok && typeValue == "enabled" {
		info.Enabled = true
	}
	
	// Extract budget tokens
	if budgetValue, ok := thinkingMap["budget_tokens"]; ok {
		if budgetFloat, ok := budgetValue.(float64); ok {
			info.BudgetTokens = int(budgetFloat)
		}
	}
	
	return info, nil
}