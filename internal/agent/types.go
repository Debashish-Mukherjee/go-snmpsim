package agent

import (
	"sync/atomic"
)

// Atomic wrapper for uint64 without importing sync/atomic separately
type atomicCounter struct {
	value int64
}

// Add increments the counter
func (ac *atomicCounter) Add(delta int64) {
	atomic.AddInt64(&ac.value, delta)
}

// Load returns the current value
func (ac *atomicCounter) Load() int64 {
	return atomic.LoadInt64(&ac.value)
}

// Store sets the counter value
func (ac *atomicCounter) Store(val int64) {
	atomic.StoreInt64(&ac.value, val)
}
