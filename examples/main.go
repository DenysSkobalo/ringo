package main

import (
	"fmt"
	"os"

	"github.com/DenysSkobalo/ringo/buffer"
	"github.com/DenysSkobalo/ringo/logger"
)


func main() {
	log := logger.NewLogger(os.Stdout, logger.WithLevel(logger.LevelDebug), logger.WithAddSource(true))

	const cap = 100
	buf, err := buffer.NewRingBuffer[string](cap)
	if err != nil {
		log.Error("failed to initialize ring buffer", "err", err)
		os.Exit(1)
	}

	log.Info("ring buffer initialized successfuly", "capacity", buf.Cap())

	if err := buf.TryPush("hello"); err != nil {
		log.Warn("failed to push item to buffer", "err", err)
	}

	if val, ok := buf.TryPop(); ok {
		fmt.Println("Popped item:", val)
	} else {
		log.Debug("buffer is empty")
	}

	if err := buf.Close(); err != nil {
		log.Error("failed to close ring buffer safely", "err", err)
		os.Exit(1)
	}

}
