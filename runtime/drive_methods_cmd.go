package runtime

import (
	"encoding/json"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// driveCmdList implements cmd.list (grant read, Debt-3): the registry's
// enumerable command specs with the registry version — the app's public
// mutation surface.
func driveCmdList(rt *Runtime, cmd drive.Command) driveOutcome {
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
	ID string `json:"id"`
}

// driveCmdExecute implements cmd.execute (grant write, FR-16): dispatch the
// existing CommandRegistry — the app-semantic mutation path, preferred over
// store.write (QP-12).
func driveCmdExecute(rt *Runtime, cmd drive.Command) driveOutcome {
	var p cmdExecuteParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.execute requires id"})
	}
	if err := json.Unmarshal(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "cmd.execute params: " + err.Error()})
	}
	reg := rt.CommandRegistry()
	if reg == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrCommandUnavailable, Detail: "no command registry configured"})
	}
	entry, ok := reg.Lookup(p.ID)
	if !ok {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrUnresolved,
			Detail: "no command " + p.ID + " (see cmd.list)",
		})
	}
	if entry.Disabled || !reg.Execute(p.ID) {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInvalidParams,
			Detail: "command " + p.ID + " is disabled or has no handler",
		})
	}
	payload, err := json.Marshal(map[string]any{
		"executed": p.ID,
		"version":  uint64(reg.Version()),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}
