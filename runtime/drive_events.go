package runtime

import (
	"encoding/json"

	"codeburg.org/lexbit/lurpicui/diagnostics"
)

// Event publishing (LDP P7 / QP-13): the runtime sink emits events at the
// cut flush — state.changed (named stores, version-diffed per cut so events
// are cut-exact and never mid-frame), tree.changed (the frame's dirty-set
// digest), frame.completed (epoch + stats subset), own.preempted
// (preemption). All publishing is gated on a registered consumer (the drive
// server fans out into per-connection subscription tables); with no consumer
// the flush is one nil check (QP-13/NFR-4).
type driveEventConsumer func(class, storeName, testID string, params json.RawMessage)

// SetEventConsumer registers the drive server's fan-out (declared
// consumer-side by drive/server; the runtime opts in structurally).
func (s *driveRuntimeSink) SetEventConsumer(consumer driveEventConsumer) {
	s.mu.Lock()
	s.consumer = consumer
	s.mu.Unlock()
}

// publishEvent routes one event to the consumer if registered.
func (s *driveRuntimeSink) publishEvent(class, storeName, testID string, params json.RawMessage) {
	s.mu.Lock()
	consumer := s.consumer
	s.mu.Unlock()
	if consumer != nil {
		consumer(class, storeName, testID, params)
	}
}

// flushEvents publishes the cut's events: called once per frame at the end
// of runFrame, after submit (the cut boundary, QP-6). Version-diffing the
// named stores means a store mutated mid-frame yields exactly one
// state.changed at the frame's cut — never per-mutation spam.
func (rt *Runtime) flushEvents(stats diagnostics.FrameStats) {
	if rt.driveSink == nil {
		return
	}
	s := rt.driveSink
	s.mu.Lock()
	consumer := s.consumer
	s.mu.Unlock()
	if consumer == nil {
		return
	}

	// frame.completed (epoch + stats subset).
	if payload, err := json.Marshal(map[string]any{
		"epoch":   rt.frameNumber,
		"batches": stats.RenderBatchCount,
		"dirty":   stats.DirtyFacets,
	}); err == nil {
		consumer("frame.completed", "", "", payload)
	}

	// tree.changed: the frame's dirty-set digest (same data the runtime
	// re-lays-out).
	if stats.DirtyFacets > 0 {
		if payload, err := json.Marshal(map[string]any{
			"epoch":  rt.frameNumber,
			"digest": rt.FramePacketHash(),
			"facets": stats.DirtyFacets,
		}); err == nil {
			consumer("tree.changed", "", "", payload)
		}
	}

	// state.changed: version-diff the named stores against the last flushed
	// versions. Entries registered mid-run appear on their first flush.
	for _, nv := range rt.namedStores.Snapshot() {
		s.lastMu.Lock()
		last, seen := s.lastVersions[nv.Name]
		s.lastMu.Unlock()
		if seen && last == nv.Version {
			continue
		}
		s.lastMu.Lock()
		s.lastVersions[nv.Name] = nv.Version
		s.lastMu.Unlock()
		payload, err := json.Marshal(map[string]any{
			"store":   nv.Name,
			"epoch":   rt.frameNumber,
			"version": nv.Version,
		})
		if err != nil {
			continue
		}
		consumer("state.changed", nv.Name, "", payload)
	}
}
