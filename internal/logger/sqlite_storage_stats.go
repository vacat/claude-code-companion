package logger

import (
	"time"
)

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