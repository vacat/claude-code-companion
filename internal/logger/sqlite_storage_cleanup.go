package logger

import (
	"fmt"
	"time"
)

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

// CleanupLogsByDays removes log entries older than the specified number of days
func (s *SQLiteStorage) CleanupLogsByDays(days int) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var deleteSQL string
	var result interface{}
	var err error

	if days == 0 {
		// Delete all logs
		deleteSQL = "DELETE FROM request_logs"
		result, err = s.db.Exec(deleteSQL)
	} else {
		// Delete logs older than specified days
		cutoffTime := time.Now().AddDate(0, 0, -days)
		deleteSQL = "DELETE FROM request_logs WHERE timestamp < ?"
		result, err = s.db.Exec(deleteSQL, cutoffTime)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to cleanup logs: %v", err)
	}

	var rowsAffected int64
	switch v := result.(type) {
	case interface{ RowsAffected() (int64, error) }:
		rowsAffected, _ = v.RowsAffected()
	}
	
	// Run VACUUM to reclaim space after deletion
	if rowsAffected > 0 {
		if _, err := s.db.Exec("VACUUM"); err != nil {
			fmt.Printf("Failed to vacuum database: %v\n", err)
		}
	}

	return rowsAffected, nil
}