// Package buffer provides low-latency, thread-safe, lock-free memory ring buffer
// primitives and data structures with zero allocations on the hot path.
//
// The package implements two specialized circular queue primitives:
//  1. SPSCRingBuffer: Optimized for Single-Producer Single-Consumer pipelines.
//     Maximizes L1D cache spatial locality through a dense memory layout.
//  2. MPMCRingBuffer: Multi-Producer Multi-Consumer queue based on Vyukov's algorithm.
//     Eliminates False Sharing between concurrent CPU cores via Cache Line Padding (64B/128B).
package buffer

import (
	"errors"
	"math/bits"
	"sync/atomic"

	"golang.org/x/sys/cpu"
)

var (
	// ErrInvalidCapacity is returned when the requested buffer capacity is zero.
	ErrInvalidCapacity = errors.New("buffer capacity must be greater than zero")

	// ErrCapacityTooLarge is returned when the requested capacity exceeds maxCapacity (2^62).
	ErrCapacityTooLarge = errors.New("capacity exceeds maximum allowed size")

	// ErrBufferFull is returned on a non-blocking push when the buffer is at capacity.
	ErrBufferFull = errors.New("ring buffer is full, item dropped")

	// ErrBufferEmpty is returned on a non-blocking pop when the buffer contains no items.
	ErrBufferEmpty = errors.New("ring buffer is empty")

	// ErrBufferClosed is returned when an operation is attempted on a shut-down buffer.
	ErrBufferClosed = errors.New("ring buffer is closed")
)

// maxCapacity defines the maximum allowable capacity for the ring buffer (2^62).
// This upper bound prevents potential uint64 overflow during power-of-two bit-shift rounding.
const maxCapacity uint64 = 1 << 62

// SlotPadded represents an individual storage cell within the MPMCRingBuffer array.
// Each slot is isolated into its own CPU cache line via cpu.CacheLinePad (64B on x86-64, 128B on ARM64)
// to completely mitigate False Sharing (Cache Line Bouncing) between concurrent producers and consumers.
type SlotPadded[T any] struct {
	// sequence serves as an atomic state barrier coordinating lock-free MPMC operations.
	sequence atomic.Uint64
	// val holds the underlying payload data.
	val T
	// _ guarantees that SlotPadded size strictly exceeds the L1 Data Cache line size.
	_ cpu.CacheLinePad
}

// MPMCRingBuffer is a thread-safe, lock-free circular queue designed for multiple producers
// and multiple consumers based on Dmitry Vyukov's algorithm.
//
// USAGE:
// Use MPMCRingBuffer when multiple goroutines concurrently invoke Push and/or Pop operations.
//
// MEMORY FOOTPRINT & TRADE-OFFS:
//   - Memory Alignment: Head, tail, mask, and slots fields are aligned on distinct cache lines.
//   - Trade-off: Higher memory footprint (each slot occupies >=64B/128B) traded for the total
//     elimination of interconnect bus locking under high contention.
type MPMCRingBuffer[T any] struct {
	// Cache Line 1: Consumer State (Consumer Hot Path)
	head atomic.Uint64
	_    cpu.CacheLinePad

	// Cache Line 2: Producer State (Producer Hot Path)
	tail atomic.Uint64
	_    cpu.CacheLinePad

	// Cache Line 3: Read-Only Configuration & Flags
	mask  uint64
	state atomic.Uint32
	_     cpu.CacheLinePad

	// Buffer Storage Array (Array of isolated cache-aligned slots)
	slots []SlotPadded[T]
}

// SPSCRingBuffer is an ultra-low-latency lock-free circular queue strictly designed for a SINGLE producer
// and a SINGLE consumer (Single-Producer Single-Consumer).
//
// USAGE:
// Use SPSCRingBuffer for 1:1 concurrent pipelines (e.g., event-loop thread -> worker thread).
// WARNING: Calling Push or Pop concurrently from multiple goroutines causes severe data races and corruption!
//
// MEMORY FOOTPRINT & PERFORMANCE:
//   - Spatial Locality: The ring slice holds a dense vector []T without internal slot padding,
//     maximizing hardware L1D prefetcher efficiency.
//   - Shadow Counters: headCache and tailCache reside within local core cache lines,
//     minimizing atomic cross-core reads over the CPU interconnect.
type SPSCRingBuffer[T any] struct {
	// Cache Line 1: Producer Hot Path
	tail      atomic.Uint64
	headCache uint64 // Local non-atomic shadow copy of head to reduce RFO traffic
	_         cpu.CacheLinePad

	// Cache Line 2: Consumer Hot Path
	head      atomic.Uint64
	tailCache uint64 // Local non-atomic shadow copy of tail to reduce RFO traffic
	_         cpu.CacheLinePad

	// Cache Line 3: Read-Only Configuration
	mask  uint64
	state atomic.Uint32
	_     cpu.CacheLinePad

	// Cache Line 4: Dense Memory Array (Dense Data Vector)
	ring []T
}

// NewSPSCRingBuffer constructs a new SPSCRingBuffer instance for single-producer single-consumer pipelines.
//
// If capacity is not a power of two (2^n), it is automatically rounded up to the next power of two
// to allow replacing costly DIV/IDIV instructions with fast bitwise AND operations.
//
// Returns ErrInvalidCapacity if capacity is 0, or ErrCapacityTooLarge if capacity exceeds 2^62.
func NewSPSCRingBuffer[T any](capacity uint64) (*SPSCRingBuffer[T], error) {
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}
	if capacity > maxCapacity {
		return nil, ErrCapacityTooLarge
	}

	// Round up to nearest power of two: O(1) via hardware LZCNT/CLZ instructions.
	if (capacity & (capacity - 1)) != 0 {
		capacity = 1 << uint64(bits.Len64(capacity-1))
	}

	return &SPSCRingBuffer[T]{
		mask: capacity - 1,
		ring: make([]T, capacity),
	}, nil
}

// NewMPMCRingBuffer constructs a new MPMCRingBuffer instance based on Vyukov's algorithm.
//
// Pre-initializes sequence barriers for every slot to match their respective array indices.
//
// Returns ErrInvalidCapacity if capacity is 0, or ErrCapacityTooLarge if capacity exceeds 2^62.
func NewMPMCRingBuffer[T any](capacity uint64) (*MPMCRingBuffer[T], error) {
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}
	if capacity > maxCapacity {
		return nil, ErrCapacityTooLarge
	}

	if (capacity & (capacity - 1)) != 0 {
		capacity = 1 << uint64(bits.Len64(capacity-1))
	}

	slots := make([]SlotPadded[T], capacity)

	// Local re-slicing eliminates runtime bounds checking (panicIndex)
	// inside the loop within Go compiler's generated assembly.
	subSlots := slots[:capacity]
	for i := uint64(0); i < capacity; i++ {
		subSlots[i].sequence.Store(i)
	}

	return &MPMCRingBuffer[T]{
		mask:  capacity - 1,
		slots: slots,
	}, nil
}
