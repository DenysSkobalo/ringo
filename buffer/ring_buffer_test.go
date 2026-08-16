package buffer_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/DenysSkobalo/ringo/buffer"
)

// ==============================================================================
// UNIT TESTS (CONSTRUCTORS & BOUNDARY CONDITIONS)
// ==============================================================================

func TestNewSPSCRingBuffer_Validation(t *testing.T) {
	tests := []struct {
		name         string
		capacity     uint64
		expectedErr  error
		expectNotNil bool
	}{
		{
			name:         "Zero capacity error",
			capacity:     0,
			expectedErr:  buffer.ErrInvalidCapacity,
			expectNotNil: false,
		},
		{
			name:         "Capacity exceeds max capacity error",
			capacity:     (1 << 62) + 1,
			expectedErr:  buffer.ErrCapacityTooLarge,
			expectNotNil: false,
		},
		{
			name:         "Valid power of two capacity",
			capacity:     16,
			expectedErr:  nil,
			expectNotNil: true,
		},
		{
			name:         "Valid non-power of two capacity (rounds up to 8)",
			capacity:     5,
			expectedErr:  nil,
			expectNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := buffer.NewSPSCRingBuffer[int](tt.capacity)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if tt.expectNotNil && buf == nil {
				t.Fatalf("expected non-nil buffer instance")
			}
			if !tt.expectNotNil && buf != nil {
				t.Fatalf("expected nil buffer instance")
			}
		})
	}
}

func TestNewMPMCRingBuffer_Validation(t *testing.T) {
	tests := []struct {
		name         string
		capacity     uint64
		expectedErr  error
		expectNotNil bool
	}{
		{
			name:         "Zero capacity error",
			capacity:     0,
			expectedErr:  buffer.ErrInvalidCapacity,
			expectNotNil: false,
		},
		{
			name:         "Capacity exceeds max capacity error",
			capacity:     (1 << 62) + 1,
			expectedErr:  buffer.ErrCapacityTooLarge,
			expectNotNil: false,
		},
		{
			name:         "Valid power of two capacity",
			capacity:     1024,
			expectedErr:  nil,
			expectNotNil: true,
		},
		{
			name:         "Valid non-power of two capacity (rounds up to 8)",
			capacity:     5,
			expectedErr:  nil,
			expectNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := buffer.NewMPMCRingBuffer[int](tt.capacity)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if tt.expectNotNil && buf == nil {
				t.Fatalf("expected non-nil buffer instance")
			}
			if !tt.expectNotNil && buf != nil {
				t.Fatalf("expected nil buffer instance")
			}
		})
	}
}

// ==============================================================================
// UNIT TESTS (SPSC FUNCTIONAL & BOUNDARY CONDITIONS)
// ==============================================================================

func TestSPSCRingBuffer_TryPush_TryPop(t *testing.T) {
	t.Run("FIFO Order and Boundary Errors", func(t *testing.T) {
		const cap = 4
		buf, err := buffer.NewSPSCRingBuffer[int](cap)
		if err != nil {
			t.Fatalf("unexpected error creating buffer: %v", err)
		}

		// 1. Pop from empty buffer -> ErrBufferEmpty
		if _, err := buf.TryPop(); !errors.Is(err, buffer.ErrBufferEmpty) {
			t.Fatalf("expected ErrBufferEmpty, got %v", err)
		}

		// 2. Fill buffer to capacity
		for i := 1; i <= cap; i++ {
			if err := buf.TryPush(i * 10); err != nil {
				t.Fatalf("unexpected error pushing element %d: %v", i*10, err)
			}
		}

		// 3. Push to full buffer -> ErrBufferFull
		if err := buf.TryPush(999); !errors.Is(err, buffer.ErrBufferFull) {
			t.Fatalf("expected ErrBufferFull, got %v", err)
		}

		// 4. Pop elements and verify FIFO ordering
		for i := 1; i <= cap; i++ {
			val, err := buf.TryPop()
			if err != nil {
				t.Fatalf("unexpected error popping element %d: %v", i, err)
			}
			if val != i*10 {
				t.Fatalf("expected %d, got %d", i*10, val)
			}
		}

		// 5. Verify buffer is empty again
		if _, err := buf.TryPop(); !errors.Is(err, buffer.ErrBufferEmpty) {
			t.Fatalf("expected ErrBufferEmpty after draining, got %v", err)
		}
	})

	t.Run("Ring Wrap-Around Mask Indexing", func(t *testing.T) {
		const cap = 4
		buf, _ := buffer.NewSPSCRingBuffer[int](cap)

		// Overwrite capacity multiple times to verify index masking (tail & mask)
		for cycle := 0; cycle < 100; cycle++ {
			for i := 0; i < cap; i++ {
				expected := cycle*100 + i
				if err := buf.TryPush(expected); err != nil {
					t.Fatalf("cycle %d: push failed: %v", cycle, err)
				}
			}

			for i := 0; i < cap; i++ {
				expected := cycle*100 + i
				val, err := buf.TryPop()
				if err != nil {
					t.Fatalf("cycle %d: pop failed: %v", cycle, err)
				}
				if val != expected {
					t.Fatalf("cycle %d: expected %d, got %d", cycle, expected, val)
				}
			}
		}
	})
}

func TestSPSCRingBuffer_Concurrent_SPSC(t *testing.T) {
	const cap = 1024
	const items = 100_000

	buf, err := buffer.NewSPSCRingBuffer[int](cap)
	if err != nil {
		t.Fatalf("failed to initialize buffer: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Producer Goroutine (1:1 SPSC Contract)
	go func() {
		defer wg.Done()
		for i := 1; i <= items; i++ {
			for {
				if err := buf.TryPush(i); err == nil {
					break
				}
			}
		}
	}()

	// Consumer Goroutine (1:1 SPSC Contract)
	go func() {
		defer wg.Done()
		for i := 1; i <= items; i++ {
			for {
				val, err := buf.TryPop()
				if err == nil {
					if val != i {
						t.Errorf("data corruption: expected %d, got %d", i, val)
					}
					break
				}
			}
		}
	}()

	wg.Wait()
}

// ==============================================================================
// BENCHMARKS (CONSTRUCTOR & HOT PATH ZERO-ALLOCATION)
// ==============================================================================

func BenchmarkNewSPSCRingBuffer(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = buffer.NewSPSCRingBuffer[uint64](1024)
	}
}

func BenchmarkNewMPMCRingBuffer(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = buffer.NewMPMCRingBuffer[uint64](1024)
	}
}

func BenchmarkSPSCRingBuffer_TryPush_TryPop(b *testing.B) {
	buf, _ := buffer.NewSPSCRingBuffer[uint64](1024)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buf.TryPush(uint64(i))
		_, _ = buf.TryPop()
	}
}
