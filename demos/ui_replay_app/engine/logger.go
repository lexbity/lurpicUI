package engine

import (
	"fmt"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LevelDebug   LogLevel = "debug"
	LevelInfo    LogLevel = "info"
	LevelWarning LogLevel = "warning"
	LevelError   LogLevel = "error"
	LevelFatal   LogLevel = "fatal"
)

// LogEntry represents a single semantic log entry.
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Category  string
	Message   string
	Step      int
	Data      map[string]interface{}
}

// SemanticLogger captures structured, machine-readable logs.
type SemanticLogger struct {
	entries []LogEntry
	step    int
}

// NewSemanticLogger creates a new semantic logger.
func NewSemanticLogger() *SemanticLogger {
	return &SemanticLogger{
		entries: make([]LogEntry, 0),
	}
}

// SetStep sets the current step context for subsequent log entries.
func (l *SemanticLogger) SetStep(step int) {
	l.step = step
}

// Log adds a semantic log entry.
func (l *SemanticLogger) Log(level LogLevel, category, message string, data map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
		Step:      l.step,
		Data:      data,
	}
	l.entries = append(l.entries, entry)
}

// Debug logs a debug message.
func (l *SemanticLogger) Debug(category, message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.Log(LevelDebug, category, message, d)
}

// Info logs an info message.
func (l *SemanticLogger) Info(category, message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.Log(LevelInfo, category, message, d)
}

// Warning logs a warning message.
func (l *SemanticLogger) Warning(category, message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.Log(LevelWarning, category, message, d)
}

// Error logs an error message.
func (l *SemanticLogger) Error(category, message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.Log(LevelError, category, message, d)
}

// Fatal logs a fatal message.
func (l *SemanticLogger) Fatal(category, message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	l.Log(LevelFatal, category, message, d)
}

// ActionStarted logs when an action starts.
func (l *SemanticLogger) ActionStarted(actionType string, step int) {
	l.SetStep(step)
	l.Info("action", fmt.Sprintf("Started %s", actionType), map[string]interface{}{
		"action_type": actionType,
		"step":        step,
	})
}

// ActionCompleted logs when an action completes.
func (l *SemanticLogger) ActionCompleted(actionType string, step int, duration time.Duration) {
	l.SetStep(step)
	l.Info("action", fmt.Sprintf("Completed %s", actionType), map[string]interface{}{
		"action_type": actionType,
		"step":        step,
		"duration_ms": duration.Milliseconds(),
	})
}

// ActionFailed logs when an action fails.
func (l *SemanticLogger) ActionFailed(actionType string, step int, err error) {
	l.SetStep(step)
	l.Error("action", fmt.Sprintf("Failed %s: %v", actionType, err), map[string]interface{}{
		"action_type": actionType,
		"step":        step,
		"error":       err.Error(),
	})
}

// AssertionChecked logs when an assertion is checked.
func (l *SemanticLogger) AssertionChecked(assertionType string, step int, passed bool) {
	l.SetStep(step)
	level := LevelInfo
	if !passed {
		level = LevelError
	}
	l.Log(level, "assertion", fmt.Sprintf("Assertion %s: %v", assertionType, passed), map[string]interface{}{
		"assertion_type": assertionType,
		"step":           step,
		"passed":         passed,
	})
}

// StateChanged logs a state change.
func (l *SemanticLogger) StateChanged(from, to RunState) {
	l.Info("state", fmt.Sprintf("State changed: %s -> %s", from, to), map[string]interface{}{
		"from_state": from,
		"to_state":   to,
	})
}

// EventCaptured logs a captured event.
func (l *SemanticLogger) EventCaptured(eventType string, target string, data map[string]interface{}) {
	mergedData := map[string]interface{}{
		"event_type": eventType,
		"target":     target,
	}
	for k, v := range data {
		mergedData[k] = v
	}
	l.Debug("event", fmt.Sprintf("Captured %s on %s", eventType, target), mergedData)
}

// SummaryCaptured logs a summary capture.
func (l *SemanticLogger) SummaryCaptured(summaryType string, summary map[string]interface{}) {
	l.Info("summary", fmt.Sprintf("Captured %s summary", summaryType), map[string]interface{}{
		"summary_type": summaryType,
		"summary":      summary,
	})
}

// Entries returns all log entries.
func (l *SemanticLogger) Entries() []LogEntry {
	return l.entries
}

// EntriesByLevel returns log entries filtered by level.
func (l *SemanticLogger) EntriesByLevel(level LogLevel) []LogEntry {
	var result []LogEntry
	for _, e := range l.entries {
		if e.Level == level {
			result = append(result, e)
		}
	}
	return result
}

// EntriesByCategory returns log entries filtered by category.
func (l *SemanticLogger) EntriesByCategory(category string) []LogEntry {
	var result []LogEntry
	for _, e := range l.entries {
		if e.Category == category {
			result = append(result, e)
		}
	}
	return result
}

// EntriesByStep returns log entries filtered by step.
func (l *SemanticLogger) EntriesByStep(step int) []LogEntry {
	var result []LogEntry
	for _, e := range l.entries {
		if e.Step == step {
			result = append(result, e)
		}
	}
	return result
}

// Clear clears all log entries.
func (l *SemanticLogger) Clear() {
	l.entries = make([]LogEntry, 0)
	l.step = 0
}

// Count returns the total number of log entries.
func (l *SemanticLogger) Count() int {
	return len(l.entries)
}

// Format returns a formatted string representation of all entries.
func (l *SemanticLogger) Format() string {
	var result string
	for _, e := range l.entries {
		result += fmt.Sprintf("[%s] %s %s (step %d): %s\n",
			e.Timestamp.Format("15:04:05.000"),
			e.Level,
			e.Category,
			e.Step,
			e.Message)
	}
	return result
}
