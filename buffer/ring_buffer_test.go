package buffer

import (
	"errors"
	"testing"
)

func TestNewRingBuffer(t *testing.T) {
	tests := []struct {
		name         string
		reqCapacity  uint64
		expectedCap  uint64
		expectedMask uint64
		expectedErr  error
	}{
		{
			name:        "Zero capacity returns error",
			reqCapacity: 0,
			expectedErr: ErrInvalidCapacity,
		},
		{
			name:         "Power of two (8) stays 8",
			reqCapacity:  8,
			expectedCap:  8,
			expectedMask: 7,
			expectedErr:  nil,
		},
		{
			name:         "Non-power of two (5) rounds up to 8",
			reqCapacity:  5,
			expectedCap:  8,
			expectedMask: 7,
			expectedErr:  nil,
		},
		{
			name:         "Non-power of two (100) rounds up to 128",
			reqCapacity:  100,
			expectedCap:  128,
			expectedMask: 127,
			expectedErr:  nil,
		},
		{
			name:         "Minimum capacity 1",
			reqCapacity:  1,
			expectedCap:  1,
			expectedMask: 0,
			expectedErr:  nil,
		},
		{
			name:        "Capacity exceeds maximum allowed limit",
			reqCapacity: (1 << 62) + 1,
			expectedErr: ErrCapacityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := NewRingBuffer[int](tt.reqCapacity)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}

			if tt.expectedErr != nil {
				return
			}

			// Прямий доступ до приватного поля rb.slots завдяки package buffer
			if uint64(len(rb.slots)) != tt.expectedCap {
				t.Errorf("slots length mismatch: expected %d, got %d", tt.expectedCap, len(rb.slots))
			}

			// Прямий доступ до приватного поля rb.mask
			if rb.mask != tt.expectedMask {
				t.Errorf("mask mismatch: expected %d, got %d", tt.expectedMask, rb.mask)
			}

			// Перевірка початкового стану секвенсорів
			for i := uint64(0); i < tt.expectedCap; i++ {
				seq := rb.slots[i].sequence.Load()
				if seq != i {
					t.Errorf("slot [%d] sequence mismatch: expected %d, got %d", i, i, seq)
				}
			}
		})
	}
}
