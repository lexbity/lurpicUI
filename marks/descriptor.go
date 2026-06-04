package marks

// Descriptor carries the static metadata for a mark family member.
// Authors declare only Family and TypeName; capability flags are derived
// by Describe() and cannot be set by the author.
type Descriptor struct {
	Family         string
	TypeName       string
	HostsChildren  bool
	Focusable      bool
	ExportsAnchors bool
	Accessible     bool
	HitTestable    bool
	DataBound      bool
}

// Describe recomputes a complete Descriptor for m.
// Family and TypeName come from m.Descriptor(); capability flags are
// derived by static type assertion — not reflect, not role discovery.
func Describe(m Mark) Descriptor {
	d := m.Descriptor()

	if _, ok := m.(Focusable); ok {
		d.Focusable = true
	}
	if _, ok := m.(AnchorExporting); ok {
		d.ExportsAnchors = true
	}
	if _, ok := m.(Accessible); ok {
		d.Accessible = true
	}
	if _, ok := m.(Composite); ok {
		d.HostsChildren = true
	}
	if _, ok := m.(DataBound); ok {
		d.DataBound = true
	}
	if base := m.Base(); base != nil && base.HitRole() != nil {
		d.HitTestable = true
	}
	return d
}
