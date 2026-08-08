package buffer_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DenysSkobalo/ringo/buffer"
)

// TestNewRingBuffer_Validation verifies strict capacity validation
// and initial state invariants of the ring buffer.
func TestNewRingBuffer_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		capacity int
		wantErr  error
	}{
		{name: "negative capacity", capacity: -5, wantErr: buffer.ErrInvalidCapacity},
		{name: "zero capacity", capacity: 0, wantErr: buffer.ErrInvalidCapacity},
		{name: "valid capacity", capacity: 10, wantErr: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf, err := buffer.NewRingBuffer[int](tt.capacity)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewRingBuffer(%d) error = %v, wantErr %v", tt.capacity, err, tt.wantErr)
			}

			if tt.wantErr == nil {
				if buf == nil {
					t.Fatal("expected non-nil buffer instance")
				}
				if buf.Cap() != tt.capacity {
					t.Fatalf("expected Cap() = %d, got %d", tt.capacity, buf.Cap())
				}
				if buf.Len() != 0 {
					t.Fatalf("expected Len() = 0, got %d", buf.Len())
				}
			}
		})
	}
}

// TestRingBuffer_TryPushPop verifies sequential FIFO behavior,
// buffer overflow handling, and empty buffer underflow checks.
func TestRingBuffer_TryPushPop(t *testing.T) {
	t.Parallel()

	const capacity = 3
	buf, err := buffer.NewRingBuffer[int](capacity)
	if err != nil {
		t.Fatalf("unexpected error creating buffer: %v", err)
	}

	// 1. Initial read on an empty buffer
	if _, ok := buf.TryPop(); ok {
		t.Fatal("expected TryPop on empty buffer to return false")
	}

	// 2. Fill buffer to capacity
	for i := 1; i <= capacity; i++ {
		if err := buf.TryPush(i * 10); err != nil {
			t.Fatalf("unexpected error pushing %d: %v", i*10, err)
		}
		if buf.Len() != i {
			t.Fatalf("expected Len() = %d, got %d", i, buf.Len())
		}
	}

	// 3. Overflow write attempt
	if err := buf.TryPush(999); !errors.Is(err, buffer.ErrBufferFull) {
		t.Fatalf("expected ErrBufferFull on overflow push, got %v", err)
	}

	// 4. Sequential FIFO drain
	for i := 1; i <= capacity; i++ {
		val, ok := buf.TryPop()
		if !ok {
			t.Fatalf("expected TryPop() to succeed for item %d", i)
		}
		if val != i*10 {
			t.Fatalf("expected value %d, got %d", i*10, val)
		}
	}

	// 5. Verify empty state after full drain
	if _, ok := buf.TryPop(); ok {
		t.Fatal("expected TryPop to return false after fully popping buffer")
	}
}

// TestRingBuffer_CloseAndDrain verifies shutdown lifecycle semantics:
// Close() idempotency, TryPush rejection, and residual item draining.
func TestRingBuffer_CloseAndDrain(t *testing.T) {
	t.Parallel()

	buf, err := buffer.NewRingBuffer[string](2)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}

	_ = buf.TryPush("item1")
	_ = buf.TryPush("item2")

	// Initial Close call
	if err := buf.Close(); err != nil {
		t.Fatalf("unexpected error closing buffer: %v", err)
	}

	// Verify idempotency (subsequent Close calls return nil without panicking)
	if err := buf.Close(); err != nil {
		t.Fatalf("expected second Close() to be idempotent no-op, got %v", err)
	}

	// Verify writes to closed buffer are rejected
	if err := buf.TryPush("item3"); !errors.Is(err, buffer.ErrBufferClosed) {
		t.Fatalf("expected ErrBufferClosed, got %v", err)
	}

	// Draining: verify buffered items remain readable
	val1, ok1 := buf.TryPop()
	if !ok1 || val1 != "item1" {
		t.Fatalf("expected ('item1', true), got (%q, %t)", val1, ok1)
	}

	val2, ok2 := buf.TryPop()
	if !ok2 || val2 != "item2" {
		t.Fatalf("expected ('item2', true), got (%q, %t)", val2, ok2)
	}

	// Verify empty reads return false after full drain
	if _, ok := buf.TryPop(); ok {
		t.Fatal("expected TryPop on fully drained closed buffer to return false")
	}
}

// TestRingBuffer_ConcurrentRace stress-tests thread safety under heavy
// parallel load (M-Producers / N-Consumers) to detect data races via TSan (-race).
func TestRingBuffer_ConcurrentRace(t *testing.T) {
	t.Parallel()

	const (
		capacity  = 100
		producers = 10
		consumers = 10
		items     = 1000
	)

	buf, err := buffer.NewRingBuffer[int](capacity)
	if err != nil {
		t.Fatalf("failed to create buffer: %v", err)
	}

	var (
		wg           sync.WaitGroup
		pushedCount  atomic.Int64
		poppedCount  atomic.Int64
		droppedCount atomic.Int64
	)

	// Spawn concurrent producer goroutines (M producers)
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < items; i++ {
				if err := buf.TryPush(i); err == nil {
					pushedCount.Add(1)
				} else if errors.Is(err, buffer.ErrBufferFull) {
					droppedCount.Add(1)
				}
			}
		}()
	}

	// Spawn concurrent consumer goroutines (N consumers)
	for c := 0; c < consumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < items; i++ {
				if _, ok := buf.TryPop(); ok {
					poppedCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// Final residual drain
	for {
		if _, ok := buf.TryPop(); ok {
			poppedCount.Add(1)
		} else {
			break
		}
	}

	_ = buf.Close()

	t.Logf("Concurrent Test Results: Pushed=%d, Popped=%d, Dropped=%d",
		pushedCount.Load(), poppedCount.Load(), droppedCount.Load())

	// Invariant check: total pushed items must equal total popped items
	if pushedCount.Load() != poppedCount.Load() {
		t.Fatalf("mismatch between total pushed (%d) and popped (%d) items",
			pushedCount.Load(), poppedCount.Load())
	}
}

// BenchmarkRingBuffer_TryPushPop measures non-blocking throughput
// and verifies zero heap allocation overhead (0 B/op target).
func BenchmarkRingBuffer_TryPushPop(b *testing.B) {
	buf, err := buffer.NewRingBuffer[int](1024)
	if err != nil {
		b.Fatalf("failed to create buffer: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := buf.TryPush(i); err == nil {
			buf.TryPop()
		}
	}
}
