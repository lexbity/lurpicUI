package simulation

import (
	"time"

	"codeburg.org/lexbit/lurpicui/store"
)

// backgroundRefresh spawns a goroutine for periodic store updates.
// This package imports both time and store, matching the demo-pattern
// import signature, so LL011 must flag the goroutine.
func backgroundRefresh(s *store.ValueStore) {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			s.Set(1)
		}
	}()
}
