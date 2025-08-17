package logger

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

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
		attempt_number INTEGER DEFAULT 1,
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
		content_type_override TEXT DEFAULT '',
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

	// Add content_type_override column to existing tables if it doesn't exist
	alterTableSQL2 := `ALTER TABLE request_logs ADD COLUMN content_type_override TEXT DEFAULT '';`
	_, err2 := s.db.Exec(alterTableSQL2)
	// Ignore error if column already exists
	if err2 != nil && !strings.Contains(err2.Error(), "duplicate column name") {
		fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err2)
	}

	// Add model rewrite fields to existing tables if they don't exist
	modelRewriteColumns := []string{
		`ALTER TABLE request_logs ADD COLUMN original_model TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN rewritten_model TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN model_rewrite_applied BOOLEAN DEFAULT FALSE;`,
	}
	
	for _, alterSQL := range modelRewriteColumns {
		_, err := s.db.Exec(alterSQL)
		// Ignore error if column already exists
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err)
		}
	}

	// Add attempt_number column for retry tracking
	attemptNumberSQL := `ALTER TABLE request_logs ADD COLUMN attempt_number INTEGER DEFAULT 1;`
	_, err3 := s.db.Exec(attemptNumberSQL)
	if err3 != nil && !strings.Contains(err3.Error(), "duplicate column name") {
		fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err3)
	}

	// Add thinking mode fields
	thinkingColumns := []string{
		`ALTER TABLE request_logs ADD COLUMN thinking_enabled BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE request_logs ADD COLUMN thinking_budget_tokens INTEGER DEFAULT 0;`,
	}
	
	for _, alterSQL := range thinkingColumns {
		_, err := s.db.Exec(alterSQL)
		// Ignore error if column already exists
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err)
		}
	}

	// Add original/final request/response fields for before/after comparison
	beforeAfterColumns := []string{
		`ALTER TABLE request_logs ADD COLUMN original_request_url TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN original_request_headers TEXT DEFAULT '{}';`,
		`ALTER TABLE request_logs ADD COLUMN original_request_body TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN original_response_headers TEXT DEFAULT '{}';`,
		`ALTER TABLE request_logs ADD COLUMN original_response_body TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN final_request_url TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN final_request_headers TEXT DEFAULT '{}';`,
		`ALTER TABLE request_logs ADD COLUMN final_request_body TEXT DEFAULT '';`,
		`ALTER TABLE request_logs ADD COLUMN final_response_headers TEXT DEFAULT '{}';`,
		`ALTER TABLE request_logs ADD COLUMN final_response_body TEXT DEFAULT '';`,
	}
	
	for _, alterSQL := range beforeAfterColumns {
		_, err := s.db.Exec(alterSQL)
		// Ignore error if column already exists
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			fmt.Printf("Note: %v (this is expected if upgrading from older version)\n", err)
		}
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