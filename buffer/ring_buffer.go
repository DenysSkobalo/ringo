// Package buffer provides low-latency, thread-safe, non-blocking and blocking
// memory channel primitives and structures for concurrent data pipelines.
package buffer

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	// ErrInvalidCapacity indicates that the requested buffer capacity is non-positive.
	ErrInvalidCapacity = errors.New("buffer capacity must be greater than zero")

	// ErrBufferFull indicates that a non-blocking push failed because the buffer is at capacity.
	ErrBufferFull = errors.New("ring buffer is full, item dropped")

	// ErrBufferEmpty indicates that a non-blocking pop failed because the buffer contains no items.
	ErrBufferEmpty = errors.New("ring buffer is empty")

	// ErrBufferClosed indicates that an operation was attempted on a shut-down buffer.
	ErrBufferClosed = errors.New("ring buffer is closed")
)

// RingBuffer represents a thread-safe, non-blocking generic ring buffer.
type RingBuffer[T any] struct {
	ch       chan T       // Internal buffered channel (pointer to runtime.hchan)
	mu       sync.RWMutex // Read-Write lock protecting state transitions and TOCTOU races
	isClosed atomic.Bool  // Atomic flag indicating shutdown status
}

// NewRingBuffer constructs a RingBuffer with the specified capacity limit.
// Returns ErrInvalidCapacity if capacity is less than or equal to zero.
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	return &RingBuffer[T]{
		ch: make(chan T, capacity),
	}, nil
}

// TryPush attempts to insert an item into the buffer without blocking.
// Returns ErrBufferFull if the buffer is capacity-constrained.
// Returns ErrBufferClosed if the buffer is shut down.
func (b *RingBuffer[T]) TryPush(val T) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.isClosed.Load() {
		return ErrBufferClosed
	}

	return TrySet(b.ch, val)
}

// TryPop attempts to retrieve an item from the buffer without blocking.
// Returns (value, true) on success, or (zero, false) if empty or closed.
// This operation is entirely lock-free.
func (b *RingBuffer[T]) TryPop() (T, bool) {
	return TryGet(b.ch)
}

// Len returns the current number of queued elements (O(1) lock-free snapshot).
func (b *RingBuffer[T]) Len() int {
	return len(b.ch)
}

// Cap returns the total capacity of the buffer (O(1) lock-free read).
func (b *RingBuffer[T]) Cap() int {
	return cap(b.ch)
}

// Close safely terminates the buffer, preventing further write operations.
// Existing buffered items remain accessible via TryPop.
// Subsequent calls to Close are idempotent no-ops.
func (b *RingBuffer[T]) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isClosed.Load() {
		return nil
	}

	b.isClosed.Store(true)
	close(b.ch)
	return nil
}

// TrySet performs a zero-allocation, non-blocking send on the target channel.
// Returns ErrBufferFull if the channel buffer is at capacity.
func TrySet[T any](ch chan<- T, val T) error {
	select {
	case ch <- val:
		return nil
	default:
		return ErrBufferFull
	}
}

// TryGet performs a zero-allocation, non-blocking receive from the target channel.
// Returns (value, true) if an item was retrieved, or (zero, false) if empty or closed.
func TryGet[T any](ch <-chan T) (T, bool) {
	var zero T
	select {
	case val, open := <-ch:
		if !open {
			return zero, false
		}
		return val, true
	default:
		return zero, false
	}
}
