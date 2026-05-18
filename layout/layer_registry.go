package layout

import (
	"fmt"
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
)

// LayerID is the canonical globally registered layer identifier.
type LayerID uint64

// LayerName identifies a globally registered layer.
type LayerName string

// LayerOrder is the explicit global ordering used by the registry.
type LayerOrder int32

// WindowBindingKind identifies which window a layer targets.
type WindowBindingKind uint8

const (
	WindowBindingPrimary WindowBindingKind = iota
	WindowBindingNamed
)

// WindowBinding binds a layer to a platform window.
type WindowBinding struct {
	Kind WindowBindingKind
	Name string
}

// DismissalTrigger identifies a dismissal input path.
type DismissalTrigger uint8

const (
	DismissalTriggerPointer DismissalTrigger = iota
	DismissalTriggerKey
	DismissalTriggerFocusLoss
)

// DismissalTriggerSet is a bitset of enabled dismissal triggers.
type DismissalTriggerSet uint8

const (
	DismissalTriggerSetPointer DismissalTriggerSet = 1 << iota
	DismissalTriggerSetKey
	DismissalTriggerSetFocusLoss
)

// OrderRange describes a half-open order range.
type OrderRange struct {
	Min int32
	Max int32
}

// DismissalScope describes outside-click dismissal behavior for a layer.
type DismissalScope struct {
	Enabled      bool
	BehindOrders OrderRange
	Triggers     DismissalTriggerSet
}

// LayerLayoutRecipeRef points at an app/theme layout recipe.
type LayerLayoutRecipeRef struct {
	Family string
	Name   string
}

// StandardLayer is a hardcoded standard layer reservation.
type StandardLayer struct {
	ID    LayerID
	Name  LayerName
	Order LayerOrder
}

const (
	StandardLayerIDBackground LayerID = 1
	StandardLayerIDBase       LayerID = 2
	StandardLayerIDSpatial    LayerID = 3
	StandardLayerIDForeground LayerID = 4
	StandardLayerIDFloating   LayerID = 5
	StandardLayerIDOverlay    LayerID = 6
	StandardLayerIDModal      LayerID = 7
	StandardLayerIDStatus     LayerID = 8
	StandardLayerIDDebug      LayerID = 9
)

var standardLayerReservations = []StandardLayer{
	{ID: StandardLayerIDBackground, Name: StandardLayerBackground, Order: StandardLayerOrderBackground},
	{ID: StandardLayerIDBase, Name: StandardLayerBase, Order: StandardLayerOrderBase},
	{ID: StandardLayerIDSpatial, Name: StandardLayerSpatial, Order: StandardLayerOrderSpatial},
	{ID: StandardLayerIDForeground, Name: StandardLayerForeground, Order: StandardLayerOrderForeground},
	{ID: StandardLayerIDFloating, Name: StandardLayerFloating, Order: StandardLayerOrderFloating},
	{ID: StandardLayerIDOverlay, Name: StandardLayerOverlay, Order: StandardLayerOrderOverlay},
	{ID: StandardLayerIDModal, Name: StandardLayerModal, Order: StandardLayerOrderModal},
	{ID: StandardLayerIDStatus, Name: StandardLayerStatus, Order: StandardLayerOrderStatus},
	{ID: StandardLayerIDDebug, Name: StandardLayerDebug, Order: StandardLayerOrderDebug},
}

// LayerDescriptor describes one frozen layer registration.
type LayerDescriptor struct {
	ID            LayerID
	Name          LayerName
	Order         LayerOrder
	WindowBinding WindowBinding
	CoordSpace    CoordSpace
	HitPolicy     LayerHitPolicy
	ClipPolicy    ClipPolicy
	Dismissal     DismissalScope
	FocusTrap     bool
	FocusRestore  facet.FocusRestoreMode
	LayoutRecipe  LayerLayoutRecipeRef
}

// LayerRegistration is the startup-time registration input for a layer.
type LayerRegistration struct {
	Name          LayerName
	Order         LayerOrder
	WindowBinding WindowBinding
	CoordSpace    CoordSpace
	HitPolicy     LayerHitPolicy
	ClipPolicy    ClipPolicy
	Dismissal     DismissalScope
	FocusTrap     bool
	FocusRestore  facet.FocusRestoreMode
	LayoutRecipe  LayerLayoutRecipeRef
}

// Standard layer names reserved by the foundation.
const (
	StandardLayerBackground LayerName = "StandardLayer_Background"
	StandardLayerBase       LayerName = "StandardLayer_Base"
	StandardLayerSpatial    LayerName = "StandardLayer_Spatial"
	StandardLayerForeground LayerName = "StandardLayer_Foreground"
	StandardLayerFloating   LayerName = "StandardLayer_Floating"
	StandardLayerOverlay    LayerName = "StandardLayer_Overlay"
	StandardLayerModal      LayerName = "StandardLayer_Modal"
	StandardLayerStatus     LayerName = "StandardLayer_Status"
	StandardLayerDebug      LayerName = "StandardLayer_Debug"
)

// Standard layer orders reserved by the foundation.
const (
	StandardLayerOrderBackground LayerOrder = 1000
	StandardLayerOrderBase       LayerOrder = 2000
	StandardLayerOrderSpatial    LayerOrder = 3000
	StandardLayerOrderForeground LayerOrder = 4000
	StandardLayerOrderFloating   LayerOrder = 5000
	StandardLayerOrderOverlay    LayerOrder = 6000
	StandardLayerOrderModal      LayerOrder = 7000
	StandardLayerOrderStatus     LayerOrder = 8000
	StandardLayerOrderDebug      LayerOrder = 9000
)

// LayerRegistryBuilder assembles a frozen registry snapshot.
type LayerRegistryBuilder struct {
	nextID             LayerID
	frozen             bool
	standardRegistered bool
	descriptors        []LayerDescriptor
	byName             map[LayerName]LayerID
	byOrder            map[LayerOrder]LayerID
	reservedIDs        map[LayerID]struct{}
	reservedNames      map[LayerName]struct{}
	reservedOrders     map[LayerOrder]struct{}
}

// NewLayerRegistryBuilder creates an empty registry builder.
func NewLayerRegistryBuilder() *LayerRegistryBuilder {
	b := &LayerRegistryBuilder{
		nextID:         1,
		byName:         make(map[LayerName]LayerID),
		byOrder:        make(map[LayerOrder]LayerID),
		reservedIDs:    make(map[LayerID]struct{}, len(standardLayerReservations)),
		reservedNames:  make(map[LayerName]struct{}, len(standardLayerReservations)),
		reservedOrders: make(map[LayerOrder]struct{}, len(standardLayerReservations)),
	}
	for _, layer := range standardLayerReservations {
		b.reservedIDs[layer.ID] = struct{}{}
		b.reservedNames[layer.Name] = struct{}{}
		b.reservedOrders[layer.Order] = struct{}{}
	}
	return b
}

// RegisterStandardLayers adds the hardcoded standard layer reservations.
func (b *LayerRegistryBuilder) RegisterStandardLayers() error {
	if b == nil {
		return fmt.Errorf("layout: nil layer registry builder")
	}
	if b.frozen {
		return fmt.Errorf("layout: layer registry builder is frozen")
	}
	if b.standardRegistered {
		return fmt.Errorf("layout: standard layers already registered")
	}
	for _, layer := range standardLayerReservations {
		if err := b.registerStandardLayer(layer); err != nil {
			return err
		}
	}
	b.standardRegistered = true
	return nil
}

// RegisterLayer inserts one layer into the builder.
func (b *LayerRegistryBuilder) RegisterLayer(spec LayerRegistration) (LayerID, error) {
	if b == nil {
		return 0, fmt.Errorf("layout: nil layer registry builder")
	}
	if b.frozen {
		return 0, fmt.Errorf("layout: layer registry builder is frozen")
	}
	if spec.Name == "" {
		return 0, fmt.Errorf("layout: cannot register layer with empty name")
	}
	if b.isReservedName(spec.Name) {
		return 0, fmt.Errorf("layout: cannot register layer %q: reserved standard layer name", spec.Name)
	}
	if b.isReservedOrder(spec.Order) {
		return 0, fmt.Errorf("layout: cannot register layer %q at order %d: reserved standard layer order", spec.Name, spec.Order)
	}
	if _, ok := b.byName[spec.Name]; ok {
		return 0, fmt.Errorf("layout: cannot register layer %q: duplicate name", spec.Name)
	}
	if _, ok := b.byOrder[spec.Order]; ok {
		return 0, fmt.Errorf("layout: cannot register layer %q at order %d: duplicate order", spec.Name, spec.Order)
	}
	if spec.WindowBinding.Kind == WindowBindingNamed && spec.WindowBinding.Name == "" {
		return 0, fmt.Errorf("layout: cannot register layer %q with named window binding and empty name", spec.Name)
	}
	id := b.nextAvailableID()
	desc := LayerDescriptor{
		ID:            id,
		Name:          spec.Name,
		Order:         spec.Order,
		WindowBinding: spec.WindowBinding,
		CoordSpace:    spec.CoordSpace,
		HitPolicy:     spec.HitPolicy,
		ClipPolicy:    spec.ClipPolicy,
		Dismissal:     spec.Dismissal,
		FocusTrap:     spec.FocusTrap,
		FocusRestore:  spec.FocusRestore,
		LayoutRecipe:  spec.LayoutRecipe,
	}
	b.descriptors = append(b.descriptors, desc)
	b.byName[spec.Name] = id
	b.byOrder[spec.Order] = id
	return id, nil
}

func (b *LayerRegistryBuilder) registerStandardLayer(layer StandardLayer) error {
	if layer.ID == 0 {
		return fmt.Errorf("layout: standard layer %q must have a non-zero id", layer.Name)
	}
	if layer.Name == "" {
		return fmt.Errorf("layout: standard layer id %d has empty name", layer.ID)
	}
	if _, ok := b.byID()[layer.ID]; ok {
		return fmt.Errorf("layout: cannot register standard layer %q: duplicate id %d", layer.Name, layer.ID)
	}
	if _, ok := b.byName[layer.Name]; ok {
		return fmt.Errorf("layout: cannot register standard layer %q: duplicate name", layer.Name)
	}
	if _, ok := b.byOrder[layer.Order]; ok {
		return fmt.Errorf("layout: cannot register standard layer %q at order %d: duplicate order", layer.Name, layer.Order)
	}
	desc := LayerDescriptor{ID: layer.ID, Name: layer.Name, Order: layer.Order}
	b.descriptors = append(b.descriptors, desc)
	b.byName[layer.Name] = layer.ID
	b.byOrder[layer.Order] = layer.ID
	b.reservedIDs[layer.ID] = struct{}{}
	b.reservedNames[layer.Name] = struct{}{}
	b.reservedOrders[layer.Order] = struct{}{}
	if layer.ID >= b.nextID {
		b.nextID = layer.ID + 1
	}
	return nil
}

func (b *LayerRegistryBuilder) nextAvailableID() LayerID {
	if b == nil {
		return 0
	}
	id := b.nextID
	for {
		if _, ok := b.reservedIDs[id]; ok {
			id++
			continue
		}
		if _, ok := b.byID()[id]; ok {
			id++
			continue
		}
		b.nextID = id + 1
		return id
	}
}

func (b *LayerRegistryBuilder) byID() map[LayerID]LayerDescriptor {
	if b == nil {
		return nil
	}
	out := make(map[LayerID]LayerDescriptor, len(b.descriptors))
	for _, desc := range b.descriptors {
		out[desc.ID] = desc
	}
	return out
}

func (b *LayerRegistryBuilder) isReservedName(name LayerName) bool {
	if b == nil {
		return false
	}
	_, ok := b.reservedNames[name]
	return ok
}

func (b *LayerRegistryBuilder) isReservedOrder(order LayerOrder) bool {
	if b == nil {
		return false
	}
	_, ok := b.reservedOrders[order]
	return ok
}

// Freeze produces an immutable registry snapshot.
func (b *LayerRegistryBuilder) Freeze() (*LayerRegistry, error) {
	if b == nil {
		return nil, fmt.Errorf("layout: nil layer registry builder")
	}
	b.frozen = true
	descs := append([]LayerDescriptor(nil), b.descriptors...)
	sort.SliceStable(descs, func(i, j int) bool {
		if descs[i].Order != descs[j].Order {
			return descs[i].Order < descs[j].Order
		}
		return descs[i].ID < descs[j].ID
	})
	registry := &LayerRegistry{
		descriptors: descs,
		byID:        make(map[LayerID]LayerDescriptor, len(descs)),
		byName:      make(map[LayerName]LayerDescriptor, len(descs)),
	}
	for _, desc := range descs {
		registry.byID[desc.ID] = desc
		registry.byName[desc.Name] = desc
	}
	return registry, nil
}

// StandardLayerRegistry returns the frozen standard layer registry snapshot.
func StandardLayerRegistry() (*LayerRegistry, error) {
	b := NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		return nil, err
	}
	return b.Freeze()
}

// LayerRegistry is an immutable frozen registry snapshot.
type LayerRegistry struct {
	descriptors []LayerDescriptor
	byID        map[LayerID]LayerDescriptor
	byName      map[LayerName]LayerDescriptor
}

// Lookup returns the descriptor for one layer ID.
func (r *LayerRegistry) Lookup(id LayerID) (LayerDescriptor, bool) {
	if r == nil {
		return LayerDescriptor{}, false
	}
	desc, ok := r.byID[id]
	return desc, ok
}

// LookupName returns the descriptor for one layer name.
func (r *LayerRegistry) LookupName(name LayerName) (LayerDescriptor, bool) {
	if r == nil {
		return LayerDescriptor{}, false
	}
	desc, ok := r.byName[name]
	return desc, ok
}

// OrderedLayers returns the registry in ascending global order.
func (r *LayerRegistry) OrderedLayers() []LayerDescriptor {
	if r == nil || len(r.descriptors) == 0 {
		return nil
	}
	out := make([]LayerDescriptor, len(r.descriptors))
	copy(out, r.descriptors)
	return out
}

// MustLookup returns the descriptor for id or panics.
func (r *LayerRegistry) MustLookup(id LayerID) LayerDescriptor {
	desc, ok := r.Lookup(id)
	if !ok {
		panic(fmt.Sprintf("layout: unknown layer %d", id))
	}
	return desc
}
