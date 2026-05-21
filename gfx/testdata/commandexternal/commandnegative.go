//go:build commandnegative

package commandexternal

import "codeburg.org/lexbit/lurpicui/gfx"

type externalCommand struct{}

func (externalCommand) isCommand() {}

var _ gfx.Command = externalCommand{}
