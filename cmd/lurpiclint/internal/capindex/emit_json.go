package capindex

import (
	"encoding/json"
	"io"
)

// jsonOutput is the JSON-serializable form of the capability index.
type jsonOutput struct {
	SchemaVersion int              `json:"schema_version"`
	Capabilities  []jsonCapability `json:"capabilities"`
}

type jsonCapability struct {
	Kind        string          `json:"kind"`
	Path        string          `json:"path"`
	TypeName    string          `json:"type_name"`
	Category    string          `json:"category"`
	Constructor string          `json:"constructor,omitempty"`
	Intent      string          `json:"intent,omitempty"`
	Fingerprint jsonFingerprint `json:"fingerprint,omitempty"`
}

type jsonFingerprint struct {
	EmbedsFacet   bool     `json:"embeds_facet"`
	Roles         []string `json:"roles,omitempty"`
	HasChildSlice bool     `json:"has_child_slice"`
	IsContainer   bool     `json:"is_container"`
}

const capindexSchemaVersion = 1

// JSONEmitter writes the capability index as a versioned JSON document.
type JSONEmitter struct {
	w io.Writer
}

func (e *JSONEmitter) Emit(caps []Capability) error {
	out := jsonOutput{
		SchemaVersion: capindexSchemaVersion,
		Capabilities:  make([]jsonCapability, 0, len(caps)),
	}

	for _, c := range caps {
		jc := jsonCapability{
			Kind:        c.Kind.String(),
			Path:        c.Path,
			TypeName:    c.TypeName,
			Category:    c.Category,
			Constructor: c.Constructor,
			Intent:      c.Intent,
			Fingerprint: jsonFingerprint{
				EmbedsFacet:   c.Fingerprint.EmbedsFacet,
				Roles:         c.Fingerprint.Roles,
				HasChildSlice: c.Fingerprint.HasChildSlice,
				IsContainer:   c.Fingerprint.IsContainer,
			},
		}
		out.Capabilities = append(out.Capabilities, jc)
	}

	enc := json.NewEncoder(e.w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
