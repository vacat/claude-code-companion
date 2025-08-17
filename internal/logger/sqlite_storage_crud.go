package logger

import (
	"encoding/json"
	"fmt"
)

// SaveLog saves a log entry to the database
func (s *SQLiteStorage) SaveLog(log *RequestLog) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Marshal headers to JSON
	requestHeaders, _ := json.Marshal(log.RequestHeaders)
	responseHeaders, _ := json.Marshal(log.ResponseHeaders)
	originalRequestHeaders, _ := json.Marshal(log.OriginalRequestHeaders)
	originalResponseHeaders, _ := json.Marshal(log.OriginalResponseHeaders)
	finalRequestHeaders, _ := json.Marshal(log.FinalRequestHeaders)
	finalResponseHeaders, _ := json.Marshal(log.FinalResponseHeaders)
	
	// Marshal tags to JSON
	tags, _ := json.Marshal(log.Tags)

	insertSQL := `
	INSERT INTO request_logs (
		timestamp, request_id, endpoint, method, path, status_code, duration_ms, attempt_number,
		request_headers, request_body, request_body_size,
		response_headers, response_body, response_body_size,
		is_streaming, model, error, tags, content_type_override,
		original_model, rewritten_model, model_rewrite_applied,
		thinking_enabled, thinking_budget_tokens,
		original_request_url, original_request_headers, original_request_body,
		original_response_headers, original_response_body,
		final_request_url, final_request_headers, final_request_body,
		final_response_headers, final_response_body
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(insertSQL,
		log.Timestamp, log.RequestID, log.Endpoint, log.Method, log.Path,
		log.StatusCode, log.DurationMs, log.AttemptNumber,
		string(requestHeaders), log.RequestBody, log.RequestBodySize,
		string(responseHeaders), log.ResponseBody, log.ResponseBodySize,
		log.IsStreaming, log.Model, log.Error, string(tags), log.ContentTypeOverride,
		log.OriginalModel, log.RewrittenModel, log.ModelRewriteApplied,
		log.ThinkingEnabled, log.ThinkingBudgetTokens,
		log.OriginalRequestURL, string(originalRequestHeaders), log.OriginalRequestBody,
		string(originalResponseHeaders), log.OriginalResponseBody,
		log.FinalRequestURL, string(finalRequestHeaders), log.FinalRequestBody,
		string(finalResponseHeaders), log.FinalResponseBody,
	)

	if err != nil {
		// Log error but don't fail the application
		fmt.Printf("Failed to save log to database: %v\n", err)
	}
}

// GetLogs retrieves logs with pagination and optional filtering
func (s *SQLiteStorage) GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if failedOnly {
		whereClause += " AND (status_code >= 400 OR error != '')"
	}

	// Get total count
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM request_logs %s", whereClause)
	var total int
	err := s.db.QueryRow(countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %v", err)
	}

	// Get logs with pagination
	querySQL := fmt.Sprintf(`
		SELECT timestamp, request_id, endpoint, method, path, status_code, duration_ms, attempt_number,
			   request_headers, request_body, request_body_size,
			   response_headers, response_body, response_body_size,
			   is_streaming, model, error, tags, content_type_override,
			   original_model, rewritten_model, model_rewrite_applied,
			   thinking_enabled, thinking_budget_tokens,
			   original_request_url, original_request_headers, original_request_body,
			   original_response_headers, original_response_body,
			   final_request_url, final_request_headers, final_request_body,
			   final_response_headers, final_response_body
		FROM request_logs %s
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?`, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := s.db.Query(querySQL, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %v", err)
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		log := &RequestLog{}
		var requestHeaders, responseHeaders, tagsJSON string
		var originalRequestHeaders, originalResponseHeaders string
		var finalRequestHeaders, finalResponseHeaders string

		err := rows.Scan(
			&log.Timestamp, &log.RequestID, &log.Endpoint, &log.Method, &log.Path,
			&log.StatusCode, &log.DurationMs, &log.AttemptNumber,
			&requestHeaders, &log.RequestBody, &log.RequestBodySize,
			&responseHeaders, &log.ResponseBody, &log.ResponseBodySize,
			&log.IsStreaming, &log.Model, &log.Error, &tagsJSON, &log.ContentTypeOverride,
			&log.OriginalModel, &log.RewrittenModel, &log.ModelRewriteApplied,
			&log.ThinkingEnabled, &log.ThinkingBudgetTokens,
			&log.OriginalRequestURL, &originalRequestHeaders, &log.OriginalRequestBody,
			&originalResponseHeaders, &log.OriginalResponseBody,
			&log.FinalRequestURL, &finalRequestHeaders, &log.FinalRequestBody,
			&finalResponseHeaders, &log.FinalResponseBody,
		)
		if err != nil {
			continue // Skip invalid rows
		}

		// Unmarshal JSON headers
		json.Unmarshal([]byte(requestHeaders), &log.RequestHeaders)
		json.Unmarshal([]byte(responseHeaders), &log.ResponseHeaders)
		json.Unmarshal([]byte(originalRequestHeaders), &log.OriginalRequestHeaders)
		json.Unmarshal([]byte(originalResponseHeaders), &log.OriginalResponseHeaders)
		json.Unmarshal([]byte(finalRequestHeaders), &log.FinalRequestHeaders)
		json.Unmarshal([]byte(finalResponseHeaders), &log.FinalResponseHeaders)
		
		// Unmarshal JSON tags
		if tagsJSON != "" && tagsJSON != "null" {
			json.Unmarshal([]byte(tagsJSON), &log.Tags)
		}

		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetAllLogsByRequestID retrieves all log entries for a specific request ID
func (s *SQLiteStorage) GetAllLogsByRequestID(requestID string) ([]*RequestLog, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	querySQL := `
		SELECT timestamp, request_id, endpoint, method, path, status_code, duration_ms, attempt_number,
			   request_headers, request_body, request_body_size,
			   response_headers, response_body, response_body_size,
			   is_streaming, model, error, tags, content_type_override,
			   original_model, rewritten_model, model_rewrite_applied,
			   thinking_enabled, thinking_budget_tokens,
			   original_request_url, original_request_headers, original_request_body,
			   original_response_headers, original_response_body,
			   final_request_url, final_request_headers, final_request_body,
			   final_response_headers, final_response_body
		FROM request_logs
		WHERE request_id = ?
		ORDER BY timestamp ASC`

	rows, err := s.db.Query(querySQL, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by request ID: %v", err)
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		log := &RequestLog{}
		var requestHeaders, responseHeaders, tagsJSON string
		var originalRequestHeaders, originalResponseHeaders string
		var finalRequestHeaders, finalResponseHeaders string

		err := rows.Scan(
			&log.Timestamp, &log.RequestID, &log.Endpoint, &log.Method, &log.Path,
			&log.StatusCode, &log.DurationMs, &log.AttemptNumber,
			&requestHeaders, &log.RequestBody, &log.RequestBodySize,
			&responseHeaders, &log.ResponseBody, &log.ResponseBodySize,
			&log.IsStreaming, &log.Model, &log.Error, &tagsJSON, &log.ContentTypeOverride,
			&log.OriginalModel, &log.RewrittenModel, &log.ModelRewriteApplied,
			&log.ThinkingEnabled, &log.ThinkingBudgetTokens,
			&log.OriginalRequestURL, &originalRequestHeaders, &log.OriginalRequestBody,
			&originalResponseHeaders, &log.OriginalResponseBody,
			&log.FinalRequestURL, &finalRequestHeaders, &log.FinalRequestBody,
			&finalResponseHeaders, &log.FinalResponseBody,
		)
		if err != nil {
			continue // Skip invalid rows
		}

		// Unmarshal JSON headers
		json.Unmarshal([]byte(requestHeaders), &log.RequestHeaders)
		json.Unmarshal([]byte(responseHeaders), &log.ResponseHeaders)
		json.Unmarshal([]byte(originalRequestHeaders), &log.OriginalRequestHeaders)
		json.Unmarshal([]byte(originalResponseHeaders), &log.OriginalResponseHeaders)
		json.Unmarshal([]byte(finalRequestHeaders), &log.FinalRequestHeaders)
		json.Unmarshal([]byte(finalResponseHeaders), &log.FinalResponseHeaders)
		
		// Unmarshal JSON tags
		if tagsJSON != "" && tagsJSON != "null" {
			json.Unmarshal([]byte(tagsJSON), &log.Tags)
		}

		logs = append(logs, log)
	}

	return logs, nil
}