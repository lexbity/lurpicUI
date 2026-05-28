package testkit

import "strings"

// CapturingLogger records log messages for test assertions.
type CapturingLogger struct {
	DebugMessages []string
	InfoMessages  []string
	WarnMessages  []string
	ErrorMessages []string
}

func (l *CapturingLogger) Debug(msg string, args ...any) { l.DebugMessages = append(l.DebugMessages, msg) }
func (l *CapturingLogger) Info(msg string, args ...any)  { l.InfoMessages = append(l.InfoMessages, msg) }
func (l *CapturingLogger) Warn(msg string, args ...any)  { l.WarnMessages = append(l.WarnMessages, msg) }
func (l *CapturingLogger) Error(msg string, args ...any) { l.ErrorMessages = append(l.ErrorMessages, msg) }

// WarnContains reports whether any warning message contains the substring s.
func (l *CapturingLogger) WarnContains(s string) bool {
	for _, msg := range l.WarnMessages {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
