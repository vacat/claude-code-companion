package logger

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