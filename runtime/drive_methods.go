package runtime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/render"
)

// driveHandler executes one drive method on the runtime thread. Handlers
// run at a cut after admission; only deferred outcomes (time.advance)
// complete later.
type driveHandler func(rt *Runtime, cmd drive.Command) driveOutcome

// driveMethods is the runtime-side execution table — the trusted-seam
// surface plus every advertised v2 method. The advertised surface is the
// proto registry (one source); entries never advertised on the major-2
// wire are trusted testkit-seam methods whose native v2 slices have not
// landed. store.write has no entry: the direct mutation surface is
// deleted, not hidden.
//
// Populated in init: a composite literal initializer would form an
// initialization cycle through the step executor back into the map.
var driveMethods map[string]driveHandler

const driveJSONFrameKey = "frame"
const driveMethodCommandExecute = "cmd.execute"
const driveMethodInputRelease = "input.release"

func init() {
	driveMethods = map[string]driveHandler{
		"sys.ping":                drivePing,
		"sys.info":                driveInfo,
		"time.mode":               driveTimeMode,
		"time.advance":            driveTimeAdvance,
		"frame.run":               driveFrameRun,
		"locate.resolve":          driveLocateResolve,
		"tree.dump":               driveTreeDump,
		"store.list":              driveStoreList,
		"store.read":              driveStoreRead,
		"derived.read":            driveDerivedRead,
		"facet.query":             driveFacetQuery,
		"frame.pixels":            driveFramePixels,
		"frame.packet":            driveFramePacket,
		"input.click":             driveInputClick,
		"input.press":             driveInputPress,
		driveMethodInputRelease:   driveInputRelease,
		"input.move":              driveInputMove,
		"input.drag":              driveInputDrag,
		"input.type":              driveInputType,
		"input.scroll":            driveInputScroll,
		"input.focus":             driveInputFocus,
		"cmd.list":                driveCmdList,
		driveMethodCommandExecute: driveCmdExecute,
		"op.status":               driveOpStatus,
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
		"pong":            true,
		"cut":             rt.drivePort.Cut(),
		driveJSONFrameKey: rt.drivePort.Frame(),
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
		"v":                    proto.ProtocolMajor,
		"cut":                  rt.drivePort.Cut(),
		driveJSONFrameKey:      rt.drivePort.Frame(),
		"schema_digest":        proto.SchemaDigest(),
		"export_schema_digest": rt.drivePort.Observe().ExportSchemaDigest,
		"methods":              advertised,
		"trusted_methods":      all,
		"named_stores":         rt.namedStores.Names(),
		"observation_index": map[string]any{
			"active": rt.driveObservation != nil,
			"nodes": func() int {
				if rt.driveObservation == nil {
					return 0
				}
				return len(rt.driveObservation.entries)
			}(),
			"bytes": func() int {
				if rt.driveObservation == nil {
					return 0
				}
				return rt.driveObservation.bytes
			}(),
			"exhausted":     rt.driveObservation != nil && rt.driveObservation.exhausted,
			"node_capacity": rt.config.Drive.ObservationNodeCapacity,
			"byte_capacity": rt.config.Drive.ObservationByteCapacity,
		},
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

// runScheduledVirtualFrame consumes one admitted frame token through the
// same top-level entry point used by the live loop and headless harness.
func (rt *Runtime) runScheduledVirtualFrame(waitForRender bool) bool {
	p := rt.drivePort
	if p == nil || len(p.frameTokens) == 0 {
		return false
	}
	if rt.renderPipeline == nil || !rt.isSurfaceReady() {
		return false
	}
	token := p.frameTokens[0]
	if !token.frontierSet {
		token.frontier = rt.jobPool.Frontier()
		token.frontierSet = true
		p.frameTokens[0] = token
	}
	if !rt.jobPool.ReadyThrough(token.frontier) {
		return false
	}
	reserved := false
	if !waitForRender && rt.renderPipeline != nil && rt.isSurfaceReady() {
		reserved = rt.renderPipeline.TryReserveFrame()
		if !reserved {
			return false
		}
	}
	token = p.frameTokens[0]
	copy(p.frameTokens, p.frameTokens[1:])
	p.frameTokens[len(p.frameTokens)-1] = pendingFrameToken{}
	p.frameTokens = p.frameTokens[:len(p.frameTokens)-1]
	if token.duration > 0 {
		if !rt.clock.CanAdvance(token.duration) {
			p.completeFrameToken(token, drive.Result{Err: &proto.Error{Code: proto.ErrInvalidParams, Detail: "duration overflows engine time"}})
			return true
		}
	}
	now := rt.clock.Now()
	if token.duration > 0 {
		now = rt.clock.Advance(token.duration)
	}
	frame := rt.frameNumber + 1
	if token.plan != nil {
		p.currentFramePlan = token.plan
		rt.drivePreparePlanInputStep(token.plan)
	}
	if token.entry != nil {
		p.pendingFrameTokens[frame] = token
	}
	rt.drivePrepareInputFrame()
	rt.runFrame(now, waitForRender, reserved, false)
	p.currentFramePlan = nil
	if waitForRender && rt.lastRenderCompletion.Frame == frame {
		rt.driveHandleRenderCompletion(rt.lastRenderCompletion)
	}
	return true
}

func (rt *Runtime) driveHandleRenderCompletion(completion RenderCompletion) {
	if completion.Lifecycle != 0 {
		rt.handleRenderLifecycleCompletion(completion)
		return
	}
	rt.observeRenderDeviceGeneration(completion)
	var accepted bool
	completion, accepted = rt.completeFrameView(completion)
	if !accepted && completion.ResourceGeneration != 0 {
		return
	}
	if completion.FallbackFrom != "" {
		rt.notifyBackendFallback(completion.FallbackFrom, completion.FallbackReason)
	}
	if rt.bootstrapFrameAwaiting && completion.Frame == rt.frameNumber {
		rt.bootstrapFrameAwaiting = false
		if completion.Ready && completion.Err == nil && rt.drivePort != nil && rt.drivePort.timeOwnerSession == "" {
			rt.drivePort.bootstrapTimeUntil = rt.clock.WallNow().Add(10 * time.Second)
		}
	}
	if rt.drivePort == nil {
		return
	}
	p := rt.drivePort
	rt.driveCompleteCommandFrame(p, completion)
	token, ok := p.pendingFrameTokens[completion.Frame]
	if !ok {
		return
	}
	delete(p.pendingFrameTokens, completion.Frame)
	if token.plan != nil {
		rt.driveCompletePlanFrame(token.plan, completion)
		return
	}
	if token.mode == "wall" && p.timeOwnerSession == token.cmd.Session && p.timeOwnerGeneration == token.cmd.Generation {
		p.releaseTimeOwner(false)
	}
	if completion.Err != nil || !completion.Ready {
		detail := "submitted frame did not produce a ready surface acknowledgment"
		if completion.Err != nil {
			detail = completion.Err.Error()
		}
		p.completeFrameToken(token, drive.Result{Err: &proto.Error{Code: proto.ErrRenderFailed, Detail: detail}, Frame: p.Frame(), Cut: p.Cut()})
		return
	}
	payload, err := json.Marshal(map[string]string{
		driveJSONFrameKey: p.Frame(),
		"cut":             p.Cut(),
		"render_status":   "ready",
		"ready_frame":     strconv.FormatUint(completion.Frame, 10),
		"ready_backend":   completion.Backend,
		"mode":            token.mode,
	})
	if err != nil {
		p.completeFrameToken(token, drive.Result{Err: &proto.Error{Code: proto.ErrInternalPanic, Detail: "frame result encoding failed"}})
		return
	}
	p.completeFrameToken(token, drive.Result{Payload: payload, Frame: p.Frame(), Cut: p.Cut()})
}

func (rt *Runtime) handleRenderLifecycleCompletion(completion RenderCompletion) {
	if completion.HasDeviceGeneration {
		rt.observeRenderDeviceGeneration(completion)
	}
	if completion.Lifecycle != renderLifecycleSurfaceCreated {
		if completion.Err != nil && rt.log != nil {
			rt.log.Error("runtime: render lifecycle operation failed", "operation", completion.Lifecycle, "generation", completion.LifecycleGeneration, "error", completion.Err)
		}
		return
	}
	rt.lifecycleMu.Lock()
	if completion.LifecycleGeneration != rt.surfaceLifecycleGeneration {
		rt.lifecycleMu.Unlock()
		return
	}
	ready := completion.Ready && completion.Err == nil
	rt.surfaceReady = ready
	rt.lifecycleMu.Unlock()
	if completion.Err != nil && rt.log != nil {
		rt.log.Error("runtime: render surface recreation failed", "generation", completion.LifecycleGeneration, "error", completion.Err)
	}
	if ready && rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) observeRenderDeviceGeneration(completion RenderCompletion) {
	if rt == nil || !completion.HasDeviceGeneration {
		return
	}
	rt.lastDeviceGeneration = completion.DeviceGeneration
	rt.hasDeviceGeneration = true
	if rt.assetManager == nil {
		return
	}
	if ev, ok := rt.assetManager.(interface{ CheckDeviceGeneration(uint64) bool }); ok {
		if ev.CheckDeviceGeneration(completion.DeviceGeneration) && rt.log != nil {
			rt.log.Info("runtime: device generation changed; GPU LODs invalidated", "generation", completion.DeviceGeneration)
		}
	}
}

func (rt *Runtime) completeFrameView(completion RenderCompletion) (RenderCompletion, bool) {
	rt.frameViewMu.Lock()
	defer rt.frameViewMu.Unlock()
	group := rt.pendingFrameViews[completion.Frame]
	if group == nil {
		return completion, false
	}
	if completion.ResourceGeneration == 0 || group.Generation() != completion.ResourceGeneration {
		return completion, false
	}
	delete(rt.pendingFrameViews, completion.Frame)
	defer func() { _ = group.Release() }()
	if !completion.Ready || completion.Err != nil {
		return completion, true
	}
	invalid := func(reason string) (RenderCompletion, bool) {
		completion.Ready = false
		completion.Err = fmt.Errorf("runtime: ready frame invalidated: %s", reason)
		return completion, true
	}
	if completion.Frame < rt.lastReadyFrame {
		return invalid("newer frame is already ready")
	}
	if completion.WindowCount != group.WindowCount() {
		return invalid("window set count differs")
	}
	for i := 0; i < group.WindowCount(); i++ {
		want, _ := group.WindowAt(i)
		got := completion.Windows[i]
		if !got.Ready || got.WindowDescriptor != want {
			return invalid("window identity or generation differs")
		}
		if want.ID == render.PrimaryWindow && want.SurfaceGeneration != rt.surfaceLifecycleGeneration {
			return invalid("surface generation ended")
		}
	}
	next := make(map[string]*render.Frame, group.WindowCount())
	for i := 0; i < group.WindowCount(); i++ {
		descriptor, _ := group.WindowAt(i)
		frame, err := group.RetainWindow(i)
		if err != nil {
			for _, retained := range next {
				_ = retained.Release()
			}
			completion.Err = err
			completion.Ready = false
			return completion, true
		}
		next[descriptor.ID] = frame
	}
	for _, old := range rt.lastWindowFrames {
		_ = old.Release()
	}
	rt.lastWindowFrames = next
	rt.lastReadyFrame = completion.Frame
	return completion, true
}

func (rt *Runtime) releaseFrameViews() {
	rt.frameViewMu.Lock()
	defer rt.frameViewMu.Unlock()
	for frameID, frame := range rt.pendingFrameViews {
		_ = frame.Release()
		delete(rt.pendingFrameViews, frameID)
	}
	for windowID, frame := range rt.lastWindowFrames {
		_ = frame.Release()
		delete(rt.lastWindowFrames, windowID)
	}
}

type pendingFrameToken struct {
	cmd         drive.Command
	entry       *drive.Entry
	duration    time.Duration
	mode        string
	plan        *framePlan
	frontier    uint64
	frontierSet bool
	waiters     []drive.Command
}

func deferredFrame(duration time.Duration) driveOutcome {
	return driveOutcome{deferred: true, frameToken: &pendingFrameToken{duration: duration}}
}

func deferredMode(mode string) driveOutcome {
	return driveOutcome{deferred: true, frameToken: &pendingFrameToken{mode: mode}}
}
