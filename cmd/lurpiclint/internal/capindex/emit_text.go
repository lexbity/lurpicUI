package capindex

import (
	"fmt"
	"io"
	"strings"
)

// TextEmitter writes the capability index in a human-readable format.
type TextEmitter struct {
	w io.Writer
}

func (e *TextEmitter) Emit(caps []Capability) error {
	byKind := map[CapabilityKind][]Capability{
		KindMark:   nil,
		KindLayout: nil,
		KindLayer:  nil,
	}
	for _, c := range caps {
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}

	for _, kind := range []CapabilityKind{KindMark, KindLayout, KindLayer} {
		group := byKind[kind]
		if len(group) == 0 {
			continue
		}
		kindName := strings.ToUpper(kind.String()) + "S"
		if _, err := fmt.Fprintf(e.w, "\n%s:\n", kindName); err != nil {
			return err
		}
		for _, c := range group {
			line := fmt.Sprintf("  %-45s  %s", c.Path, c.Constructor)
			if c.Intent != "" {
				line += "  " + c.Intent
			}
			if _, err := fmt.Fprintln(e.w, line); err != nil {
				return err
			}
		}
	}
	return nil
}
