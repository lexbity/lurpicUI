package studio

import (
	"time"
)

// timer starts an animation goroutine.  Demo packages must not use raw
// goroutines even when no facet type is defined.
func timer() {
	go func() {
		time.Sleep(time.Second)
	}()
}
