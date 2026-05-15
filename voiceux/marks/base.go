package marks

import (
	"sort"
	"strings"
	"sync"
)

// Family identifies a Voice UX mark family.
type Family string

const (
	FamilyVoice Family = "voice"
)

// ConstructionClass identifies how a Voice UX mark is authored.
type ConstructionClass string

const (
	ConstructionPrimitive ConstructionClass = "primitive"
	ConstructionComposed  ConstructionClass = "composed"
	ConstructionGenerated ConstructionClass = "generated"
)

// TypeName identifies one Voice UX mark type.
type TypeName string

// Descriptor describes one Voice UX mark type.
type Descriptor struct {
	Family            Family
	ConstructionClass ConstructionClass
	Type              TypeName
	Focusable         bool
	HitTestable       bool
	ChildHosting      bool
}

// Mark is the local Voice UX authored-object contract.
type Mark interface {
	Descriptor() Descriptor
	AuthoredID() string
}

type baseMark struct {
	id   string
	desc Descriptor
}

var (
	descriptorsMu sync.Mutex
	descriptors   []Descriptor
)

func registerDescriptor(d Descriptor) {
	descriptorsMu.Lock()
	descriptors = append(descriptors, d)
	descriptorsMu.Unlock()
}

// Descriptors returns only the descriptors authored by the Voice UX package.
func Descriptors() []Descriptor {
	descriptorsMu.Lock()
	out := append([]Descriptor(nil), descriptors...)
	descriptorsMu.Unlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].ConstructionClass < out[j].ConstructionClass
	})
	return out
}

func validateDescriptor(d Descriptor) {
	if strings.TrimSpace(string(d.Type)) == "" {
		panic("voiceux/marks: descriptor type must not be empty")
	}
	if strings.TrimSpace(string(d.Family)) == "" {
		panic("voiceux/marks: descriptor family must not be empty")
	}
	if strings.TrimSpace(string(d.ConstructionClass)) == "" {
		panic("voiceux/marks: descriptor construction class must not be empty")
	}
}

func (m baseMark) Descriptor() Descriptor { return m.desc }
func (m baseMark) AuthoredID() string     { return m.id }
