package utils

import (
	"sync"
	"time"
)

// CircularBuffer represents a circular buffer for request records
type CircularBuffer struct {
	records   []RequestRecord
	size      int
	head      int
	count     int
	mutex     sync.RWMutex
	windowDur time.Duration
}

// RequestRecord represents a single request record
type RequestRecord struct {
	Timestamp time.Time
	Success   bool
	
	// 新增：请求ID（用于追踪失败原因）
	RequestID string
}

// NewCircularBuffer creates a new circular buffer with the specified size and time window
func NewCircularBuffer(size int, windowDuration time.Duration) *CircularBuffer {
	return &CircularBuffer{
		records:   make([]RequestRecord, size),
		size:      size,
		head:      0,
		count:     0,
		windowDur: windowDuration,
	}
}

// Add adds a new record to the circular buffer
func (cb *CircularBuffer) Add(record RequestRecord) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.records[cb.head] = record
	cb.head = (cb.head + 1) % cb.size

	if cb.count < cb.size {
		cb.count++
	}
}

// GetWindowStats returns statistics for records within the time window
func (cb *CircularBuffer) GetWindowStats(now time.Time) (total, failed int) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	cutoff := now.Add(-cb.windowDur)

	for i := 0; i < cb.count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		record := cb.records[idx]

		if record.Timestamp.Before(cutoff) {
			break // Records are sorted by time, so we can break early
		}

		total++
		if !record.Success {
			failed++
		}
	}

	return total, failed
}

// ShouldMarkInactive determines if the endpoint should be marked as inactive
// based on the failure pattern in the time window
func (cb *CircularBuffer) ShouldMarkInactive(now time.Time) bool {
	total, failed := cb.GetWindowStats(now)
	
	// Mark inactive if: more than 1 request in window AND all requests failed
	return total > 1 && failed == total
}

// Clear clears all records from the buffer
func (cb *CircularBuffer) Clear() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.count = 0
	cb.head = 0
}

// GetRecentFailureRequestIDs 获取时间窗口内的所有失败请求ID
func (cb *CircularBuffer) GetRecentFailureRequestIDs(now time.Time) []string {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	cutoff := now.Add(-cb.windowDur)
	var failureRequestIDs []string

	for i := 0; i < cb.count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		record := cb.records[idx]

		if record.Timestamp.Before(cutoff) {
			break
		}

		if !record.Success && record.RequestID != "" {
			failureRequestIDs = append(failureRequestIDs, record.RequestID)
		}
	}

	return failureRequestIDs
}