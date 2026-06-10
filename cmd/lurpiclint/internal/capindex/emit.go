package capindex

import "io"

// NewTextEmitter creates a text-formatted capability index emitter.
func NewTextEmitter(w io.Writer) *TextEmitter {
	return &TextEmitter{w: w}
}

// NewJSONEmitter creates a JSON-formatted capability index emitter.
func NewJSONEmitter(w io.Writer) *JSONEmitter {
	return &JSONEmitter{w: w}
}
