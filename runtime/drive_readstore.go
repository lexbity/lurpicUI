package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"

	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/readstore"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/store"
)

// initNamedRegistry validates the startup named-store set (fail-loud at
// construction, never mid-frame).
func (rt *Runtime) initNamedRegistry() error {
	reg, err := readstore.NewRegistry(rt.config.NamedStores)
	if err != nil {
		return err
	}
	rt.namedStores = reg
	return nil
}

// RegisterNamedStore binds an app-domain store after construction (the
// studio registers in its root builder). Runtime thread; must run before
// frames depend on the name. Freeze tests make the registered set a
// reviewable diff.
func (rt *Runtime) RegisterNamedStore(name string, s any) error {
	syncutil.AssertRuntimeThread()
	if rt.namedStores == nil {
		reg, err := readstore.NewRegistry(nil)
		if err != nil {
			return err
		}
		rt.namedStores = reg
	}
	return rt.namedStores.Register(name, s)
}

// storeTarget is the shared store address (QP-8): a named address ("name")
// or a structural one ({target locator, field}). Exactly one form per
// request.
type storeTarget struct {
	Name   string          `json:"name,omitempty"`
	Target *locate.Locator `json:"target,omitempty"`
	Field  string          `json:"field,omitempty"`
}

// resolve resolves the target to a store plus a human-readable address label.
func (t storeTarget) resolve(rt *Runtime) (any, string, *proto.Error) {
	switch {
	case t.Name != "":
		s, ok := rt.namedStores.Lookup(t.Name)
		if !ok {
			return nil, "", &proto.Error{
				Code:   proto.ErrUnresolved,
				Detail: fmt.Sprintf("no named store %q (see store.list)", t.Name),
			}
		}
		return s, t.Name, nil
	case t.Target != nil && t.Field != "":
		m, _, lerr := locate.Resolve(rt.root, *t.Target, rt.driveDescribe, rt.driveHitTest)
		if lerr != nil {
			return nil, "", driveLocateError(lerr)
		}
		s, err := readstore.FieldStore(m.Impl, t.Field)
		if err != nil {
			return nil, "", &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}
		}
		return s, m.Path + "." + t.Field, nil
	default:
		return nil, "", &proto.Error{
			Code:   proto.ErrInternal,
			Detail: `store address requires "name" or both "target" and "field"`,
		}
	}
}

// driveStoreList implements store.list (grant read): the union of the named
// registry and the tree's enumerable facet store fields.
func driveStoreList(rt *Runtime, _ drive.Command) driveOutcome {
	type entry struct {
		Target string `json:"target"`
		Field  string `json:"field,omitempty"`
		Kind   string `json:"kind"`
	}
	out := make([]entry, 0, 16)
	for _, name := range rt.namedStores.Names() {
		s, _ := rt.namedStores.Lookup(name)
		out = append(out, entry{Target: name, Kind: readstore.Kind(s)})
	}
	// Structural surface: facet-tree children only. Hosted rows' stores
	// belong to their hosts, whose own fields are enumerated here.
	stack := []facet.FacetImpl{rt.root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || n.Base() == nil {
			continue
		}
		path := locate.PathFor(n.Base())
		for _, f := range readstore.StructFields(n) {
			out = append(out, entry{Target: path, Field: f.Name, Kind: f.Kind})
		}
		children := n.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			if c := children[i].Impl(); c != nil {
				stack = append(stack, c)
			}
		}
	}
	payload, err := json.Marshal(map[string]any{"stores": out})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// driveStoreRead implements store.read (grant read, cut-exact): the store's
// current value and version.
func driveStoreRead(rt *Runtime, cmd drive.Command) driveOutcome {
	var p storeTarget
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "store.read params: " + err.Error()}, rt.driveSink.Epoch())
		}
	}
	s, label, perr := p.resolve(rt)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	value, err := readstore.ReadValue(s)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	payload, err := json.Marshal(map[string]any{
		"address": label,
		"value":   value,
		"version": storeVersionOf(s),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// storeVersionOf reads a store's version through its canonical method
// (uint64-typed Version across all three store shapes).
func storeVersionOf(s any) uint64 {
	if v, ok := s.(interface{ Version() store.Version }); ok {
		return uint64(v.Version())
	}
	return 0
}

// storeWriteParams is the store.write payload.
type storeWriteParams struct {
	storeTarget
	Value json.RawMessage `json:"value,omitempty"`
}

// driveStoreWrite implements store.write (grant write, QP-12): the
// debugging tool of last resort. Responses carry bypassed_app_invariants
// and the write lands in the drive frame log; the preferred mutation paths
// are input.* and cmd.execute.
func driveStoreWrite(rt *Runtime, cmd drive.Command) driveOutcome {
	var p storeWriteParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "store.write params: " + err.Error()}, rt.driveSink.Epoch())
		}
	}
	s, label, perr := p.resolve(rt)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	if err := readstore.WriteValue(s, p.Value); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	rt.driveSink.recordWrite(label)
	payload, err := json.Marshal(map[string]any{
		"address":                 label,
		"bypassed_app_invariants": true,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// driveDerivedRead implements derived.read (grant read, QP-9): Peek only —
// an observation must not perturb the observed frame.
func driveDerivedRead(rt *Runtime, cmd drive.Command) driveOutcome {
	var p storeTarget
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "derived.read params: " + err.Error()}, rt.driveSink.Epoch())
		}
	}
	s, label, perr := p.resolve(rt)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	value, dirty, err := readstore.ReadDerivedPeek(s)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	payload, err := json.Marshal(map[string]any{
		"address": label,
		"value":   value,
		"dirty":   dirty,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// facetQueryParams is the facet.query payload.
type facetQueryParams struct {
	Target  locate.Locator `json:"target"`
	Details bool           `json:"details,omitempty"`
}

// driveFacetQuery implements facet.query (grant read, FR-12): descriptor,
// TestID, arranged bounds, measured size, hit surface, focus state, layer
// attachment, and opt-in DriveDetails (the sanctioned bespoke-host escape
// hatch, §3.7).
func driveFacetQuery(rt *Runtime, cmd drive.Command) driveOutcome {
	var p facetQueryParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "facet.query params: " + err.Error()}, rt.driveSink.Epoch())
		}
	}
	m, _, lerr := locate.Resolve(rt.root, p.Target, rt.driveDescribe, rt.driveHitTest)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr), rt.driveSink.Epoch())
	}
	out := map[string]any{
		"path":     m.Path,
		"testid":   m.Base.TestID(),
		"facet_id": uint64(m.Base.ID()),
	}
	meta := rt.driveDescribe(m.Impl)
	out["kind"] = meta.Kind
	out["role"] = meta.Role
	out["name"] = meta.Name
	if lr := m.Base.LayoutRole(); lr != nil {
		if b := lr.ArrangedBounds; !b.IsEmpty() {
			out["bounds"] = b
		}
		out["measured"] = map[string]float32{"w": lr.MeasuredSize.W, "h": lr.MeasuredSize.H}
	}
	if m.Base.IsLayer() {
		att := m.Base.LayerAttachment()
		layer := map[string]any{"band": att.Band.String(), "order": att.Order}
		if att.Mount != nil {
			layer["mount"] = att.Mount.Get()
		}
		out["layer"] = layer
	}
	if rt.focusManager != nil {
		if focused := rt.focusManager.FocusedImpl(); focused != nil && focused.Base() != nil &&
			focused.Base().ID() == m.Base.ID() {
			out["focused"] = true
		}
	}
	hitDeclared := false
	if hr := m.Base.HitRole(); hr != nil {
		hitDeclared = hr.OnHitTest != nil
	}
	out["hit_declared"] = hitDeclared
	if p.Details {
		if d, ok := m.Impl.(interface{ DriveDetails() map[string]any }); ok {
			out["details"] = d.DriveDetails()
		} else {
			out["details"] = map[string]any{}
		}
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// driveFramePixels implements frame.pixels (grant read, FR-13): PNG bytes of
// the current software surface. Requires a software surface (the software
// backend's target); a Vulkan-only surface refuses with a typed error until
// the readback path lands with render/equivalence (the tolerance tagging
// rides that landing).
func driveFramePixels(rt *Runtime, _ drive.Command) driveOutcome {
	surface := rt.softwareSurface()
	if surface == nil {
		return errorOutcome(&proto.Error{
			Code: proto.ErrInternal,
			Detail: "frame.pixels requires a software surface " +
				"(vulkan readback lands with render/equivalence, tolerance psnr>=40db)",
		}, rt.driveSink.Epoch())
	}
	if err := surface.Lock(); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "surface lock: " + err.Error()}, rt.driveSink.Epoch())
	}
	w, h := surface.Size()
	img := imageFromBuffer(surface.Buffer(), surface.Stride(), w, h)
	_ = surface.Unlock(nil)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "png encode: " + err.Error()}, rt.driveSink.Epoch())
	}
	payload, err := json.Marshal(map[string]any{
		"png":    pngDataURL(out.Bytes()),
		"width":  w,
		"height": h,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// driveFramePacket implements frame.packet (grant read): the last submitted
// primary frame's packet digest and epoch.
func driveFramePacket(rt *Runtime, _ drive.Command) driveOutcome {
	frame := rt.LastPrimaryFrame()
	if frame == nil {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInternal,
			Detail: "no frame submitted yet",
		}, rt.driveSink.Epoch())
	}
	payload, err := json.Marshal(map[string]any{
		"epoch":   rt.driveSink.Epoch(),
		"hash":    rt.FramePacketHash(),
		"batches": len(frame.RenderBatchs),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// softwareSurface returns the render target when it is a software surface
// (nil for Vulkan-only windows).
func (rt *Runtime) softwareSurface() render.SoftwareSurface {
	if rt.window == nil {
		return nil
	}
	if s, ok := rt.window.Surface().(render.SoftwareSurface); ok {
		return s
	}
	return nil
}

// imageFromBuffer wraps the surface's RGBA buffer (stride-padded) as an
// image without copying rows.
func imageFromBuffer(buf []byte, stride, w, h int) image.Image {
	return &image.RGBA{
		Pix:    buf,
		Stride: stride,
		Rect:   image.Rect(0, 0, w, h),
	}
}

// pngDataURL encodes PNG bytes as a data URL (a JSON-safe pixel transport
// for frame.pixels).
func pngDataURL(pngBytes []byte) string {
	const prefix = "data:image/png;base64,"
	const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb bytes.Buffer
	sb.WriteString(prefix)
	for i := 0; i < len(pngBytes); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], pngBytes[i:])
		b := uint32(chunk[0])<<16 | uint32(chunk[1])<<8 | uint32(chunk[2])
		sb.WriteByte(b64[(b>>18)&0x3F])
		sb.WriteByte(b64[(b>>12)&0x3F])
		if n > 1 {
			sb.WriteByte(b64[(b>>6)&0x3F])
		} else {
			sb.WriteByte('=')
		}
		if n > 2 {
			sb.WriteByte(b64[b&0x3F])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}
