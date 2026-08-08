package main

import (
	"testing"
)

func TestNewChannelBuf_Validation(t *testing.T) {
	_, err := NewChannelBuf[int](-1)
	if err == nil {
		t.Fatal("expected error for negative buffer size, got nil")
	}
}

func TestTrySet_BufferOverflow(t *testing.T) {
	const capacity = 2
	ch, err := NewChannelBuf[int](capacity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := TrySet(ch, 1); err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}
	if err := TrySet(ch, 2); err != nil {
		t.Fatalf("expected successful send, got: %v", err)
	}

	err = TrySet(ch, 3)
	if err != ErrBufferFull {
		t.Fatalf("expected ErrBufferFull, got: %v", err)
	}
}
