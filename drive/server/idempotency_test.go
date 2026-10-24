package server

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

func protoResponse(id uint64) proto.Response {
	return proto.Response{ID: id, OK: true, Result: json.RawMessage(`{"pong":true}`)}
}

func TestResponseCache_InsertHitEvict(t *testing.T) {
	c := newResponseCache(3)
	for id := uint64(1); id <= 3; id++ {
		c.put(id, protoResponse(id))
	}
	if c.len() != 3 {
		t.Fatalf("len = %d, want 3", c.len())
	}
	for id := uint64(1); id <= 3; id++ {
		got, ok := c.get(id)
		if !ok || got.ID != id {
			t.Fatalf("get(%d) = (%d, %v)", id, got.ID, ok)
		}
	}
	// Insert 4 → evicts 1 (oldest).
	c.put(4, protoResponse(4))
	if _, ok := c.get(1); ok {
		t.Fatal("id 1 should have been evicted")
	}
	if c.len() != 3 {
		t.Fatalf("len = %d, want 3", c.len())
	}
	for id := uint64(2); id <= 4; id++ {
		if _, ok := c.get(id); !ok {
			t.Fatalf("id %d missing after eviction", id)
		}
	}
}

func TestResponseCache_EvictsInArrivalOrder(t *testing.T) {
	c := newResponseCache(2)
	for id := uint64(10); id <= 20; id++ {
		c.put(id, protoResponse(id))
	}
	if c.len() != 2 {
		t.Fatalf("len = %d, want 2", c.len())
	}
	// Oldest two live ids are 19 and 20.
	if _, ok := c.get(19); !ok {
		t.Fatal("id 19 missing")
	}
	if _, ok := c.get(20); !ok {
		t.Fatal("id 20 missing")
	}
	for id := uint64(10); id <= 18; id++ {
		if _, ok := c.get(id); ok {
			t.Fatalf("id %d should have been evicted", id)
		}
	}
}

func TestResponseCache_ReputOverwrites(t *testing.T) {
	c := newResponseCache(2)
	c.put(1, protoResponse(1))
	c.put(1, protoResponse(1)) // same id twice: overwrite, no eviction
	if c.len() != 1 {
		t.Fatalf("len = %d, want 1", c.len())
	}
	if _, ok := c.get(1); !ok {
		t.Fatal("id 1 missing after reput")
	}
}

func TestResponseCache_CapacityOne(t *testing.T) {
	c := newResponseCache(1)
	c.put(1, protoResponse(1))
	c.put(2, protoResponse(2))
	if c.len() != 1 {
		t.Fatalf("len = %d, want 1", c.len())
	}
	if _, ok := c.get(1); ok {
		t.Fatal("id 1 should have been evicted")
	}
	if _, ok := c.get(2); !ok {
		t.Fatal("id 2 missing")
	}
}
