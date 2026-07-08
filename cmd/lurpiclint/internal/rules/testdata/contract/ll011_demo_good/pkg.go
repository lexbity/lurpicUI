package studio

import (
	"time"

	"codeburg.org/lexbit/lurpicui/job"
)

// poller uses job.Schedule for deferred work, which is allowlisted.
func poller() {
	job.Schedule(func() {
		_ = time.Now()
	})
}
