//go:build linux && cgo

package linux

import "codeburg.org/lexbit/lurpicui/platform"

const (
	keysymBackSpace = 0xff08
	keysymTab       = 0xff09
	keysymReturn    = 0xff0d
	keysymEscape    = 0xff1b
	keysymHome      = 0xff50
	keysymLeft      = 0xff51
	keysymUp        = 0xff52
	keysymRight     = 0xff53
	keysymDown      = 0xff54
	keysymPageUp    = 0xff55
	keysymPageDown  = 0xff56
	keysymEnd       = 0xff57
)

func keyFromKeysym(sym uint32) platform.Key {
	switch sym {
	case keysymLeft:
		return platform.KeyLeft
	case keysymRight:
		return platform.KeyRight
	case keysymUp:
		return platform.KeyUp
	case keysymDown:
		return platform.KeyDown
	case keysymHome:
		return platform.KeyHome
	case keysymEnd:
		return platform.KeyEnd
	case keysymPageUp:
		return platform.KeyPageUp
	case keysymPageDown:
		return platform.KeyPageDown
	case keysymEscape:
		return platform.KeyEscape
	case keysymReturn:
		return platform.KeyEnter
	case keysymTab:
		return platform.KeyTab
	case keysymBackSpace:
		return platform.KeyBackspace
	}

	if sym >= 'A' && sym <= 'Z' {
		return platform.Key(int(sym-'A') + int(platform.KeyA))
	}
	if sym >= 'a' && sym <= 'z' {
		return platform.Key(int(sym-'a') + int(platform.KeyA))
	}
	return platform.KeyUnknown
}

func textFromKeysym(sym uint32) (string, bool) {
	if sym >= ' ' && sym <= '~' {
		return string(rune(sym)), true
	}
	switch sym {
	case keysymTab:
		return "\t", true
	case keysymReturn:
		return "\n", true
	}
	return "", false
}

func modifiersFromState(state uint16) platform.ModifierKeys {
	var mods platform.ModifierKeys
	const (
		xcbModShift   = 1 << 0
		xcbModLock    = 1 << 1
		xcbModControl = 1 << 2
		xcbMod1       = 1 << 3
		xcbMod4       = 1 << 6
	)
	if state&xcbModShift != 0 {
		mods |= platform.ModShift
	}
	if state&xcbModControl != 0 {
		mods |= platform.ModControl
	}
	if state&xcbMod1 != 0 {
		mods |= platform.ModAlt
	}
	if state&xcbMod4 != 0 {
		mods |= platform.ModSuper
	}
	_ = xcbModLock
	return mods
}
