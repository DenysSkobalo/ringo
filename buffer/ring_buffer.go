// Package buffer provides low-latency, thread-safe, lock-free memory ring buffer
// primitives and data structures for high-throughput concurrent pipelines.
package buffer

import (
	"errors"
	"math/bits"
	"sync/atomic"
)

var (
	// ErrInvalidCapacity indicates that the requested buffer capacity is zero.
	ErrInvalidCapacity = errors.New("buffer capacity must be greater than zero")

	// ErrCapacityTooLarge indicates that the requested capacity exceeds maxCapacity.
	ErrCapacityTooLarge = errors.New("capacity exceeds maximum allowed size")

	// ErrBufferFull indicates that a non-blocking push operation failed because the buffer is at capacity.
	ErrBufferFull = errors.New("ring buffer is full, item dropped")

	// ErrBufferEmpty indicates that a non-blocking pop operation failed because the buffer contains no items.
	ErrBufferEmpty = errors.New("ring buffer is empty")

	// ErrBufferClosed indicates that an operation was attempted on a shut-down buffer.
	ErrBufferClosed = errors.New("ring buffer is closed")
)

// maxCapacity defines the maximum allowable capacity for the ring buffer (2^62).
// This upper bound prevents potential uint64 bit-shift overflow during power-of-two rounding.
const maxCapacity uint64 = 1 << 62

// Slot represents an individual storage cell within the ring buffer array.
// Each slot maintains an atomic sequence barrier to coordinate lock-free MPMC operations
// without data races between concurrent producers and consumers.
type Slot[T any] struct {
	sequence atomic.Uint64
	val      T
}

// RingBuffer represents a thread-safe, lock-free generic circular queue based on Vyukov's algorithm.
// Fields are explicitly padded with blank byte arrays to isolate high-frequency atomic state variables
// across distinct 64-byte CPU cache lines, mitigating False Sharing performance degradation under high contention.
type RingBuffer[T any] struct {
	// Cache Line 1: Consumer State (64-byte alignment)
	_    [56]byte
	head atomic.Uint64

	// Cache Line 2: Producer State (64-byte alignment)
	_    [56]byte
	tail atomic.Uint64

	// Cache Line 3: Read-Only Configuration & Atomic Flags (64-byte alignment)
	_     [56]byte
	mask  uint64
	state atomic.Uint32

	// Cache Line 4: Buffer Storage Array
	_     [48]byte
	slots []Slot[T]
}

// NewRingBuffer constructs a new Lock-Free RingBuffer instance with the requested capacity.
//
// If capacity is not a power of two (2^n), it is automatically rounded up to the next power of two.
// Pre-initializes all slot sequences to match their respective array indices to enable immediate writing.
//
// Returns ErrInvalidCapacity if capacity is 0, or ErrCapacityTooLarge if capacity > 2^62.
func NewRingBuffer[T any](capacity uint64) (*RingBuffer[T], error) {
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}
	if capacity > maxCapacity {
		return nil, ErrCapacityTooLarge
	}

	// Round up capacity to the nearest power of two if it isn't already one.
	if (capacity & (capacity - 1)) != 0 {
		capacity = 1 << uint64(bits.Len64(capacity-1))
	}

	mask := capacity - 1
	slots := make([]Slot[T], capacity)

	// Initialize each slot's sequence counter to its initial array index.
	for i := uint64(0); i < capacity; i++ {
		slots[i].sequence.Store(i)
	}

	return &RingBuffer[T]{
		mask:  mask,
		slots: slots,
	}, nil
}
