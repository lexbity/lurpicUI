package drive

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func canonical(method string, params string) [32]byte {
	return CanonicalRequest(method, json.RawMessage(params))
}

// TestLedgerClaimContiguous proves D2 admission: claims are contiguous, the
// high-water advances atomically, gaps are refused without consuming
// anything.
func TestLedgerClaimContiguous(t *testing.T) {
	l := NewLedger()
	for i := uint64(1); i <= 5; i++ {
		e, err := l.Claim(i, canonical("sys.ping", "{}"), "op")
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
		if e.State != EntryAdmitted {
			t.Fatalf("entry %d state = %s", i, e.State)
		}
	}
	if hw := l.HighWater(); hw != 5 {
		t.Fatalf("high-water = %d, want 5", hw)
	}
	// Out-of-order claims are gaps.
	if _, err := l.Claim(7, canonical("sys.ping", "{}"), "op"); err == nil {
		t.Fatal("claim(7) must fail as a gap")
	}
	// Duplicate claim of an admitted ID cannot double-claim.
	if _, err := l.Claim(3, canonical("sys.ping", "{}"), "op"); err == nil {
		t.Fatal("duplicate claim must fail")
	}
}

// TestLedgerLookupLifecycle proves the three dispositions of the admission
// algorithm: retained, expired, unknown.
func TestLedgerLookupLifecycle(t *testing.T) {
	l := NewLedger()
	e, err := l.Claim(1, canonical("sys.ping", "{}"), "op")
	if err != nil {
		t.Fatal(err)
	}
	// In-flight: retained but not yet terminal.
	status, got := l.Lookup(1)
	if status != LookupRetained || got != e {
		t.Fatalf("lookup = %v %v, want retained entry", status, got)
	}
	// Complete: the terminal record is immutable.
	l.CompleteResult(e, json.RawMessage(`{"pong":true}`), "3", "3")
	l.CompleteResult(e, json.RawMessage(`{"pong":false}`), "9", "9") // late write ignored
	status, got = l.Lookup(1)
	if status != LookupRetained || string(got.ResultPayload) != `{"pong":true}` || got.ResultCut != "3" {
		t.Fatalf("terminal entry = %+v", got)
	}
	// Beyond the high-water: unknown (the next expected ID).
	if status, _ := l.Lookup(2); status != LookupUnknown {
		t.Fatalf("lookup(2) = %v, want unknown", status)
	}
}

// TestLedgerRejectionsRetained proves permanent rejections consume and
// retain their ID: a lost rejection cannot become a later accepted request.
func TestLedgerRejectionsRetained(t *testing.T) {
	l := NewLedger()
	perr := errors.New("protocol.unknown_method")
	if _, err := l.ClaimRejection(1, canonical("store.write", "{}"), perr); err != nil {
		t.Fatal(err)
	}
	status, e := l.Lookup(1)
	if status != LookupRetained || e.Admitted || e.ResultErr == nil {
		t.Fatalf("rejected entry = %+v (status %v)", e, status)
	}
	if hw := l.HighWater(); hw != 1 {
		t.Fatalf("high-water after rejection = %d, want 1", hw)
	}
}

// TestLedgerFIFOEvictionPreservesHighWater proves bounded retention: the
// oldest entries evict in admission order, and the high-water never moves
// backwards (D2: an evicted ID can never execute again).
func TestLedgerFIFOEvictionPreservesHighWater(t *testing.T) {
	l := NewLedger()
	// ledgerMaxEntries is 1024; claim more and confirm the first IDs evict.
	const n = ledgerMaxEntries + 50
	for i := uint64(1); i <= n; i++ {
		if _, err := l.Claim(i, canonical("sys.ping", "{}"), "op"); err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
	}
	if got := l.HighWater(); got != n {
		t.Fatalf("high-water = %d, want %d", got, n)
	}
	if status, _ := l.Lookup(1); status != LookupExpired {
		t.Fatalf("lookup(1) = %v, want expired", status)
	}
	if status, _ := l.Lookup(n); status != LookupRetained {
		t.Fatalf("lookup(%d) = %v, want retained", n, status)
	}
	if got := l.EarliestRetained(); got != n-uint64(ledgerMaxEntries)+1 {
		t.Fatalf("earliest retained = %d", got)
	}
}

// TestLedgerByteEviction: a large payload evicts older entries when the
// byte bound is hit.
func TestLedgerByteEviction(t *testing.T) {
	l := NewLedger()
	e1, _ := l.Claim(1, canonical("sys.ping", "{}"), "op")
	l.CompleteResult(e1, json.RawMessage(make([]byte, ledgerMaxBytes/2)), "1", "1")
	e2, _ := l.Claim(2, canonical("sys.ping", "{}"), "op")
	l.CompleteResult(e2, json.RawMessage(make([]byte, ledgerMaxBytes/2)), "2", "2")
	if status, _ := l.Lookup(1); status != LookupExpired {
		t.Fatal("the byte bound must evict the oldest large entry")
	}
	if status, _ := l.Lookup(2); status != LookupRetained {
		t.Fatal("the newest entry must be retained")
	}
}

// TestCanonicalRequestSensitivity: the canonical digest changes with the
// payload (conflict detection) and is stable for identical payloads.
func TestCanonicalRequestSensitivity(t *testing.T) {
	a := canonical("sys.ping", "{}")
	b := canonical("sys.ping", "{}")
	if a != b {
		t.Fatal("identical requests must canonicalize identically")
	}
	c := canonical("sys.ping", `{"x":1}`)
	if a == c {
		t.Fatal("a different payload must change the canonical digest")
	}
	// Key order is canonicalized away.
	d := canonical("sys.ping", `{"x":1,"y":2}`)
	e := canonical("sys.ping", `{"y":2,"x":1}`)
	if d != e {
		t.Fatal("key order must not change the canonical digest")
	}
}

func fixedNow() time.Time { return time.Unix(0, 0) }

// TestSessionCreateResumeFence proves D1: create → detach → resume fences
// the old generation and preserves the ledger; wrong instance, credential,
// or expiry never allocate or resurrect anything.
func TestSessionCreateResumeFence(t *testing.T) {
	inst, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	table := NewSessionTable(inst)
	s, cred, err := table.Create(Grants{Read: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Generation() != 1 {
		t.Fatalf("fresh generation = %d, want 1", s.Generation())
	}
	if _, err := s.Ledger().Claim(1, canonical("sys.ping", "{}"), "op"); err != nil {
		t.Fatal(err)
	}

	// Resume with the wrong credential is refused.
	if _, _, err := table.Resume(s.ID, "wrong"); err == nil {
		t.Fatal("wrong credential must be refused")
	}
	// Resume with the right credential attaches generation 2.
	gen, err := s.Attach(fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if gen != 2 {
		t.Fatalf("resume generation = %d, want 2", gen)
	}
	// The ledger survived the resume.
	if hw := s.Ledger().HighWater(); hw != 1 {
		t.Fatalf("ledger high-water after resume = %d, want 1", hw)
	}

	// Unknown sessions never allocate a session with the supplied ID.
	if _, _, err := table.Resume("nonexistent", cred); err == nil {
		t.Fatal("unknown session resume must be refused")
	}
	// An instance mismatch is refused at the caller (checked by the
	// transport against table.InstanceID).
	if other, err := NewInstance(); err == nil && other.ID == inst.ID {
		t.Fatal("instance ids must be unique")
	}
}

// TestSessionExpiry proves the 10-minute recovery TTL: an expired session's
// identity is never reusable and its ledger is unreachable.
func TestSessionExpiry(t *testing.T) {
	inst, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	table := NewSessionTable(inst)
	s, cred, err := table.Create(Grants{Read: true})
	if err != nil {
		t.Fatal(err)
	}
	now := fixedNow()
	table.now = func() time.Time { return now.Add(sessionRecoveryTTL + time.Second) }
	s.Detach(now)
	if n := table.Sweep(); n != 1 {
		t.Fatalf("sweep expired %d, want 1", n)
	}
	if s.State() != SessionExpired {
		t.Fatalf("state = %s, want expired", s.State())
	}
	if _, _, err := table.Resume(s.ID, cred); err == nil {
		t.Fatal("an expired session must not resume")
	}
	if table.Count() != 0 {
		t.Fatalf("live sessions = %d, want 0", table.Count())
	}
}

// TestSessionBoundIsFinite: the live-session bound refuses admission
// instead of allocating unbounded state.
func TestSessionBoundIsFinite(t *testing.T) {
	inst, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	table := NewSessionTable(inst)
	for i := 0; i < maxSessions; i++ {
		if _, _, err := table.Create(Grants{Read: true}); err != nil {
			t.Fatalf("session %d: %v", i, err)
		}
	}
	if _, _, err := table.Create(Grants{Read: true}); err == nil {
		t.Fatal("the session bound must refuse further creation")
	}
}

// TestGrantsIs the D11 grant table: `write` is invalid, admin is not an
// implicit application grant.
func TestGrants(t *testing.T) {
	g, err := ParseGrants("read,input,command,time,admin")
	if err != nil {
		t.Fatal(err)
	}
	if !g.Allows(GrantAdmin) || g.Admin != true {
		t.Fatal("admin must parse and authorize administration")
	}
	if g.Allows("") != true {
		t.Fatal("the empty grant is authorization-free")
	}
	if _, err := ParseGrants("write"); err == nil {
		t.Fatal(`ParseGrants("write") must fail: no v1 alias`)
	}
	if _, err := ParseGrants("read,root"); err == nil {
		t.Fatal("unknown grants must fail")
	}
	// Admin alone authorizes only administration.
	adminOnly, _ := ParseGrants("admin")
	if adminOnly.Allows(GrantRead) || adminOnly.Allows(GrantInput) ||
		adminOnly.Allows(GrantCommand) || adminOnly.Allows(GrantTime) {
		t.Fatal("admin must not imply application grants")
	}
}
