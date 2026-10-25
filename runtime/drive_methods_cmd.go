package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// driveCmdList implements cmd.list (grant read): the registry's
// enumerable command specs with the registry version — the app's public
// mutation surface.
func driveCmdList(rt *Runtime, cmd drive.Command) driveOutcome {
	if len(cmd.Params) != 0 {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Params, &params); err != nil || len(params) != 0 {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.list takes an empty object"})
		}
	}
	reg := rt.CommandRegistry()
	if reg == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrCommandUnavailable, Detail: "no command registry configured"})
	}
	specs := reg.List()
	if specs == nil {
		specs = []CommandSpec{}
	}
	payload, err := json.Marshal(map[string]any{
		"commands": specs,
		"version":  uint64(reg.Version()),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// cmdExecuteParams is the cmd.execute payload.
type cmdExecuteParams struct {
	Name    string `json:"name"`
	Version uint64 `json:"version"`
}

// driveCmdExecute dispatches the exact versioned GUI command registered in
// the same CommandRegistry used by the palette.
func driveCmdExecute(rt *Runtime, cmd drive.Command) driveOutcome {
	var p cmdExecuteParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.execute requires name and version"})
	}
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.execute params: " + err.Error()})
	}
	if p.Name == "" || p.Version == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.execute requires a non-empty name and positive version"})
	}
	reg := rt.CommandRegistry()
	if reg == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrCommandUnavailable, Detail: "no command registry configured"})
	}
	entry, ok := reg.Lookup(p.Name)
	if !ok || entry.Hidden || entry.Version != p.Version {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrCommandUnavailable,
			Detail: "command is unavailable or its version changed (see cmd.list)",
		})
	}
	if entry.Disabled || entry.Execute == nil {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrCommandUnavailable,
			Detail: "command is disabled or has no GUI handler",
		})
	}
	return driveOutcome{activation: &commandActivation{
		name: p.Name, version: p.Version,
	}}
}

type opStatusParams struct {
	OperationID string `json:"operation_id"`
}

func driveOpStatus(rt *Runtime, cmd drive.Command) driveOutcome {
	var p opStatusParams
	dec := json.NewDecoder(bytes.NewReader(cmd.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil || p.OperationID == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "op.status requires operation_id"})
	}
	prefix := rt.drivePort.sessions.InstanceID() + "/" + cmd.Session + "/"
	if !strings.HasPrefix(p.OperationID, prefix) {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionFenced, Detail: "operation does not belong to this session"})
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(p.OperationID, prefix), 10, 64)
	if err != nil || id == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrOperationExpired, Detail: "operation identity is invalid"})
	}
	sess, ok := rt.drivePort.sessions.Get(cmd.Session)
	if !ok {
		return errorOutcome(&proto.Error{Code: proto.ErrSessionExpired})
	}
	status, entry := sess.Ledger().Lookup(id)
	if status != drive.LookupRetained || entry.OpID != p.OperationID {
		return errorOutcome(&proto.Error{Code: proto.ErrOperationExpired, Detail: "operation outcome is no longer retained"})
	}
	state := "running"
	var result json.RawMessage
	var failure *proto.Error
	if entry.State == drive.EntryTerminal {
		state = "completed"
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
	}
	frame := entry.AcceptedFrame
	if entry.State == drive.EntryTerminal && entry.ResultFrame != "" {
		frame = entry.ResultFrame
	}
	payload, err := json.Marshal(map[string]any{
		"operation_id": p.OperationID, "state": state, "result": result, "error": failure,
		"accepted_cut": entry.AcceptedCut, driveJSONFrameKey: frame,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}
