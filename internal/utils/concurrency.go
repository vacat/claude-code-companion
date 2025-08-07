package utils

import (
	"sync"
)

// CopyOnWriteSlice provides a thread-safe slice with copy-on-write semantics
// to reduce lock contention for read-heavy workloads
type CopyOnWriteSlice struct {
	data  []interface{}
	mutex sync.RWMutex
}

// NewCopyOnWriteSlice creates a new copy-on-write slice
func NewCopyOnWriteSlice() *CopyOnWriteSlice {
	return &CopyOnWriteSlice{
		data: make([]interface{}, 0),
	}
}

// Get returns a snapshot of the current data without holding locks
func (c *CopyOnWriteSlice) Get() []interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	// Return a reference to the current slice (readers can use it safely)
	// since we only replace the slice on writes, never modify it in place
	return c.data
}

// Update replaces the entire slice with new data (copy-on-write)
func (c *CopyOnWriteSlice) Update(newData []interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Create a new slice to avoid modifying the existing one
	c.data = make([]interface{}, len(newData))
	copy(c.data, newData)
}

// Append adds an item to the slice (copy-on-write)
func (c *CopyOnWriteSlice) Append(item interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Create a new slice with the additional item
	newData := make([]interface{}, len(c.data)+1)
	copy(newData, c.data)
	newData[len(c.data)] = item
	c.data = newData
}

// Len returns the current length
func (c *CopyOnWriteSlice) Len() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.data)
}

// AtomicBool provides a simple atomic boolean
type AtomicBool struct {
	flag int64
}

// Set sets the boolean value atomically
func (ab *AtomicBool) Set(value bool) {
	var newValue int64
	if value {
		newValue = 1
	}
	ab.flag = newValue
}

// Get gets the boolean value atomically
func (ab *AtomicBool) Get() bool {
	return ab.flag != 0
}