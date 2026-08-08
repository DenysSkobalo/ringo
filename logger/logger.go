// Package logger provides a lightweight, thread-safe, and highly configurable
// wrapper around log/slog utilizing the Functional Options pattern.
package logger

import (
	"io"
	"log/slog"
)

// LevelDebug is a convenience alias for slog.LevelDebug.
// It can be used to set the minimum log severity to Debug.
var LevelDebug = slog.LevelDebug

// options defines the internal configuration state for constructing an slog.Logger.
type options struct {
	level     slog.Leveler
	addSource bool
	isJSON    bool
}

// Option defines a function signature used to configure an options instance.
type Option func(*options)

// WithLevel returns an Option that sets the minimum log level filter.
// The provided level must implement slog.Leveler, allowing both static levels
// (e.g., slog.LevelInfo) and dynamic atomic variables (e.g., *slog.LevelVar).
// If level is nil, the option is safely ignored.
func WithLevel(level slog.Leveler) Option {
	return func(o *options) {
		if level != nil {
			o.level = level
		}
	}
}

// WithAddSource returns an Option that toggles the inclusion of source code
// location metadata (filename and line number) in log records.
func WithAddSource(addSource bool) Option {
	return func(o *options) {
		o.addSource = addSource
	}
}

// WithJSON returns an Option that selects the log output format.
// If isJSON is true, logs are formatted as JSON using slog.JSONHandler.
// Otherwise, plain text format is used via slog.TextHandler.
func WithJSON(isJSON bool) Option {
	return func(o *options) {
		o.isJSON = isJSON
	}
}

// NewLogger creates and initializes a new *slog.Logger targeting the given io.Writer.
// By default, the logger uses JSON formatting, log level INFO, and disables source location.
// These defaults can be overridden by passing one or more Option functions.
func NewLogger(w io.Writer, opts ...Option) *slog.Logger {
	optsState := &options{
		level:     slog.LevelInfo,
		addSource: false,
		isJSON:    true,
	}

	for _, opt := range opts {
		opt(optsState)
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     optsState.level,
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
