package logger

import (
	"io"
	"log/slog"
)

var LevelDebug = slog.LevelDebug

type options struct {
	level     slog.Leveler
	addSource bool
	isJSON    bool
}

type Option func(*options)

func WithLevel(level slog.Leveler) Option {
	return func(o *options) {
		o.level = level
	}
}

func WithAddSource(addSource bool) Option {
	return func(o *options) {
		o.addSource = addSource
	}
}

func WithJSON(isJSON bool) Option {
	return func(o *options) {
		o.isJSON = isJSON
	}
}

func NewLogger(w io.Writer, opts ...Option) *slog.Logger {
	optsState := &options{
		level: slog.LevelInfo,
		addSource: false,
		isJSON: true,
	}

	for _, opt := range opts {
		opt(optsState)
	}

	handlerOpts := &slog.HandlerOptions{
		Level: optsState.level,
		AddSource: optsState.addSource,
	}

	var handler slog.Handler
	if optsState.isJSON {
		handler = slog.NewJSONHandler(w, handlerOpts)
	} else {
		handler = slog.NewTextHandler(w, handlerOpts)
	}

	return slog.New(handler)
}
