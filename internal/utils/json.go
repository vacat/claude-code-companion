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

// ExtractUserIDFromRequestBody extracts the user ID from request body JSON
func ExtractUserIDFromRequestBody(body []byte) string {
	userID, _ := ExtractNestedStringField(body, []string{"metadata", "user_id"})
	return userID
}

// TruncateBody truncates body content to specified length
func TruncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "... [truncated]"
}