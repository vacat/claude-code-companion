package logger

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage provides SQLite-based log storage with automatic cleanup
type SQLiteStorage struct {
	db             *sql.DB
	mutex          sync.RWMutex
	logDir         string
	dbPath         string
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
}

// NewSQLiteStorage creates a new SQLite-based log storage
func NewSQLiteStorage(logDir string) (*SQLiteStorage, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}
	
	dbPath := filepath.Join(logDir, "logs.db")
	
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	storage := &SQLiteStorage{
		db:          db,
		logDir:      logDir,
		dbPath:      dbPath,
		stopCleanup: make(chan struct{}),
	}

	if err := storage.initDatabase(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// Start background cleanup routine (runs every 24 hours)
	storage.startBackgroundCleanup()

	return storage, nil
}

// initDatabase creates the necessary tables and indexes
func (s *SQLiteStorage) initDatabase() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS request_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		request_id TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		status_code INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		request_headers TEXT DEFAULT '{}',
		request_body TEXT DEFAULT '',
		request_body_size INTEGER DEFAULT 0,
		response_headers TEXT DEFAULT '{}',
		response_body TEXT DEFAULT '',
		response_body_size INTEGER DEFAULT 0,
		is_streaming BOOLEAN DEFAULT FALSE,
		model TEXT DEFAULT '',
		error TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Add tags column to existing tables if it doesn't exist
	alterTableSQL := `ALTER TABLE request_logs ADD COLUMN tags TEXT DEFAULT '[]';`
	_, err := s.db.Exec(alterTableSQL)
	// Ignore error if column already exists (SQLite returns error for existing column)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		// Only log other errors, not duplicate column errors
		fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err)
	}

	// Create indexes for better query performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);",
		"CREATE INDEX IF NOT EXISTS idx_request_logs_endpoint ON request_logs(endpoint);",
		"CREATE INDEX IF NOT EXISTS idx_request_logs_status_code ON request_logs(status_code);",
		"CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at);",
		"CREATE INDEX IF NOT EXISTS idx_request_logs_failed ON request_logs(status_code) WHERE status_code >= 400;",
	}

	for _, indexSQL := range indexes {
		if _, err := s.db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	return nil
}

// SaveLog saves a log entry to the database
func (s *SQLiteStorage) SaveLog(log *RequestLog) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Marshal headers to JSON
	requestHeaders, _ := json.Marshal(log.RequestHeaders)
	responseHeaders, _ := json.Marshal(log.ResponseHeaders)
	
	// Marshal tags to JSON
	tags, _ := json.Marshal(log.Tags)

	insertSQL := `
	INSERT INTO request_logs (
		timestamp, request_id, endpoint, method, path, status_code, duration_ms,
		request_headers, request_body, request_body_size,
		response_headers, response_body, response_body_size,
		is_streaming, model, error, tags
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(insertSQL,
		log.Timestamp, log.RequestID, log.Endpoint, log.Method, log.Path,
		log.StatusCode, log.DurationMs,
		string(requestHeaders), log.RequestBody, log.RequestBodySize,
		string(responseHeaders), log.ResponseBody, log.ResponseBodySize,
		log.IsStreaming, log.Model, log.Error, string(tags),
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
		SELECT timestamp, request_id, endpoint, method, path, status_code, duration_ms,
			   request_headers, request_body, request_body_size,
			   response_headers, response_body, response_body_size,
			   is_streaming, model, error, tags
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

		err := rows.Scan(
			&log.Timestamp, &log.RequestID, &log.Endpoint, &log.Method, &log.Path,
			&log.StatusCode, &log.DurationMs,
			&requestHeaders, &log.RequestBody, &log.RequestBodySize,
			&responseHeaders, &log.ResponseBody, &log.ResponseBodySize,
			&log.IsStreaming, &log.Model, &log.Error, &tagsJSON,
		)
		if err != nil {
			continue // Skip invalid rows
		}

		// Unmarshal JSON headers
		json.Unmarshal([]byte(requestHeaders), &log.RequestHeaders)
		json.Unmarshal([]byte(responseHeaders), &log.ResponseHeaders)
		
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
		SELECT timestamp, request_id, endpoint, method, path, status_code, duration_ms,
			   request_headers, request_body, request_body_size,
			   response_headers, response_body, response_body_size,
			   is_streaming, model, error, tags
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

		err := rows.Scan(
			&log.Timestamp, &log.RequestID, &log.Endpoint, &log.Method, &log.Path,
			&log.StatusCode, &log.DurationMs,
			&requestHeaders, &log.RequestBody, &log.RequestBodySize,
			&responseHeaders, &log.ResponseBody, &log.ResponseBodySize,
			&log.IsStreaming, &log.Model, &log.Error, &tagsJSON,
		)
		if err != nil {
			continue // Skip invalid rows
		}

		// Unmarshal JSON headers
		json.Unmarshal([]byte(requestHeaders), &log.RequestHeaders)
		json.Unmarshal([]byte(responseHeaders), &log.ResponseHeaders)
		
		// Unmarshal JSON tags
		if tagsJSON != "" && tagsJSON != "null" {
			json.Unmarshal([]byte(tagsJSON), &log.Tags)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// startBackgroundCleanup starts a background goroutine to clean up old logs
func (s *SQLiteStorage) startBackgroundCleanup() {
	// Clean up immediately on startup
	go s.cleanupOldLogs()

	// Set up periodic cleanup (every 24 hours)
	s.cleanupTicker = time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				s.cleanupOldLogs()
			case <-s.stopCleanup:
				return
			}
		}
	}()
}

// cleanupOldLogs removes log entries older than 30 days
func (s *SQLiteStorage) cleanupOldLogs() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Delete logs older than 30 days
	cutoffTime := time.Now().AddDate(0, 0, -30) // 30 days ago
	deleteSQL := "DELETE FROM request_logs WHERE timestamp < ?"
	
	result, err := s.db.Exec(deleteSQL, cutoffTime)
	if err != nil {
		fmt.Printf("Failed to cleanup old logs: %v\n", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("Cleaned up %d old log entries (older than %s)\n", rowsAffected, cutoffTime.Format("2006-01-02"))

		// Run VACUUM to reclaim space after deletion
		if _, err := s.db.Exec("VACUUM"); err != nil {
			fmt.Printf("Failed to vacuum database: %v\n", err)
		}
	}
}

// GetStats returns database statistics
func (s *SQLiteStorage) GetStats() (map[string]interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := make(map[string]interface{})

	// Total logs count
	var totalLogs int
	s.db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&totalLogs)
	stats["total_logs"] = totalLogs

	// Failed logs count
	var failedLogs int
	s.db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE status_code >= 400 OR error != ''").Scan(&failedLogs)
	stats["failed_logs"] = failedLogs

	// Oldest log timestamp
	var oldestLog time.Time
	err := s.db.QueryRow("SELECT MIN(timestamp) FROM request_logs").Scan(&oldestLog)
	if err == nil {
		stats["oldest_log"] = oldestLog
	}

	// Database size (approximate)
	var pageCount, pageSize int
	s.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	s.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	stats["db_size_bytes"] = pageCount * pageSize

	return stats, nil
}

// Close closes the database connection and stops cleanup routines
func (s *SQLiteStorage) Close() error {
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}
	
	select {
	case s.stopCleanup <- struct{}{}:
	default:
	}

	return s.db.Close()
}