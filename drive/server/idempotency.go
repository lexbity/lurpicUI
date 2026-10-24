package server

import "codeburg.org/lexbit/lurpicui/drive/proto"

// responseCache is the per-connection idempotency window (QP-3): a bounded
// map of request id → response with FIFO eviction (retried ids are not
// re-inserted, so ring eviction is exact). A retried id returns the cached
// response and MUST NOT re-execute a mutating method.
//
// Owned by the connection's single reader goroutine; no locking needed.
type responseCache struct {
	entries map[uint64]proto.Response
	ring    []uint64 // circular buffer of live ids in arrival order
	start   int      // oldest live id (eviction point)
	head    int      // next write slot
	size    int
}

func newResponseCache(capacity int) *responseCache {
	if capacity <= 0 {
		panic("server: responseCache capacity must be positive")
	}
	return &responseCache{
		entries: make(map[uint64]proto.Response, capacity),
		ring:    make([]uint64, capacity),
	}
}

// get returns the cached response for id, if present.
func (c *responseCache) get(id uint64) (proto.Response, bool) {
	res, ok := c.entries[id]
	return res, ok
}

// put stores the response for id, evicting the oldest id once full.
func (c *responseCache) put(id uint64, res proto.Response) {
	if _, ok := c.entries[id]; ok {
		c.entries[id] = res
		return
	}
	if c.size == len(c.ring) {
		delete(c.entries, c.ring[c.start])
	} else {
		c.size++
	}
	c.ring[c.head] = id
	c.entries[id] = res
	c.head = (c.head + 1) % len(c.ring)
	if c.size == len(c.ring) {
		c.start = c.head
	}
}

// len reports the number of live entries.
func (c *responseCache) len() int {
	return c.size
}
