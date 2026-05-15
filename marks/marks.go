package marks

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Family identifies an authored mark family.
type Family uint8

const (
	FamilyBasic Family = iota
	FamilyStructure
	FamilyAnnotation
	FamilyUIInput
	FamilyUINav
	FamilyUINotification
	FamilyChart
)

// String returns the canonical family name.
func (f Family) String() string {
	switch f {
	case FamilyBasic:
		return "basic"
	case FamilyStructure:
		return "structure"
	case FamilyAnnotation:
		return "annotation"
	case FamilyUIInput:
		return "uiinput"
	case FamilyUINav:
		return "uinav"
	case FamilyUINotification:
		return "uinotification"
	case FamilyChart:
		return "chart"
	default:
		return fmt.Sprintf("Family(%d)", uint8(f))
	}
}

// ParseFamily parses a canonical family name.
func ParseFamily(s string) (Family, bool) {
	switch normalizeName(s) {
	case "basic":
		return FamilyBasic, true
	case "structure":
		return FamilyStructure, true
	case "annotation":
		return FamilyAnnotation, true
	case "uiinput":
		return FamilyUIInput, true
	case "uinav":
		return FamilyUINav, true
	case "uinotification":
		return FamilyUINotification, true
	case "chart":
		return FamilyChart, true
	default:
		return 0, false
	}
}

// ConstructionClass identifies how a mark is authored and built.
type ConstructionClass uint8

const (
	ConstructionPrimitive ConstructionClass = iota
	ConstructionComposed
	ConstructionGenerated
)

// String returns the canonical construction class name.
func (c ConstructionClass) String() string {
	switch c {
	case ConstructionPrimitive:
		return "primitive"
	case ConstructionComposed:
		return "composed"
	case ConstructionGenerated:
		return "generated"
	default:
		return fmt.Sprintf("ConstructionClass(%d)", uint8(c))
	}
}

// ParseConstructionClass parses a canonical construction class name.
func ParseConstructionClass(s string) (ConstructionClass, bool) {
	switch normalizeName(s) {
	case "primitive":
		return ConstructionPrimitive, true
	case "composed":
		return ConstructionComposed, true
	case "generated":
		return ConstructionGenerated, true
	default:
		return 0, false
	}
}

// TypeName identifies one authored mark type.
type TypeName string

// Descriptor describes one mark type for diagnostics and tooling.
//
// Descriptors classify authored marks; they do not define shell placement or
// layer ownership.
type Descriptor struct {
	Family            Family
	ConstructionClass ConstructionClass
	Type              TypeName
	Focusable         bool
	HitTestable       bool
	AnchorExporting   bool
	ChildHosting      bool
	Customizable      bool
}

// Mark is the base authored-object contract for authored geometry.
type Mark interface {
	Descriptor() Descriptor
	AuthoredID() string
}

// FocusableMark is implemented by marks that can receive focus.
type FocusableMark interface {
	Mark
	CanFocus() bool
}

// AnchorExportingMark is implemented by marks that expose anchors.
type AnchorExportingMark interface {
	Mark
	ExportedAnchorNames() []string
}

// CustomizableMark is implemented by marks that expose controlled subpart customization.
type CustomizableMark interface {
	Mark
	SupportsSubpartCustomization() bool
}

var (
	descriptorsMu      sync.RWMutex
	descriptorsByType  = make(map[TypeName]Descriptor)
	allowedFamilyFlags = map[Family]descriptorFlags{
		FamilyBasic:          {allowChildHosting: false},
		FamilyStructure:      {allowChildHosting: true},
		FamilyAnnotation:     {allowChildHosting: true},
		FamilyUIInput:        {allowChildHosting: true},
		FamilyUINav:          {allowChildHosting: true},
		FamilyUINotification: {allowChildHosting: true},
		FamilyChart:          {allowChildHosting: true},
	}
)

type descriptorFlags struct {
	allowChildHosting bool
}

// RegisterDescriptor registers a descriptor for diagnostics/doc generation.
func RegisterDescriptor(d Descriptor) {
	validateDescriptor(d)
	descriptorsMu.Lock()
	defer descriptorsMu.Unlock()
	if _, exists := descriptorsByType[d.Type]; exists {
		panic(fmt.Sprintf("marks: descriptor already registered for %q", d.Type))
	}
	descriptorsByType[d.Type] = d
}

// DescriptorFor returns the descriptor registered for the supplied type.
func DescriptorFor(t TypeName) (Descriptor, bool) {
	descriptorsMu.RLock()
	d, ok := descriptorsByType[t]
	descriptorsMu.RUnlock()
	return d, ok
}

// AllDescriptors returns all registered descriptors in stable deterministic order.
func AllDescriptors() []Descriptor {
	descriptorsMu.RLock()
	out := make([]Descriptor, 0, len(descriptorsByType))
	for _, d := range descriptorsByType {
		out = append(out, d)
	}
	descriptorsMu.RUnlock()
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
		panic("marks: descriptor type must not be empty")
	}
	if _, ok := ParseFamily(d.Family.String()); !ok {
		panic(fmt.Sprintf("marks: invalid family %d", d.Family))
	}
	if _, ok := ParseConstructionClass(d.ConstructionClass.String()); !ok {
		panic(fmt.Sprintf("marks: invalid construction class %d", d.ConstructionClass))
	}
	flags, ok := allowedFamilyFlags[d.Family]
	if !ok {
		panic(fmt.Sprintf("marks: no validation rules for family %d", d.Family))
	}
	if d.ChildHosting && !flags.allowChildHosting {
		panic(fmt.Sprintf("marks: family %s does not allow child-hosting descriptors", d.Family))
	}
}

func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
