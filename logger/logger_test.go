package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/DenysSkobalo/ringo/logger"
)

// TestNewLogger_Configurations verifies that logger creation correctly applies
// format choices (JSON vs Text), log level filtering, and source metadata fields.
func TestNewLogger_Configurations(t *testing.T) {
	t.Run("JSON Handler with AddSource", func(t *testing.T) {
		var buf bytes.Buffer
		l := logger.NewLogger(&buf, logger.WithJSON(true), logger.WithAddSource(true), logger.WithLevel(slog.LevelInfo))

		l.Info("test message", "key", "value")

		out := buf.String()
		if !strings.Contains(out, `"msg":"test message"`) || !strings.Contains(out, `"key":"value"`) {
			t.Fatalf("unexpected log output format: %s", out)
		}

		// Verify that the 'source' attribute is correctly emitted in JSON mode
		var data map[string]any
		if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
			t.Fatalf("failed to parse JSON log output: %v", err)
		}
		if _, exists := data["source"]; !exists {
			t.Fatal("expected 'source' field in JSON output, got none")
		}
	})

	t.Run("Text Handler formatting", func(t *testing.T) {
		var buf bytes.Buffer
		l := logger.NewLogger(&buf, logger.WithJSON(false), logger.WithLevel(slog.LevelWarn))

		// Ensures messages below the configured threshold (WARN) are filtered out
		l.Info("should be ignored")
		if buf.Len() > 0 {
			t.Fatalf("expected empty buffer for INFO level, got: %s", buf.String())
		}

		l.Warn("warning message")
		if !strings.Contains(buf.String(), "level=WARN msg=\"warning message\"") {
			t.Fatalf("unexpected text log output: %s", buf.String())
		}
	})
}

// TestNewLogger_DynamicLevelVar verifies zero-allocation runtime log level mutations
// using atomic *slog.LevelVar pointers.
func TestNewLogger_DynamicLevelVar(t *testing.T) {
	var buf bytes.Buffer
	lvlVar := new(slog.LevelVar)
	lvlVar.Set(slog.LevelInfo)

	l := logger.NewLogger(&buf, logger.WithLevel(lvlVar), logger.WithJSON(true))

	// DEBUG should be ignored initially
	l.Debug("debug trace 1")
	if buf.Len() > 0 {
		t.Fatalf("expected no output for DEBUG when level is INFO, got: %s", buf.String())
	}

	// Dynamically update level to DEBUG at runtime (atomic operation)
	lvlVar.Set(slog.LevelDebug)

	l.Debug("debug trace 2")
	if !strings.Contains(buf.String(), `"msg":"debug trace 2"`) {
		t.Fatalf("expected DEBUG log after level switch, got: %s", buf.String())
	}
}
