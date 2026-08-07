package main

import (
	"errors"
	"fmt"
	"os"
	"playground-golang/logger"
	"sync"
)

var (
	ErrInvalidBufferSize = errors.New("channel buffer size must be non-negative")
	ErrBufferFull        = errors.New("channel buffer is full, message dropped")
)

func NewChannelBuf[T any](size int) (chan T, error) {
	if size < 0 {
		return nil, ErrInvalidBufferSize
	}
	return make(chan T, size), nil
}

// TrySet Non-blocking send
//go:noinline
func TrySet[T any](ch chan<- T, val T) error {
	select {
	case ch <- val:
		return nil
	default: 
		return ErrBufferFull
	}
}

// TryGet Non-blocking receive
//go:noinline
func TryGet[T any](ch <- chan T) (T, bool) {
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

func main() {
	logger := logger.NewLogger(os.Stdout, logger.WithLevel(logger.LevelDebug), logger.WithAddSource(true))

	const buf = 5
	const totalProducers = 10

	ch, err := NewChannelBuf[int](buf)
	if err != nil {
		logger.Error("Failed to initialize channel", "error", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var droppedCount int

	for i:=1; i < totalProducers; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			if err := TrySet(ch, val); err != nil {
				mu.Lock()
				droppedCount++
				mu.Unlock()
				logger.Warn("Producer failed to send item", "value", val, "reason", err)
			}
		}(i)
	}

	wg.Wait()
	close(ch)

	for res := range ch {
		fmt.Println("Result:", res)
	}
}
