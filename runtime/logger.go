package runtime

import (
	"fmt"
	"os"
)

// Logger emits runtime diagnostics.
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

// LogLevel controls StderrLogger verbosity.
type LogLevel uint8

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

// StderrLogger writes formatted logs to stderr.
type StderrLogger struct {
	Level LogLevel
}

func (l StderrLogger) Debug(msg string, args ...any) { l.log(LogDebug, msg, args...) }
func (l StderrLogger) Info(msg string, args ...any)  { l.log(LogInfo, msg, args...) }
func (l StderrLogger) Warn(msg string, args ...any)  { l.log(LogWarn, msg, args...) }
func (l StderrLogger) Error(msg string, args ...any) { l.log(LogError, msg, args...) }

func (l StderrLogger) log(level LogLevel, msg string, args ...any) {
	if level < l.Level {
		return
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintln(os.Stderr, msg, args)
}
