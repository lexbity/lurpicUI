package runtime

import (
	"encoding/json"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

// driveCmdList implements cmd.list (grant read, Debt-3): the registry's
// enumerable command specs with the registry version — the app's public
// mutation surface.
func driveCmdList(rt *Runtime, cmd drive.Command) driveOutcome {
	reg := rt.CommandRegistry()
	if reg == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "no command registry configured"}, rt.driveSink.Epoch())
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
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
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
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "cmd.execute requires id"}, rt.driveSink.Epoch())
	}
	if err := json.Unmarshal(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "cmd.execute params: " + err.Error()}, rt.driveSink.Epoch())
	}
	reg := rt.CommandRegistry()
	if reg == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "no command registry configured"}, rt.driveSink.Epoch())
	}
	entry, ok := reg.Lookup(p.ID)
	if !ok {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrUnresolved,
			Detail: "no command " + p.ID + " (see cmd.list)",
		}, rt.driveSink.Epoch())
	}
	if entry.Disabled || !reg.Execute(p.ID) {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrMutating,
			Detail: "command " + p.ID + " is disabled or has no handler",
		}, rt.driveSink.Epoch())
	}
	payload, err := json.Marshal(map[string]any{
		"executed": p.ID,
		"version":  uint64(reg.Version()),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}
