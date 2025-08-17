package logger

import (
	"database/sql"
	"sync"
	"time"
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