package input

import (
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/internal/common"
)

func KeyFromKeysym(sym uint32) platform.Key { return common.KeyFromKeysym(sym) }

func TextFromKeysym(sym uint32) (string, bool) { return common.TextFromKeysym(sym) }

func ModifiersFromState(state uint16) platform.ModifierKeys { return common.ModifiersFromState(state) }
