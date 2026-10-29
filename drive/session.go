// Session policy (RFC D1/D2/D11): authenticated, resumable sessions own
// monotonically increasing request IDs, a bounded outcome ledger, and a
// connection generation. A session survives transport loss for
// sessionRecoveryTTL of monotonic wall time; its operations become terminal
// at the next safe boundary — they never resume execution.
package drive

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Session policy constants (the finite default limits of the RFC table).
const (
	// sessionRecoveryTTL is how long a detached session's ledger remains
	// recoverable (monotonic wall time).
	sessionRecoveryTTL = 10 * time.Minute
	// maxSessions is the live session bound; admission is refused before
	// allocating session resources beyond it.
	maxSessions = 32
	// resumeCredentialBytes is the crypto/rand size of a resume credential.
	resumeCredentialBytes = 32
	// instanceIDBytes is the crypto/rand size of an instance identity.
	instanceIDBytes = 16
)

// SessionState is the session lifecycle state (ATTACHED / DETACHED /
// EXPIRED).
type SessionState string

const (
	SessionAttached SessionState = "attached"
	SessionDetached SessionState = "detached"
	SessionExpired  SessionState = "expired"
)

// Instance is one app launch's identity. Every session it issues is bound
// to it; an instance mismatch is never a resume.
type Instance struct {
	ID string
}

// SessionRecoveryTTL exposes the recovery TTL for hello's limits document.
func SessionRecoveryTTL() time.Duration { return sessionRecoveryTTL }

// MaxSessions exposes the live-session bound for hello's limits document.
func MaxSessions() int { return maxSessions }

// NewInstance creates the random instance identity for one app launch.
func NewInstance() (Instance, error) {
	raw := make([]byte, instanceIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return Instance{}, fmt.Errorf("drive: instance id: %w", err)
	}
	return Instance{ID: hex.EncodeToString(raw)}, nil
}

// Session is one resumable automation session. The mutex guards lifecycle
// fields; the ledger is internally synchronized.
type Session struct {
	ID string
	// InstanceID this session belongs to.
	InstanceID string
	// credentialHash is sha256(resume credential) — the credential itself
	// is returned once at create and never stored.
	credentialHash []byte
	// grants are immutable for the session's life; resume MUST NOT widen.
	Grants Grants

	mu         sync.Mutex
	state      SessionState
	generation uint64 // connection generation; resume fences the old one
	detachedAt time.Time
	ledger     *Ledger
}

// Generation reports the current connection generation.
func (s *Session) Generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation
}

// State reports the lifecycle state.
func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Ledger returns the session's outcome ledger.
func (s *Session) Ledger() *Ledger { return s.ledger }

// Attach validates and installs a new connection generation for a resume:
// it fences the old generation (which cancels the replaced generation's
// outstanding operations) and returns the new generation number. A session
// that is expired or attached to a live newer generation is refused.
func (s *Session) Attach(now time.Time) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case SessionExpired:
		return 0, fmt.Errorf("session expired")
	case SessionAttached:
		// A resume of an already-attached session fences the old
		// generation; the old connection's delayed EOF must not cancel
		// new-generation work.
		s.state = SessionDetached
	}
	s.generation++
	s.state = SessionAttached
	s.detachedAt = time.Time{}
	return s.generation, nil
}

// Detach records transport loss. Returns whether the session is expired
// (past TTL at this moment).
func (s *Session) Detach(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SessionAttached {
		s.state = SessionDetached
		s.detachedAt = now
	}
}

// DetachGeneration detaches only the connection generation that observed
// loss. A delayed EOF from a replaced connection must not detach the new
// attachment or signal cancellation for its work.
func (s *Session) DetachGeneration(generation uint64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionAttached || s.generation != generation {
		return false
	}
	s.state = SessionDetached
	s.detachedAt = now
	return true
}

// WithAttachedGeneration runs fn while the session generation is fenced
// against concurrent resume/detach. The callback must be short and must
// not call back into Session.
func (s *Session) WithAttachedGeneration(generation uint64, fn func()) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionAttached || s.generation != generation {
		return false
	}
	fn()
	return true
}

// ExpiredDue reports whether a detached session's recovery TTL has passed.
func (s *Session) ExpiredDue(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionDetached {
		return false
	}
	return now.Sub(s.detachedAt) >= sessionRecoveryTTL
}

// Expire terminally expires the session; its identity is never reusable.
func (s *Session) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = SessionExpired
}

// DetachedAt reports when the session detached (zero while attached or
// never detached). session.state exposes bounded recovery metadata from it.
func (s *Session) DetachedAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detachedAt
}

// VerifyCredential reports whether the presented resume credential matches.
// Comparison is constant-time over the hash.
func (s *Session) VerifyCredential(credential string) bool {
	raw, err := hex.DecodeString(credential)
	if err != nil || len(raw) != resumeCredentialBytes {
		return false
	}
	sum := sha256.Sum256(raw)
	return hmac.Equal(sum[:], s.credentialHash)
}

// newSession allocates a session with a fresh identity and resume
// credential. The credential is returned once, to the authenticated peer.
func newSession(inst Instance, grants Grants, rng func(int) ([]byte, error)) (*Session, string, error) {
	if rng == nil {
		rng = func(n int) ([]byte, error) {
			b := make([]byte, n)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			return b, nil
		}
	}
	idRaw, err := rng(8)
	if err != nil {
		return nil, "", fmt.Errorf("drive: session id: %w", err)
	}
	credRaw, err := rng(resumeCredentialBytes)
	if err != nil {
		return nil, "", fmt.Errorf("drive: resume credential: %w", err)
	}
	sum := sha256.Sum256(credRaw)
	s := &Session{
		ID:             hex.EncodeToString(idRaw),
		InstanceID:     inst.ID,
		credentialHash: sum[:],
		Grants:         grants,
		state:          SessionAttached,
		generation:     1,
		ledger:         NewLedger(),
	}
	return s, hex.EncodeToString(credRaw), nil
}

// SessionTable owns the live and recovering sessions of one instance.
type SessionTable struct {
	mu      sync.Mutex
	inst    Instance
	byID    map[string]*Session
	rng     func(int) ([]byte, error)
	now     func() time.Time
	expired int // cumulative expired count (diagnostics)
}

// NewSessionTable builds the table for one instance identity.
func NewSessionTable(inst Instance) *SessionTable {
	return &SessionTable{
		inst: inst,
		byID: make(map[string]*Session),
		now:  time.Now,
	}
}

// InstanceID returns the owning instance identity.
func (t *SessionTable) InstanceID() string { return t.inst.ID }

// Count reports the number of non-expired sessions.
func (t *SessionTable) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.byID)
}

// Create allocates a new session under the launcher's grant policy. It
// refuses admission when the live-session bound is exhausted (capacity is
// rejected before allocating session resources).
func (t *SessionTable) Create(grants Grants) (*Session, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.byID) >= maxSessions {
		return nil, "", fmt.Errorf("drive: session limit (%d) reached", maxSessions)
	}
	s, cred, err := newSession(t.inst, grants, t.rng)
	if err != nil {
		return nil, "", err
	}
	t.byID[s.ID] = s
	return s, cred, nil
}

// Resume authenticates a resume: the session must exist, be unexpired, and
// prove the credential. The old generation is fenced. An unknown or expired
// credential MUST NOT allocate a session with the supplied ID.
func (t *SessionTable) Resume(sessionID, credential string) (*Session, uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.byID[sessionID]
	if !ok {
		return nil, 0, fmt.Errorf("unknown session")
	}
	if !s.VerifyCredential(credential) {
		return nil, 0, fmt.Errorf("resume credential mismatch")
	}
	if s.ExpiredDue(t.now()) {
		t.expireLocked(s)
		return nil, 0, fmt.Errorf("session expired")
	}
	gen, err := s.Attach(t.now())
	if err != nil {
		return nil, 0, err
	}
	return s, gen, nil
}

// Get returns the session by ID (for owning-session authority checks).
func (t *SessionTable) Get(sessionID string) (*Session, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.byID[sessionID]
	return s, ok
}

// Revoke terminally revokes one session (admin administration).
func (t *SessionTable) Revoke(sessionID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.byID[sessionID]
	if !ok {
		return false
	}
	s.Expire()
	delete(t.byID, sessionID)
	t.expired++
	return true
}

// SessionSummary is the redacted session.list entry: lifecycle metadata
// only — no credentials exist to redact, and none are synthesized.
type SessionSummary struct {
	ID         string
	State      SessionState
	Generation uint64
	Grants     Grants
}

// List returns the redacted summaries of every live session, sorted by ID
// (stable pagination order).
func (t *SessionTable) List() []SessionSummary {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]SessionSummary, 0, len(t.byID))
	for _, s := range t.byID {
		out = append(out, SessionSummary{
			ID:         s.ID,
			State:      s.State(),
			Generation: s.Generation(),
			Grants:     s.Grants,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Sweep expires detached sessions past the recovery TTL. Returns the
// number expired.
func (t *SessionTable) Sweep() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	n := 0
	for _, s := range t.byID {
		if s.ExpiredDue(now) {
			t.expireLocked(s)
			n++
		}
	}
	return n
}

// expireLocked removes an expired session; mu must be held.
func (t *SessionTable) expireLocked(s *Session) {
	s.Expire()
	delete(t.byID, s.ID)
	t.expired++
}
