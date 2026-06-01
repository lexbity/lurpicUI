//go:build android

package software

import platformandroid "codeburg.org/lexbit/lurpicui/platform/android"

func androidTracef(format string, args ...any) {
	platformandroid.AndroidLogInfo(format, args...)
}
