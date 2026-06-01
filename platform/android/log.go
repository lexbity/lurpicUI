//go:build android

package android

import "codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"

// AndroidLogInfo writes a formatted info log to logcat.
func AndroidLogInfo(format string, args ...interface{}) {
	bridge.AndroidLogInfo(format, args...)
}
