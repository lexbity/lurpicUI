//go:build android

package runtime

import platformandroid "codeburg.org/lexbit/lurpicui/platform/android"

func androidTracef(format string, args ...any) {
	platformandroid.AndroidLogInfo(format, args...)
}
