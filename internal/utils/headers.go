package utils

import (
	"net/http"
	"strings"
)

// CopyRequestHeaders copies HTTP headers from source to destination, excluding specified headers
func CopyRequestHeaders(src http.Header, exclude []string) http.Header {
	dest := make(http.Header)
	
	// Create a map for fast exclusion lookup
	excludeMap := make(map[string]bool)
	for _, header := range exclude {
		excludeMap[strings.ToLower(header)] = true
	}
	
	for key, values := range src {
		if !excludeMap[strings.ToLower(key)] {
			for _, value := range values {
				dest.Add(key, value)
			}
		}
	}
	
	return dest
}

// SetAuthHeaderForEndpoint sets the appropriate authentication header based on endpoint auth configuration
func SetAuthHeaderForEndpoint(req *http.Request, authType, authValue string) {
	if authType == "api_key" {
		req.Header.Set("x-api-key", authValue)
	} else {
		// For auth_token, we need to determine if it already has "Bearer " prefix
		if strings.HasPrefix(authValue, "Bearer ") {
			req.Header.Set("Authorization", authValue)
		} else {
			req.Header.Set("Authorization", "Bearer "+authValue)
		}
	}
}

// ExtractRequestHeaders extracts relevant headers from HTTP request, excluding sensitive ones
func ExtractRequestHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	
	// Headers to exclude from extraction
	excludeHeaders := map[string]bool{
		"authorization":   true,
		"x-api-key":      true,
		"content-length": true,
		"host":           true,
	}

	for key, values := range headers {
		lowKey := strings.ToLower(key)
		if !excludeHeaders[lowKey] && len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

// HeadersToMap converts http.Header to map[string]string (takes first value if multiple)
func HeadersToMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}