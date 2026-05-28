// Package log provides a minimal Logger interface for runtime diagnostics.
package log

import (
	"log/slog"
	"os"
)

// Logger emits runtime diagnostics.
// *slog.Logger satisfies this interface directly.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NopLogger drops all log messages.
type NopLogger struct{}

func (NopLogger) Debug(string, ...any) {}
func (NopLogger) Info(string, ...any)  {}
func (NopLogger) Warn(string, ...any)  {}
func (NopLogger) Error(string, ...any) {}

// NewStderrLogger returns a Logger backed by slog.TextHandler writing to stderr.
func NewStderrLogger() Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

var _ Logger = (*slog.Logger)(nil)
