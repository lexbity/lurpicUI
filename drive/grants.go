// Grant vocabulary (RFC D11): grants are immutable per session, checked at
// admission and at execution. Explicit `command` replaces the v1 `write`
// spelling — `write` is invalid, not an alias. `admin` grants protocol
// administration only, never implicit input/command/time authority.
package drive

import (
	"fmt"
	"strings"
)

// Grant names (the canonical v2 vocabulary; the v1 "write" spelling is
// invalid, not an alias).
const (
	GrantRead    = "read"
	GrantInput   = "input"
	GrantCommand = "command"
	GrantTime    = "time"
	GrantAdmin   = "admin"
)

// Grants is the capability set baked into a session at creation.
type Grants struct {
	Read    bool
	Input   bool
	Command bool
	Time    bool
	Admin   bool
}

// ParseGrants parses a comma-separated grant list ("read,input"). Unknown
// grants — including the invalid v1 "write" spelling — fail startup. This
// is the single parse site; there is no best-effort subset.
func ParseGrants(s string) (Grants, error) {
	var g Grants
	if s == "" {
		return g, nil
	}
	for _, name := range strings.Split(s, ",") {
		switch strings.TrimSpace(name) {
		case GrantRead:
			g.Read = true
		case GrantInput:
			g.Input = true
		case GrantCommand:
			g.Command = true
		case GrantTime:
			g.Time = true
		case GrantAdmin:
			g.Admin = true
		case "write":
			return Grants{}, fmt.Errorf("drive: grant %q is not part of protocol major 2 (use %q)", "write", GrantCommand)
		default:
			return Grants{}, fmt.Errorf("drive: unknown grant %q (want read, input, command, time, admin)", name)
		}
	}
	return g, nil
}

// Allows reports whether the grant set authorizes the named grant. Admin
// authorizes protocol administration only; it does NOT imply the
// application grants.
func (g Grants) Allows(name string) bool {
	switch name {
	case "":
		return true
	case GrantRead:
		return g.Read
	case GrantInput:
		return g.Input
	case GrantCommand:
		return g.Command
	case GrantTime:
		return g.Time
	case GrantAdmin:
		return g.Admin
	default:
		return false
	}
}

// String renders the grant set as the canonical comma list.
func (g Grants) String() string {
	names := make([]string, 0, 5)
	for _, e := range []struct {
		on bool
		n  string
	}{
		{g.Read, GrantRead}, {g.Input, GrantInput}, {g.Command, GrantCommand},
		{g.Time, GrantTime}, {g.Admin, GrantAdmin},
	} {
		if e.on {
			names = append(names, e.n)
		}
	}
	return strings.Join(names, ",")
}
