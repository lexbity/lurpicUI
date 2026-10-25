package runtime

import (
	"encoding/json"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// driveHandler executes one drive method on the runtime thread. Handlers
// run at a cut after admission; only deferred outcomes (time.advance)
// complete later.
type driveHandler func(rt *Runtime, cmd drive.Command) driveOutcome

// driveMethods is the runtime-side execution table — the trusted-seam
// surface plus every advertised v2 method. The advertised surface is the
// proto registry (one source); entries never advertised on the major-2
// wire are trusted testkit-seam methods whose native v2 slices land in
// later slices. store.write has no entry: the direct mutation surface is
// deleted, not hidden.
//
// Populated in init: a composite literal initializer would form an
// initialization cycle through the step executor back into the map.
var driveMethods map[string]driveHandler

func init() {
	driveMethods = map[string]driveHandler{
		"sys.ping":       drivePing,
		"sys.info":       driveInfo,
		"time.mode":      driveTimeMode,
		"time.advance":   driveTimeAdvance,
		"frame.run":      driveFrameRun,
		"locate.resolve": driveLocateResolve,
		"tree.dump":      driveTreeDump,
		"store.list":     driveStoreList,
		"store.read":     driveStoreRead,
		"derived.read":   driveDerivedRead,
		"facet.query":    driveFacetQuery,
		"frame.pixels":   driveFramePixels,
		"frame.packet":   driveFramePacket,
		"input.click":    driveInputClick,
		"input.press":    driveInputPress,
		"input.release":  driveInputRelease,
		"input.move":     driveInputMove,
		"input.drag":     driveInputDrag,
		"input.type":     driveInputType,
		"input.scroll":   driveInputScroll,
		"input.focus":    driveInputFocus,
		"cmd.list":       driveCmdList,
		"cmd.execute":    driveCmdExecute,
	}
}

// registrySpec resolves a method's registry entry (the admission table).
func registrySpec(method string) (proto.MethodSpec, bool) {
	for _, spec := range proto.Registry {
		if spec.Name == method {
			return spec, true
		}
	}
	return proto.MethodSpec{}, false
}

// specGrant returns the entry's required grant ("" = none).
func specGrant(spec proto.MethodSpec) string { return spec.Grant }

// drivePing implements sys.ping: cut-exact liveness. Params must be the
// empty object; the result identifies the sealed cut and last submitted
// frame the pong observes.
func drivePing(rt *Runtime, cmd drive.Command) driveOutcome {
	if len(cmd.Params) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Params, &raw); err != nil || len(raw) != 0 {
			return errorOutcome(&proto.Error{
				Code:   proto.ErrInvalidParams,
				Detail: "sys.ping takes no parameters",
			})
		}
	}
	payload, err := json.Marshal(map[string]any{
		"pong":  true,
		"cut":   rt.drivePort.Cut(),
		"frame": rt.drivePort.Frame(),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// driveInfo implements sys.info: the runtime's capability inventory and
// redacted counters.
func driveInfo(rt *Runtime, cmd drive.Command) driveOutcome {
	snap := rt.drivePort.stats.Load()
	advertised := proto.AdvertisedMethodNames()
	all := proto.MethodNames()
	payload, err := json.Marshal(map[string]any{
		"v":               proto.ProtocolMajor,
		"cut":             rt.drivePort.Cut(),
		"frame":           rt.drivePort.Frame(),
		"schema_digest":   proto.SchemaDigest(),
		"methods":         advertised,
		"trusted_methods": all,
		"named_stores":    rt.namedStores.Names(),
		"drive": map[string]any{
			"commands_admitted":       snap.CommandsAdmitted,
			"commands_drained":        snap.CommandsDrained,
			"responses_sent":          snap.ResponsesSent,
			"grant_denied":            snap.GrantDenied,
			"socket_goroutine_panics": snap.SocketGoroutinePanics,
			"handler_panics":          snap.HandlerPanics,
			"duplicate_hits":          snap.DuplicateHits,
			"request_conflicts":       snap.RequestConflicts,
			"request_expired":         snap.RequestExpired,
		},
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// drivePostFrame completes admitted virtual-time advances deferred at the
// last cut: each moves the clock, runs exactly one full frame, then
// completes its immutable terminal record and replies (the reply carries
// the new frame/cut attribution). Advances orphaned by a mode flip back to
// Wall are refused with a typed error, never silently dropped.
func (rt *Runtime) drivePostFrame() {
	if rt.drivePort == nil {
		return
	}
	p := rt.drivePort
	pending := p.pendingAdvances
	if len(pending) == 0 {
		return
	}
	p.pendingAdvances = nil
	for _, adv := range pending {
		if rt.clock.Mode() != ClockVirtual {
			p.completeClaimed(adv.entry, adv.cmd, drive.Result{Err: &proto.Error{
				Code:   proto.ErrTimeMode,
				Detail: "clock left virtual mode before the advance executed",
			}})
			continue
		}
		now := rt.clock.Advance(adv.d)
		rt.runFrame(now, true)
		payload, err := json.Marshal(map[string]string{
			"frame": p.Frame(),
			"cut":   p.Cut(),
		})
		if err != nil {
			p.completeClaimed(adv.entry, adv.cmd, drive.Result{
				Err: &proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()},
			})
			continue
		}
		p.completeClaimed(adv.entry, adv.cmd, drive.Result{Payload: payload})
	}
}

// durationSeconds is a JSON float of seconds; time advance uses it.
type durationSeconds float64

// Duration converts to a time.Duration.
func (d durationSeconds) Duration() time.Duration {
	return time.Duration(float64(d) * float64(time.Second))
}

// deferredAdvance builds time.advance's deferred outcome: the admitter
// owns the ledger entry; drivePostFrame completes it after the frame.
func deferredAdvance(d durationSeconds) driveOutcome {
	return driveOutcome{deferred: true, d: d.Duration()}
}
