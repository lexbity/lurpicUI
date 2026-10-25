// The outcome ledger (RFC D2): at-most-once protection that outlives
// individual results. Every session request has a positive uint64 sequence;
// admission is contiguous; the ledger claims the ID before scheduling work.
// Retried retained IDs return the original receipt/outcome; a changed
// canonical payload is a conflict; an ID at or below the high-water mark
// with no retained result is explicitly expired — it can never execute
// again.
package drive

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
)

// Ledger retention bounds (the per-session defaults of the RFC table).
const (
	ledgerMaxEntries = 1024
	ledgerMaxBytes   = 4 << 20 // 4 MiB
)

// EntryState is a ledger entry's lifecycle.
type EntryState string

const (
	EntryAdmitted EntryState = "admitted" // claimed, work not yet terminal
	EntryTerminal EntryState = "terminal" // immutable outcome retained
)

// Entry is one claimed request identity. Terminal payloads are bounded;
// rejections are retained too, so a lost rejection never becomes a later
// accepted request with the same ID.
type Entry struct {
	ID uint64
	// OpID is the stable operation identity.
	OpID string
	// Canonical digest of the validated method+params (sha256).
	Canonical [sha256.Size]byte
	State     EntryState
	// Admitted records whether the request passed admission (false for
	// permanent rejections, which still consume their ID).
	Admitted bool
	// ResultPayload is the retained terminal result (may be nil for
	// rejections); ResultErr is the retained typed error.
	ResultPayload json.RawMessage
	ResultErr     error
	ResultCut     string
	ResultFrame   string
}

// CanonicalRequest computes the canonical digest input: the validated
// method name and params re-encoded deterministically. Unknown parameters
// cannot disappear during canonicalization because the params object was
// strictly decoded first and is hashed verbatim in sorted-key canonical
// JSON form.
func CanonicalRequest(method string, params json.RawMessage) [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write(canonicalJSON(params))
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// canonicalJSON re-encodes a strictly-decoded JSON object with sorted
// object keys. The input has already passed the strict decoder, so it is
// valid UTF-8 JSON of bounded depth; map ordering makes the output
// deterministic.
func canonicalJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("null")
	}
	var v any
	dec := json.NewDecoder(bytesReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		// Unreachable for strictly decoded input; hash the raw bytes
		// rather than panicking.
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// Lookup reports a retried ID's disposition.
type Lookup int

const (
	// LookupUnknown means the ID was never admitted (it is the next
	// expected ID or a future gap).
	LookupUnknown Lookup = iota
	// LookupRetained means a retained entry exists for the ID.
	LookupRetained
	// LookupExpired means the ID is at or below the high-water mark but no
	// longer retained — it returns request.expired, never executes again.
	LookupExpired
)

// NewLedger allocates an empty ledger.
func NewLedger() *Ledger { return &Ledger{} }

// Ledger is one session's bounded outcome ledger. Admission happens on the
// runtime thread (or under the table lock in tests); Claim and lookup are
// internally synchronized.
type Ledger struct {
	mu        sync.Mutex
	highWater uint64
	// entries holds retained entries in admission order (FIFO terminal
	// retention; this is explicit eviction order, not mislabeled LRU).
	entries []*Entry
	bytes   int
}

// HighWater returns the admitted high-water mark.
func (l *Ledger) HighWater() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.highWater
}

// Lookup classifies a request ID for the admission algorithm.
func (l *Ledger) Lookup(id uint64) (Lookup, *Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if id > l.highWater {
		return LookupUnknown, nil
	}
	for _, e := range l.entries {
		if e.ID == id {
			return LookupRetained, e
		}
	}
	return LookupExpired, nil
}

// Claim admits the next contiguous ID: it requires id == highWater+1,
// claims the entry, and advances the high-water atomically. A gap is an
// admission bug and an error.
func (l *Ledger) Claim(id uint64, canonical [sha256.Size]byte, opID string) (*Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if id != l.highWater+1 {
		return nil, fmt.Errorf("drive: request.gap: admitted %d, requested %d", l.highWater, id)
	}
	e := &Entry{ID: id, OpID: opID, Canonical: canonical, State: EntryAdmitted, Admitted: true}
	l.highWater = id
	l.entries = append(l.entries, e)
	l.bytes += ledgerEntryOverhead
	l.evictLocked()
	return e, nil
}

// ClaimRejection retains a permanent rejection: the ID is consumed and the
// rejection recorded so the lost rejection cannot become an accepted
// request under the same ID.
func (l *Ledger) ClaimRejection(id uint64, canonical [sha256.Size]byte, perr error) (*Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if id != l.highWater+1 {
		return nil, fmt.Errorf("drive: request.gap: admitted %d, requested %d", l.highWater, id)
	}
	e := &Entry{ID: id, Canonical: canonical, State: EntryTerminal, Admitted: false, ResultErr: perr}
	l.highWater = id
	l.entries = append(l.entries, e)
	l.bytes += ledgerEntryOverhead
	l.evictLocked()
	return e, nil
}

// CompleteResult records an entry's immutable successful outcome. Late
// calls are ignored: terminal records never change.
func (l *Ledger) CompleteResult(e *Entry, payload json.RawMessage, cut, frame string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.State == EntryTerminal || e != l.findLocked(e.ID) {
		return
	}
	e.State = EntryTerminal
	e.ResultPayload = payload
	e.ResultErr = nil
	e.ResultCut = cut
	e.ResultFrame = frame
	l.bytes += len(payload)
	l.evictLocked()
}

// CompleteError records an entry's immutable failed outcome.
func (l *Ledger) CompleteError(e *Entry, perr error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.State == EntryTerminal || e != l.findLocked(e.ID) {
		return
	}
	e.State = EntryTerminal
	e.ResultErr = perr
	l.evictLocked()
}

// findLocked resolves the live entry for an ID; mu must be held. An evicted
// pointer can no longer be completed.
func (l *Ledger) findLocked(id uint64) *Entry {
	for _, e := range l.entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// evictLocked applies the FIFO retention bounds; mu must be held. The
// admitted high-water is preserved across eviction.
func (l *Ledger) evictLocked() {
	for len(l.entries) > 0 && (len(l.entries) > ledgerMaxEntries || l.bytes > ledgerMaxBytes) {
		old := l.entries[0]
		l.entries = l.entries[1:]
		l.bytes -= ledgerEntryOverhead + len(old.ResultPayload)
	}
	if len(l.entries) == 0 {
		l.bytes = 0
	}
}

// ledgerEntryOverhead is the accounted fixed bytes per retained entry
// (identity, digest, error text allowance).
const ledgerEntryOverhead = 256

// LedgerMaxEntries exposes the retention bound for hello's limits document.
func LedgerMaxEntries() int { return ledgerMaxEntries }

// LedgerMaxBytes exposes the retention byte bound for hello's limits
// document.
func LedgerMaxBytes() int { return ledgerMaxBytes }

// EarliestRetained reports the earliest retained ID, or 0 when none is
// retained (session.state metadata).
func (l *Ledger) EarliestRetained() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[0].ID
}
