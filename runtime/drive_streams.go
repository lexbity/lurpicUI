package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
)

// Streaming, operation cancellation, and protocol administration (LDP v2
// RFC §Registry: session.state/list/revoke, op.cancel, own.state, sub.*).
// All handlers run on the runtime thread at a cut, after admission.

// subWatchParams is the strictly validated sub.watch payload: a filter,
// exactly one of after/snapshot, and a credit window.
type subWatchParams struct {
	Filter struct {
		Classes []string `json:"classes,omitempty"`
		Names   []string `json:"names,omitempty"`
	} `json:"filter,omitempty"`
	After    *string `json:"after,omitempty"`
	Snapshot bool    `json:"snapshot,omitempty"`
	Credit   int     `json:"credit,omitempty"`
}

func driveSubWatch(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	var params subWatchParams
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if len(cmd.Params) > 0 {
		if err := dec.Decode(&params); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "sub.watch params: " + err.Error()})
		}
	}
	filter := drive.StreamFilter{}
	for _, c := range params.Filter.Classes {
		filter.Classes = append(filter.Classes, drive.EventClass(c))
	}
	filter.Names = params.Filter.Names
	after := uint64(0)
	haveAfter := params.After != nil
	if haveAfter {
		v, err := strconv.ParseUint(*params.After, 10, 64)
		if err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrStreamInvalidCursor, Detail: "after must be a decimal cursor"})
		}
		after = v
	}
	if haveAfter == params.Snapshot {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "sub.watch requires exactly one of after or snapshot"})
	}
	credit := params.Credit
	if credit == 0 {
		credit = 4
	}
	stream, err := p.hub.Watch(drive.WatchSpec{
		Session:    cmd.Session,
		Connection: cmd.Connection,
		Filter:     filter,
		After:      after,
		Credit:     credit,
	})
	if err != nil {
		code := proto.ErrInvalidParams
		var serr *drive.StreamError
		if errors.As(err, &serr) && serr.Kind == drive.StreamErrCursor {
			code = proto.ErrStreamInvalidCursor
		}
		return errorOutcome(&proto.Error{Code: code, Detail: err.Error()})
	}
	floor, head := p.hub.Bounds()
	result := map[string]any{
		"stream_id": stream.ID,
		"position":  strconv.FormatUint(after, 10),
		"credit":    stream.Credit,
		"bounds": map[string]string{
			"retained_floor": strconv.FormatUint(floor, 10),
			"retained_head":  strconv.FormatUint(head, 10),
		},
	}
	if params.Snapshot {
		result["snapshot"] = p.snapshotDocument()
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "sub.watch result encoding failed"})
	}
	return payloadOutcome(payload)
}

// snapshotDocument captures the declared exports' versions at the current
// cut: the snapshot-to-stream handoff base a subscriber reconciles against
// before consuming live events. Values are fetched through store.read.
func (p *drivePort) snapshotDocument() map[string]any {
	versions := map[string]string{}
	for _, name := range p.rt.namedStores.Names() {
		export, ok := p.rt.namedStores.Lookup(name)
		if !ok {
			continue
		}
		versions[name] = strconv.FormatUint(uint64(export.Version()), 10)
	}
	for _, exports := range p.rt.facetReadExports {
		for _, export := range exports {
			info := export.Info()
			if info.Field == "" {
				continue
			}
			versions[info.Field] = strconv.FormatUint(uint64(export.Version()), 10)
		}
	}
	doc := map[string]any{
		"cut":             p.Cut(),
		driveJSONFrameKey: p.Frame(),
		"stores":          versions,
	}
	if digest := p.rt.driveTreeDigest(); digest != "" {
		doc["tree_digest"] = digest
	}
	return doc
}

func driveSubState(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	var params struct {
		StreamID string `json:"stream_id"`
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil || params.StreamID == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "sub.state requires stream_id"})
	}
	status, ok := p.hub.Status(params.StreamID, cmd.Session)
	if !ok {
		return errorOutcome(&proto.Error{Code: proto.ErrStreamInvalidCursor, Detail: "unknown stream for this session"})
	}
	payload, err := json.Marshal(map[string]any{
		"stream_id":       status.ID,
		driveJSONStateKey: string(status.State),
		"reason":          status.Reason,
		"sent":            strconv.FormatUint(status.Sent, 10),
		"acked":           strconv.FormatUint(status.Acked, 10),
		"credit":          status.Credit,
		"retained_floor":  strconv.FormatUint(status.Floor, 10),
		"retained_head":   strconv.FormatUint(status.Head, 10),
		"filter":          status.Filter,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

func driveSubCancel(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	var params struct {
		StreamID string `json:"stream_id"`
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil || params.StreamID == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "sub.cancel requires stream_id"})
	}
	state, reason := p.hub.Cancel(params.StreamID, cmd.Session)
	if state == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrStreamInvalidCursor, Detail: "unknown stream for this session"})
	}
	payload, err := json.Marshal(map[string]any{
		"stream_id":       params.StreamID,
		driveJSONStateKey: string(state),
		"reason":          reason,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// driveOpCancel implements op.cancel: request cancellation of one of this
// session's outstanding operations. Cancellation lands at the next safe
// boundary — already-applied effects stand. Cancelling a terminal operation
// is idempotent and returns its outcome.
func driveOpCancel(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	var params struct {
		OperationID string `json:"operation_id"`
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil || params.OperationID == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "op.cancel requires operation_id"})
	}
	prefix := p.sessions.InstanceID() + "/" + cmd.Session + "/"
	if !strings.HasPrefix(params.OperationID, prefix) {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionFenced, Detail: "operation does not belong to this session"})
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(params.OperationID, prefix), 10, 64)
	if err != nil || id == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrOperationExpired, Detail: "operation identity is invalid"})
	}
	sess, ok := p.sessions.Get(cmd.Session)
	if !ok {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionExpired})
	}
	status, entry := sess.Ledger().Lookup(id)
	if status != drive.LookupRetained || entry == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrOperationExpired, Detail: "operation outcome is no longer retained"})
	}
	if entry.State != drive.EntryAdmitted {
		// Already terminal: idempotent cancellation reports the outcome.
		return opStatusPayload(params.OperationID, entry)
	}
	cancelled := p.cancelOperationBySeq(cmd.Session, id)
	if !cancelled {
		// No live runtime work carries this ID and the entry is still open —
		// it is queued behind other admissions and will surface when
		// admitted. Report the request as recorded; the terminal notification
		// will arrive.
		payload, merr := json.Marshal(map[string]any{
			driveJSONOperationIDKey: params.OperationID,
			driveJSONStateKey:       "cancel_requested",
		})
		if merr != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: merr.Error()})
		}
		return payloadOutcome(payload)
	}
	payload, merr := json.Marshal(map[string]any{
		"operation_id":    params.OperationID,
		driveJSONStateKey: "cancelling",
		"cut":             p.Cut(),
		driveJSONFrameKey: p.Frame(),
	})
	if merr != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: merr.Error()})
	}
	return payloadOutcome(payload)
}

// cancelOperationBySeq cancels every live runtime representation of one
// operation (same semantics as the disconnect fence, narrowed to one ID).
func (p *drivePort) cancelOperationBySeq(session string, id uint64) bool {
	cancelled := false
	reason := &proto.Error{Code: proto.ErrOperationCancelled, Detail: "cancelled by op.cancel"}
	for i := 0; i < len(p.frameTokens); {
		token := p.frameTokens[i]
		if token.cmd.Session == session && token.cmd.ID == id {
			p.frameTokens = append(p.frameTokens[:i], p.frameTokens[i+1:]...)
			if token.plan == nil {
				p.completeFrameToken(token, drive.Result{Err: reason, Cut: p.Cut(), Frame: p.Frame()})
			}
			cancelled = true
			continue
		}
		i++
	}
	for frame, token := range p.pendingFrameTokens {
		if token.cmd.Session == session && token.cmd.ID == id {
			delete(p.pendingFrameTokens, frame)
			if token.plan == nil {
				p.completeFrameToken(token, drive.Result{Err: reason, Frame: strconv.FormatUint(frame, 10)})
			}
			cancelled = true
		}
	}
	if p.framePlan != nil && p.framePlan.cmd.Session == session && p.framePlan.cmd.ID == id {
		p.rt.finishFramePlan(p.framePlan, reason)
		cancelled = true
	}
	for i := 0; i < len(p.pendingCommandActivations); {
		pending := p.pendingCommandActivations[i]
		if pending.cmd.Session == session && pending.cmd.ID == id {
			p.pendingCommandActivations = append(p.pendingCommandActivations[:i], p.pendingCommandActivations[i+1:]...)
			p.finishCommand(pending, drive.Result{Err: reason})
			cancelled = true
			continue
		}
		i++
	}
	for frame, pending := range p.pendingCommandFrames {
		kept := pending[:0]
		for _, command := range pending {
			if command.cmd.Session == session && command.cmd.ID == id {
				p.finishCommand(command, drive.Result{Err: reason})
				cancelled = true
				continue
			}
			kept = append(kept, command)
		}
		if len(kept) == 0 {
			delete(p.pendingCommandFrames, frame)
		} else {
			p.pendingCommandFrames[frame] = kept
		}
	}
	for opID, pending := range p.waitingCommandJobs {
		if pending.cmd.Session == session && pending.cmd.ID == id {
			p.finishCommand(pending, drive.Result{Err: reason})
			delete(p.waitingCommandJobs, opID)
			cancelled = true
		}
	}
	if p.rt.driveInputStep != nil && p.rt.driveInputStep.cmd.Session == session && p.rt.driveInputStep.cmd.ID == id {
		p.rt.preemptInputStepOwned(func(step *driveInputStep) bool {
			return step.cmd.Session == session && step.cmd.ID == id
		})
		cancelled = true
	}
	return cancelled
}

// driveSessionState implements session.state: the calling session's own
// protocol state — never a credential.
func driveSessionState(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	if len(cmd.Params) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Params, &raw); err != nil || len(raw) != 0 {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "session.state takes no parameters"})
		}
	}
	sess, ok := p.sessions.Get(cmd.Session)
	if !ok {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionExpired})
	}
	ledger := sess.Ledger()
	var activeOps []string
	instance := p.sessions.InstanceID()
	p.forEachPendingOperation(cmd.Session, func(seq uint64) {
		activeOps = append(activeOps, instance+"/"+cmd.Session+"/"+strconv.FormatUint(seq, 10))
	})
	sort.Strings(activeOps)
	detachedAt := sess.DetachedAt()
	var expiresMs any
	if !detachedAt.IsZero() {
		remaining := drive.SessionRecoveryTTL() - time.Since(detachedAt)
		if remaining < 0 {
			remaining = 0
		}
		expiresMs = remaining.Milliseconds()
	}
	payload, err := json.Marshal(map[string]any{
		"session_id":             cmd.Session,
		"instance_id":            instance,
		driveJSONStateKey:        string(sess.State()),
		"generation":             strconv.FormatUint(sess.Generation(), 10),
		"next_id":                strconv.FormatUint(ledger.HighWater()+1, 10),
		"earliest_retained":      strconv.FormatUint(ledger.EarliestRetained(), 10),
		"active_operations":      activeOps,
		"recovery_expires_in_ms": expiresMs,
		"time_owner":             p.timeOwnershipDocument(),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// forEachPendingOperation reports the request sequences of the session's
// live runtime work (admitted, not yet terminal).
func (p *drivePort) forEachPendingOperation(session string, fn func(seq uint64)) {
	seen := map[uint64]bool{}
	add := func(seq uint64) {
		if seq != 0 && !seen[seq] {
			seen[seq] = true
			fn(seq)
		}
	}
	for _, token := range p.frameTokens {
		if token.cmd.Session == session {
			add(token.cmd.ID)
		}
	}
	for _, token := range p.pendingFrameTokens {
		if token.cmd.Session == session {
			add(token.cmd.ID)
		}
	}
	if p.framePlan != nil && p.framePlan.cmd.Session == session {
		add(p.framePlan.cmd.ID)
	}
	for _, pending := range p.pendingCommandActivations {
		if pending.cmd.Session == session {
			add(pending.cmd.ID)
		}
	}
	for _, pending := range p.pendingCommandFrames {
		for _, command := range pending {
			if command.cmd.Session == session {
				add(command.cmd.ID)
			}
		}
	}
	for _, pending := range p.waitingCommandJobs {
		if pending.cmd.Session == session {
			add(pending.cmd.ID)
		}
	}
}

// timeOwnershipDocument reports the current time-lease attribution.
func (p *drivePort) timeOwnershipDocument() map[string]any {
	doc := map[string]any{"session": p.timeOwnerSession}
	if p.timeOwnerSession != "" {
		doc["generation"] = strconv.FormatUint(p.timeOwnerGeneration, 10)
		doc["lease"] = "session"
	}
	if p.bootstrapTimeUntil.IsZero() {
		return doc
	}
	remaining := time.Until(p.bootstrapTimeUntil)
	if remaining < 0 {
		remaining = 0
	}
	doc["lease"] = "bootstrap"
	doc["lease_expires_in_ms"] = remaining.Milliseconds()
	return doc
}

// driveSessionList implements session.list (admin): the redacted session
// metadata page.
func driveSessionList(rt *Runtime, cmd drive.Command) driveOutcome {
	var params struct {
		Cursor string `json:"cursor,omitempty"`
		Limit  int    `json:"limit,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if len(cmd.Params) > 0 {
		if err := dec.Decode(&params); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "session.list params: " + err.Error()})
		}
	}
	limit := params.Limit
	if limit == 0 {
		limit = 32
	}
	if limit < 1 || limit > 32 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "session.list limit must be 1..32"})
	}
	all := rt.drivePort.sessions.List()
	start := 0
	if params.Cursor != "" {
		// Sorted-ID pagination: skip entries at or before the cursor.
		for start < len(all) && all[start].ID <= params.Cursor {
			start++
		}
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]
	next := ""
	if end < len(all) {
		next = page[len(page)-1].ID
	}
	entries := make([]map[string]any, 0, len(page))
	for _, s := range page {
		grants := make([]string, 0, 5)
		for _, g := range []string{protoGrantRead, protoGrantInput, protoGrantCommand, protoGrantTime, protoGrantAdmin} {
			if s.Grants.Allows(g) {
				grants = append(grants, g)
			}
		}
		entries = append(entries, map[string]any{
			"session_id":      s.ID,
			driveJSONStateKey: string(s.State),
			"generation":      strconv.FormatUint(s.Generation, 10),
			"grants":          grants,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"sessions":    entries,
		"next_cursor": next,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

const (
	protoGrantRead    = "read"
	protoGrantInput   = "input"
	protoGrantCommand = "command"
	protoGrantTime    = "time"
	protoGrantAdmin   = "admin"
)

// sessionRevokeReasons is the closed reason enum of session.revoke.
var sessionRevokeReasons = map[string]bool{
	"operator":          true,
	"policy":            true,
	"instance.shutdown": true,
}

// driveSessionRevoke implements session.revoke (admin): terminally revoke
// one session, fence its outstanding work at this safe boundary, and cancel
// its streams. Already-revoked sessions are idempotently reported.
func driveSessionRevoke(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	var params struct {
		SessionID string `json:"session_id"`
		Reason    string `json:"reason"`
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil || params.SessionID == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "session.revoke requires session_id and reason"})
	}
	if !sessionRevokeReasons[params.Reason] {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "reason must be operator, policy, or instance.shutdown"})
	}
	if !p.sessions.Revoke(params.SessionID) {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionExpired, Detail: "session is unknown or already revoked"})
	}
	p.hub.CloseSession(params.SessionID)
	p.cancelSessionWork(params.SessionID)
	payload, err := json.Marshal(map[string]any{
		"revoked":         true,
		"session":         params.SessionID,
		"reason":          params.Reason,
		"cut":             p.Cut(),
		driveJSONFrameKey: p.Frame(),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// cancelSessionWork cancels every live operation of one session regardless
// of generation (revocation is generation-independent).
func (p *drivePort) cancelSessionWork(session string) {
	reason := &proto.Error{Code: proto.ErrOperationCancelled, Detail: "session revoked"}
	for i := 0; i < len(p.frameTokens); {
		token := p.frameTokens[i]
		if token.cmd.Session == session {
			p.frameTokens = append(p.frameTokens[:i], p.frameTokens[i+1:]...)
			if token.plan == nil {
				p.completeFrameToken(token, drive.Result{Err: reason, Cut: p.Cut(), Frame: p.Frame()})
			}
			continue
		}
		i++
	}
	for frame, token := range p.pendingFrameTokens {
		if token.cmd.Session == session {
			delete(p.pendingFrameTokens, frame)
			if token.plan == nil {
				p.completeFrameToken(token, drive.Result{Err: reason, Frame: strconv.FormatUint(frame, 10)})
			}
		}
	}
	if p.framePlan != nil && p.framePlan.cmd.Session == session {
		p.rt.finishFramePlan(p.framePlan, reason)
	}
	for i := 0; i < len(p.pendingCommandActivations); {
		pending := p.pendingCommandActivations[i]
		if pending.cmd.Session == session {
			p.pendingCommandActivations = append(p.pendingCommandActivations[:i], p.pendingCommandActivations[i+1:]...)
			p.finishCommand(pending, drive.Result{Err: reason})
			continue
		}
		i++
	}
	for frame, pending := range p.pendingCommandFrames {
		kept := pending[:0]
		for _, command := range pending {
			if command.cmd.Session == session {
				p.finishCommand(command, drive.Result{Err: reason})
				continue
			}
			kept = append(kept, command)
		}
		if len(kept) == 0 {
			delete(p.pendingCommandFrames, frame)
		} else {
			p.pendingCommandFrames[frame] = kept
		}
	}
	for opID, pending := range p.waitingCommandJobs {
		if pending.cmd.Session == session {
			p.finishCommand(pending, drive.Result{Err: reason})
			delete(p.waitingCommandJobs, opID)
		}
	}
	if p.timeOwnerSession == session {
		p.releaseTimeOwner(true)
	}
	if p.rt.driveInputStep != nil && p.rt.driveInputStep.sessionID == session {
		p.rt.preemptInputStepOwned(func(step *driveInputStep) bool { return step.sessionID == session })
	}
}

// driveOwnState implements own.state: the immutable control/ownership view
// a driver uses to know who owns input and time right now.
func driveOwnState(rt *Runtime, cmd drive.Command) driveOutcome {
	p := rt.drivePort
	if len(cmd.Params) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Params, &raw); err != nil || len(raw) != 0 {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "own.state takes no parameters"})
		}
	}
	snapshot := rt.ControlSnapshot()
	owner := "human"
	if snapshot.InputOwner == facet.ControlOwnerClient {
		owner = "client"
	}
	doc := map[string]any{
		"owner":         owner,
		"input_session": snapshot.InputSession,
		"input_generation": func() any {
			if snapshot.InputGeneration == 0 {
				return nil
			}
			return strconv.FormatUint(snapshot.InputGeneration, 10)
		}(),
		"held_gestures": snapshot.HeldGestures,
		"mode":          modeString(rt),
		"time_owner":    p.timeOwnershipDocument(),
		"cut":           p.Cut(),
		"frame":         p.Frame(),
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// Event emission -----------------------------------------------------------------

// emitSealEvents publishes the cut-sealed observation record: frame
// completion, export version deltas, and (on topology change) the tree
// digest. The hub bounds retention; an oversize record is refused loudly
// through the stats rather than silently dropped.
func (p *drivePort) emitSealEvents(frame uint64) {
	if p.hub == nil {
		return
	}
	cut := strconv.FormatUint(frame, 10)
	frameStr := strconv.FormatUint(frame, 10)
	events := make([]drive.Event, 0, 4)
	events = append(events, drive.Event{
		Class:   drive.EventFrameCompleted,
		Payload: mustJSON(map[string]any{"frame": frameStr, "ready": false}),
	})
	events = append(events, p.exportVersionEvents()...)
	if rebuilds := p.rt.projectionSystem.TopologyRebuilds; rebuilds != p.lastTopologyRebuilds {
		p.lastTopologyRebuilds = rebuilds
		if digest := p.rt.driveTreeDigest(); digest != "" {
			events = append(events, drive.Event{
				Class:   drive.EventTreeChanged,
				Payload: mustJSON(map[string]any{"root": p.rt.rootObservationName(), "tree_digest": digest}),
			})
		}
	}
	if len(events) == 0 {
		return
	}
	if err := p.hub.Append(drive.CutRecord{Cut: cut, Frame: frameStr, Events: events}); err != nil {
		p.stats.AddStreamRecordRefused()
		if p.rt.log != nil {
			p.rt.log.Error("runtime: drive cut record refused", "error", err)
		}
	}
}

// exportVersionEvents compares each declared export's version against the
// last sealed cut and reports the deltas (state.changed is a version delta,
// never a per-write replay).
func (p *drivePort) exportVersionEvents() []drive.Event {
	var events []drive.Event
	observe := func(key, name string, version uint64) {
		prev, seen := p.lastExportVersions[key]
		p.lastExportVersions[key] = version
		if seen && prev != version {
			events = append(events, drive.Event{
				Class: drive.EventStateChanged,
				Name:  name,
				Payload: mustJSON(map[string]string{
					"previous_version": strconv.FormatUint(prev, 10),
					"version":          strconv.FormatUint(version, 10),
				}),
			})
		}
	}
	for _, name := range p.rt.namedStores.Names() {
		export, ok := p.rt.namedStores.Lookup(name)
		if !ok {
			continue
		}
		observe("#"+name, name, uint64(export.Version()))
	}
	for _, exports := range p.rt.facetReadExports {
		for _, export := range exports {
			info := export.Info()
			if info.Field == "" {
				continue
			}
			observe("@"+info.Field, info.Field, uint64(export.Version()))
		}
	}
	return events
}

// emitFrameReady publishes the surface-ready acknowledgment for one frame.
func (p *drivePort) emitFrameReady(frame uint64) {
	if p.hub == nil {
		return
	}
	err := p.hub.Append(drive.CutRecord{
		Cut:   p.Cut(),
		Frame: strconv.FormatUint(frame, 10),
		Events: []drive.Event{{
			Class:   drive.EventFrameReady,
			Payload: mustJSON(map[string]any{"frame": strconv.FormatUint(frame, 10)}),
		}},
	})
	if err != nil {
		p.stats.AddStreamRecordRefused()
	}
}

// emitOperationTerminal publishes one operation's terminal state,
// supplementing reliable op.status (subscription expiry never erases
// operation outcomes).
func (p *drivePort) emitOperationTerminal(operationID, state string) {
	if p.hub == nil {
		return
	}
	err := p.hub.Append(drive.CutRecord{
		Cut:   p.Cut(),
		Frame: p.Frame(),
		Events: []drive.Event{{
			Class:   drive.EventOperationTerminal,
			Name:    operationID,
			Payload: mustJSON(map[string]any{driveJSONOperationIDKey: operationID, driveJSONStateKey: state}),
		}},
	})
	if err != nil {
		p.stats.AddStreamRecordRefused()
	}
}

// emitOwnPreempted publishes the human-preemption record. It is a retained
// event; authoritative ownership state remains queryable via own.state.
func (p *drivePort) emitOwnPreempted(reason string) {
	if p.hub == nil {
		return
	}
	err := p.hub.Append(drive.CutRecord{
		Cut:   p.Cut(),
		Frame: p.Frame(),
		Events: []drive.Event{{
			Class:   drive.EventOwnPreempted,
			Payload: mustJSON(map[string]any{"reason": reason}),
		}},
	})
	if err != nil {
		p.stats.AddStreamRecordRefused()
	}
}

// driveTreeDigest hashes the canonical identity/structure/bounds/layer
// export of the retained projection topology — not a packet hash or dirty
// count. Empty when no topology is published.
func (rt *Runtime) driveTreeDigest() string {
	if rt.projectionSystem == nil {
		return ""
	}
	snapshots := rt.projectionSystem.OutputSnapshots()
	if len(snapshots) == 0 {
		return ""
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].FacetID < snapshots[j].FacetID })
	h := sha256.New()
	writeCount := func(n uint64) { writeUint64(h, n) }
	writeCount(uint64(len(snapshots)))
	for _, s := range snapshots {
		writeCount(uint64(s.FacetID))
		writeCount(uint64(s.Bounds.Min.X))
		writeCount(uint64(s.Bounds.Min.Y))
		writeCount(uint64(s.Bounds.Max.X))
		writeCount(uint64(s.Bounds.Max.Y))
		writeCount(uint64(s.LayerID))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// rootObservationName is the tree.changed root attribution.
func (rt *Runtime) rootObservationName() string {
	if rt.root == nil || rt.root.Base() == nil {
		return ""
	}
	return "facet:" + strconv.FormatUint(uint64(rt.root.Base().ID()), 10)
}

// writeUint64 appends n in little-endian order (digest input only; the
// values are facet/layout counters whose magnitudes the window bounds).
func writeUint64(h interface{ Write([]byte) (int, error) }, n uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], n)
	_, _ = h.Write(buf[:])
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

// modeString names the current clock mode for control documents.
func modeString(rt *Runtime) string {
	if rt.clock != nil && rt.clock.Mode() == ClockVirtual {
		return "virtual"
	}
	return "wall"
}

// opStatusPayload renders an entry's terminal outcome (shared by op.status
// and op.cancel's idempotent path).
func opStatusPayload(operationID string, entry *drive.Entry) driveOutcome {
	state := "completed"
	var result json.RawMessage
	var failure *proto.Error
	if entry.ResultErr != nil {
		state = "failed"
		if !errors.As(entry.ResultErr, &failure) {
			failure = &proto.Error{Code: proto.ErrCommandFailed, Detail: "operation failed"}
		}
		if failure.Code == proto.ErrOperationCancelled {
			state = "cancelled"
		}
	} else {
		result = append(json.RawMessage(nil), entry.ResultPayload...)
	}
	frame := entry.AcceptedFrame
	if entry.ResultFrame != "" {
		frame = entry.ResultFrame
	}
	payload, err := json.Marshal(map[string]any{
		driveJSONOperationIDKey: operationID, driveJSONStateKey: state, "result": result, "error": failure,
		"accepted_cut": entry.AcceptedCut, "frame": frame,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}
