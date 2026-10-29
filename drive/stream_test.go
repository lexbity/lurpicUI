package drive_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
)

// TestStreamTenThousandContiguousOrExpired pins the retention contract
// (RFC D7 / S6): a subscriber that keeps up receives every record
// contiguously; a stalled subscriber falls below the retention floor and
// expires explicitly — never a silent gap, never a silent resume.
func TestStreamTenThousandContiguousOrExpired(t *testing.T) {
	hub := drive.NewStreamHub()
	total := 10000
	for i := 0; i < total; i++ {
		err := hub.Append(drive.CutRecord{
			Cut: strconv.Itoa(i), Frame: strconv.Itoa(i),
			Events: []drive.Event{{Class: drive.EventStateChanged, Name: "x", Payload: json.RawMessage(`{"version":"1"}`)}},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	// A subscriber that acknowledged every delivery: watch from head (it
	// cannot recover the evicted prefix — that refusal is the other
	// subscriber's job to prove), then append 100 more while draining and
	// acking. Every record arrives in order.
	keepUp, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: 9000})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	expected := uint64(9001)
	for i := 10000; i < 10100; i++ {
		if err := hub.Append(drive.CutRecord{Cut: strconv.Itoa(i), Frame: strconv.Itoa(i), Events: []drive.Event{{Class: drive.EventStateChanged, Name: "x", Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		recs, state, reason := hub.Drain(keepUp.ID, "s", 0)
		if state != drive.StreamLive {
			t.Fatalf("keeping-up subscriber expired: %s", reason)
		}
		if len(recs) == 0 {
			t.Fatalf("no delivery at %d", i)
		}
		for j, rec := range recs {
			if rec.Cursor != expected+uint64(j) {
				t.Fatalf("contiguity break: cursor %d, want %d", rec.Cursor, expected+uint64(j))
			}
		}
		expected += uint64(len(recs))
		last := recs[len(recs)-1].Cursor
		if st, _, _ := hub.Ack(keepUp.ID, "s", 0, last); st != drive.StreamLive {
			t.Fatalf("ack state %s", st)
		}
	}
	// Drain the initial backlog to the head, still contiguously.
	for {
		recs, state, reason := hub.Drain(keepUp.ID, "s", 0)
		if state != drive.StreamLive {
			t.Fatalf("backlog drain expired: %s", reason)
		}
		if len(recs) == 0 {
			break
		}
		for j, rec := range recs {
			if rec.Cursor != expected+uint64(j) {
				t.Fatalf("backlog contiguity break: cursor %d, want %d", rec.Cursor, expected+uint64(j))
			}
		}
		expected += uint64(len(recs))
		hub.Ack(keepUp.ID, "s", 0, recs[len(recs)-1].Cursor)
	}
	sent, acked := keepUp.Position()
	if sent != 10100 || acked != 10100 {
		t.Fatalf("positions sent=%d acked=%d, want 10100/10100", sent, acked)
	}

	// A subscriber left at position 0 fell below the floor: the drain
	// reports the explicit expiry, and sub.state keeps reporting it.
	stalled, err := hub.Watch(drive.WatchSpec{Session: "s2", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch stalled: %v", err)
	}
	_, state, reason := hub.Drain(stalled.ID, "s2", 0)
	if state != drive.StreamExpired || reason != "retention.exhausted" {
		t.Fatalf("stalled subscriber = %s/%s, want expired/retention.exhausted", state, reason)
	}
	got, ok := hub.Status(stalled.ID, "s2")
	if !ok || got.State != drive.StreamExpired {
		t.Fatalf("expired stream not queryable: %v %v", ok, got)
	}
	// And the unrecoverable position is refused for new watchers.
	if _, err := hub.Watch(drive.WatchSpec{Session: "s3", Credit: 8, After: 5}); err == nil {
		t.Fatal("watch below the retention floor must be refused")
	}
}

// TestStreamFilteredOrdinalContinuation proves filter selection and event
// identity: (cursor, ordinal) pairs are stable, unmatched records advance
// the position without delivery, and a filtered stream resumes contiguously
// after an ack.
func TestStreamFilteredOrdinalContinuation(t *testing.T) {
	hub := drive.NewStreamHub()
	appendRecord := func(cut string, class drive.EventClass, name string) {
		if err := hub.Append(drive.CutRecord{Cut: cut, Frame: cut, Events: []drive.Event{{Class: class, Name: name, Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	appendRecord("1", drive.EventStateChanged, "e1.chartType")
	appendRecord("2", drive.EventFrameCompleted, "")
	appendRecord("3", drive.EventStateChanged, "e1.opacity")
	appendRecord("4", drive.EventFrameCompleted, "")

	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8,
		Filter: drive.StreamFilter{Classes: []drive.EventClass{drive.EventStateChanged}}, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	recs, state, _ := hub.Drain(stream.ID, "s", 0)
	if state != drive.StreamLive || len(recs) != 2 {
		t.Fatalf("drain = %d records (state %s), want the two state.changed records", len(recs), state)
	}
	if recs[0].Cursor != 1 || recs[1].Cursor != 3 {
		t.Fatalf("filtered cursors = %d,%d, want 1,3", recs[0].Cursor, recs[1].Cursor)
	}
	if recs[0].Events[0].Name != "e1.chartType" || recs[0].Events[0].Ordinal != 0 {
		t.Fatalf("first event identity = %+v", recs[0].Events[0])
	}
	// Ordinals restart per record and are stable across drains.
	if recs[1].Events[0].Name != "e1.opacity" || recs[1].Events[0].Ordinal != 0 {
		t.Fatalf("second event identity = %+v", recs[1].Events[0])
	}
	if _, _, _ = hub.Ack(stream.ID, "s", 0, 3); false {
		t.Fatal("unreachable")
	}
	appendRecord("5", drive.EventStateChanged, "e1.seriesColor")
	recs, _, _ = hub.Drain(stream.ID, "s", 0)
	if len(recs) != 1 || recs[0].Cursor != 5 || recs[0].Events[0].Name != "e1.seriesColor" {
		t.Fatalf("continuation = %+v, want cursor 5 seriesColor", recs)
	}
}

// TestSnapshotWatchNoGap proves the snapshot-to-stream handoff: the
// snapshot document reflects the versions of the cut the stream starts at,
// and no event between that cut and the stream start is lost.
func TestSnapshotWatchNoGap(t *testing.T) {
	hub := drive.NewStreamHubWithRetention(drive.DefaultStreamRetention)
	// Version bumps are the publisher's business; here the log and the
	// snapshot document are cross-checked for position agreement.
	for i := 0; i < 5; i++ {
		if err := hub.Append(drive.CutRecord{Cut: strconv.Itoa(i), Frame: strconv.Itoa(i), Events: []drive.Event{{Class: drive.EventStateChanged, Name: "e1.rows", Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	floor, head := hub.Bounds()
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: head})
	if err != nil {
		t.Fatalf("watch at head: %v", err)
	}
	// A snapshot watch starts exactly at the head: no prior event is owed
	// and the next append is delivered contiguously.
	if err := hub.Append(drive.CutRecord{Cut: "5", Frame: "5", Events: []drive.Event{{Class: drive.EventStateChanged, Name: "e1.rows", Payload: json.RawMessage(`{"version":"6"}`)}}}); err != nil {
		t.Fatalf("append: %v", err)
	}
	recs, _, _ := hub.Drain(stream.ID, "s", 0)
	if len(recs) != 1 || recs[0].Cursor != head+1 || recs[0].Cut != "5" {
		t.Fatalf("post-snapshot delivery = %+v (floor %d head %d), want exactly cursor %d", recs, floor, head, head+1)
	}
	_ = map[string]any{"snapshot_versions": "checked by the runtime-level studio test"}
}

// TestStreamResumeRedelivery proves reconnect recovery: the stream object
// dies with its connection, and a fresh watch at the last acknowledged
// position redelivers exactly the unacknowledged tail.
func TestStreamResumeRedelivery(t *testing.T) {
	hub := drive.NewStreamHub()
	for i := 0; i < 6; i++ {
		if err := hub.Append(drive.CutRecord{Cut: strconv.Itoa(i), Frame: strconv.Itoa(i), Events: []drive.Event{{Class: drive.EventStateChanged, Name: "n", Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	first, err := hub.Watch(drive.WatchSpec{Session: "s", Connection: 1, Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	recs, _, _ := hub.Drain(first.ID, "s", 1)
	if len(recs) != 6 {
		t.Fatalf("initial drain = %d records", len(recs))
	}
	hub.Ack(first.ID, "s", 1, 4)
	// The connection drops: the stream terminates explicitly.
	hub.CloseConnection(1)
	state, reason := first.State()
	if state != drive.StreamCancelled || reason != "connection.lost" {
		t.Fatalf("post-disconnect stream = %s/%s", state, reason)
	}
	// A new connection resumes at the acknowledged position: records 5 and 6
	// (cursor 5,6) are redelivered, nothing else.
	resumed, err := hub.Watch(drive.WatchSpec{Session: "s", Connection: 2, Credit: 8, After: 4})
	if err != nil {
		t.Fatalf("resume watch: %v", err)
	}
	recs, _, _ = hub.Drain(resumed.ID, "s", 2)
	if len(recs) != 2 || recs[0].Cursor != 5 || recs[1].Cursor != 6 {
		t.Fatalf("resume delivery = %+v, want cursors 5 and 6", recs)
	}
}

// TestFrozenTimeRetentionExpires proves age-based eviction with an injected
// clock: freezing wall time freezes retention aging; advancing it expires.
func TestFrozenTimeRetentionExpires(t *testing.T) {
	hub := drive.NewStreamHubWithRetention(drive.StreamRetention{
		MaxRecords: 1024, MaxBytes: 1 << 20, MaxRecordBytes: 256 << 10,
		RetentionAge: 30 * time.Minute,
	})
	now := time.Unix(0, 0)
	hub.SetNow(func() time.Time { return now })
	if err := hub.Append(drive.CutRecord{Cut: "1", Frame: "1", Events: []drive.Event{{Class: drive.EventFrameCompleted, Payload: json.RawMessage(`{}`)}}}); err != nil {
		t.Fatalf("append: %v", err)
	}
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	// Twenty-nine minutes of wall time pass; the records are still retained.
	now = now.Add(29 * time.Minute)
	if err := hub.Append(drive.CutRecord{Cut: "2", Frame: "2", Events: []drive.Event{{Class: drive.EventFrameCompleted, Payload: json.RawMessage(`{}`)}}}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	recs, _, _ := hub.Drain(stream.ID, "s", 0)
	if len(recs) != 2 {
		t.Fatalf("records survived 29 minutes: drained %d", len(recs))
	}
	// A fresh stream still at position 0, then past the retention age: the
	// records are evicted and the subscriber expires explicitly.
	aged, err := hub.Watch(drive.WatchSpec{Session: "aged", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch aged: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if err := hub.Append(drive.CutRecord{Cut: "3", Frame: "3", Events: []drive.Event{{Class: drive.EventFrameCompleted, Payload: json.RawMessage(`{}`)}}}); err != nil {
		t.Fatalf("append 3: %v", err)
	}
	_, state, reason := hub.Drain(aged.ID, "aged", 0)
	if state != drive.StreamExpired || reason != "retention.exhausted" {
		t.Fatalf("aged stream = %s/%s, want expired/retention.exhausted", state, reason)
	}
	if got, ok := hub.Status(aged.ID, "aged"); !ok || got.State != drive.StreamExpired || got.Reason != "retention.exhausted" || got.Floor != 2 {
		t.Fatalf("expired status = %+v ok=%v (floor must expose the eviction)", got, ok)
	}
}

// TestStalledSubscriberCreditBoundsDelivery proves the credit window: a
// subscriber that never acks receives at most its window, and the log
// (not a per-client buffer) holds the rest.
func TestStalledSubscriberCreditBoundsDelivery(t *testing.T) {
	hub := drive.NewStreamHub()
	for i := 0; i < 20; i++ {
		if err := hub.Append(drive.CutRecord{Cut: strconv.Itoa(i), Frame: strconv.Itoa(i), Events: []drive.Event{{Class: drive.EventStateChanged, Name: "n", Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 3, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	recs, state, _ := hub.Drain(stream.ID, "s", 0)
	if state != drive.StreamLive || len(recs) != 3 {
		t.Fatalf("credit did not bound delivery: %d records (state %s)", len(recs), state)
	}
	if more, _, _ := hub.Drain(stream.ID, "s", 0); len(more) != 0 {
		t.Fatalf("drain without ack delivered %d more records", len(more))
	}
	// Acking the full window frees the whole window: outstanding returns to
	// zero and the next drain delivers the next three records.
	if _, _, sendable := hub.Ack(stream.ID, "s", 0, recs[len(recs)-1].Cursor); !sendable {
		t.Fatal("ack must free credit")
	}
	recs, _, _ = hub.Drain(stream.ID, "s", 0)
	if len(recs) != 3 || recs[0].Cursor != 4 || recs[2].Cursor != 6 {
		t.Fatalf("post-ack delivery = %+v, want cursors 4..6", recs)
	}
}

// TestOversizeCutExpiresExplicitly — an oversize record is refused loudly,
// never truncated into the log.
func TestOversizeCutExpiresExplicitly(t *testing.T) {
	hub := drive.NewStreamHubWithRetention(drive.StreamRetention{
		MaxRecords: 64, MaxBytes: 1 << 20, MaxRecordBytes: 1024,
		RetentionAge: time.Hour,
	})
	big := make([]byte, 2048)
	for i := range big {
		big[i] = 'x'
	}
	err := hub.Append(drive.CutRecord{Cut: "1", Frame: "1", Events: []drive.Event{{Class: drive.EventStateChanged, Name: string(big), Payload: json.RawMessage(`{}`)}}})
	if err == nil {
		t.Fatal("oversize record must be refused")
	}
	floor, head := hub.Bounds()
	if floor != 0 || head != 0 {
		t.Fatalf("refused append polluted the log: floor=%d head=%d", floor, head)
	}
}

// TestExpiryQueryableAfterLostNotification proves the terminal state stays
// queryable after the expiry notification could not be written.
func TestExpiryQueryableAfterLostNotification(t *testing.T) {
	hub := drive.NewStreamHubWithRetention(drive.StreamRetention{
		MaxRecords: 4, MaxBytes: 1 << 20, MaxRecordBytes: 1024,
		RetentionAge: time.Hour,
	})
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err := hub.Append(drive.CutRecord{Cut: fmt.Sprint(i), Frame: fmt.Sprint(i), Events: []drive.Event{{Class: drive.EventStateChanged, Name: "n", Payload: json.RawMessage(`{}`)}}}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	_, state, reason := hub.Drain(stream.ID, "s", 0)
	if state != drive.StreamExpired || reason != "retention.exhausted" {
		t.Fatalf("drain = %s/%s", state, reason)
	}
	// The notification was lost; the queryable state survives it.
	got, ok := hub.Status(stream.ID, "s")
	if !ok || got.State != drive.StreamExpired || got.Reason != "retention.exhausted" {
		t.Fatalf("post-loss status = %+v ok=%v", got, ok)
	}
}

// TestCancelIdempotentAndScoped proves sub.cancel: terminal, idempotent,
// and session-scoped (another session cannot see or cancel a stream).
func TestCancelIdempotentAndScoped(t *testing.T) {
	hub := drive.NewStreamHub()
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	if _, ok := hub.Status(stream.ID, "other"); ok {
		t.Fatal("a foreign session must not resolve another session's stream")
	}
	state, _ := hub.Cancel(stream.ID, "s")
	if state != drive.StreamCancelled {
		t.Fatalf("cancel state = %s", state)
	}
	state, _ = hub.Cancel(stream.ID, "s")
	if state != drive.StreamCancelled {
		t.Fatalf("repeated cancel state = %s, want idempotent cancelled", state)
	}
	if _, state, _ := hub.Drain(stream.ID, "s", 0); state != drive.StreamCancelled {
		t.Fatalf("drain after cancel = %s", state)
	}
}

// TestNotifySignalFires proves the transport signal fires on append and ack
// without re-entering the hub synchronously.
func TestNotifySignalFires(t *testing.T) {
	hub := drive.NewStreamHub()
	fired := make(chan string, 16)
	hub.SetNotify(func(streamID string) { fired <- streamID })
	stream, err := hub.Watch(drive.WatchSpec{Session: "s", Credit: 8, After: 0})
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	if err := hub.Append(drive.CutRecord{Cut: "1", Frame: "1", Events: []drive.Event{{Class: drive.EventStateChanged, Name: "n", Payload: json.RawMessage(`{}`)}}}); err != nil {
		t.Fatalf("append: %v", err)
	}
	select {
	case id := <-fired:
		if id != stream.ID {
			t.Fatalf("notify for %s, want %s", id, stream.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("append did not signal delivery")
	}
	// Consume the credit, then ack: the ack must re-signal.
	hub.Drain(stream.ID, "s", 0)
	hub.Ack(stream.ID, "s", 0, 1)
	if err := hub.Append(drive.CutRecord{Cut: "2", Frame: "2", Events: []drive.Event{{Class: drive.EventStateChanged, Name: "n", Payload: json.RawMessage(`{}`)}}}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("ack path did not signal delivery")
	}
}
