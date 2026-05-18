package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestLayerRegistryBuilder_standardLayers_haveReservedIDsAndOrders(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	ordered := reg.OrderedLayers()
	if len(ordered) != len(standardLayerReservations) {
		t.Fatalf("ordered layer count = %d, want %d", len(ordered), len(standardLayerReservations))
	}
	for i, want := range standardLayerReservations {
		got := ordered[i]
		if got.ID != want.ID || got.Name != want.Name || got.Order != want.Order {
			t.Fatalf("ordered[%d] = %#v, want %#v", i, got, want)
		}
		byID, ok := reg.Lookup(want.ID)
		if !ok || byID.Name != want.Name || byID.Order != want.Order {
			t.Fatalf("Lookup(%d) = %#v, ok=%v", want.ID, byID, ok)
		}
		byName, ok := reg.LookupName(want.Name)
		if !ok || byName.ID != want.ID || byName.Order != want.Order {
			t.Fatalf("LookupName(%q) = %#v, ok=%v", want.Name, byName, ok)
		}
	}
}

func TestLayerRegistryBuilder_rejectsReservedStandardNamesAndOrders(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if _, err := b.RegisterLayer(LayerRegistration{Name: StandardLayerBackground, Order: 2500}); err == nil {
		t.Fatal("expected reserved standard name to be rejected")
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "custom", Order: StandardLayerOrderSpatial}); err == nil {
		t.Fatal("expected reserved standard order to be rejected")
	}
}

func TestLayerRegistryBuilder_rejectsDuplicateCustomNamesOrdersAndFreeze(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if _, err := b.RegisterLayer(LayerRegistration{Name: "alpha", Order: 2500, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err != nil {
		t.Fatalf("register alpha: %v", err)
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "alpha", Order: 2600, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err == nil {
		t.Fatal("expected duplicate layer name to be rejected")
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "beta", Order: 2500, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err == nil {
		t.Fatal("expected duplicate layer order to be rejected")
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "gamma", Order: 2700, WindowBinding: WindowBinding{Kind: WindowBindingNamed, Name: "main"}}); err != nil {
		t.Fatalf("register gamma: %v", err)
	}
	if _, err := b.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "delta", Order: 2800, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err == nil {
		t.Fatal("expected registration after freeze to be rejected")
	}
}

func TestLayerRegistryBuilder_windowBindingValidation(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if _, err := b.RegisterLayer(LayerRegistration{Name: "primary", Order: 2500, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err != nil {
		t.Fatalf("primary binding rejected: %v", err)
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "named", Order: 2600, WindowBinding: WindowBinding{Kind: WindowBindingNamed, Name: "tools"}}); err != nil {
		t.Fatalf("named binding rejected: %v", err)
	}
	if _, err := b.RegisterLayer(LayerRegistration{Name: "bad-named", Order: 2700, WindowBinding: WindowBinding{Kind: WindowBindingNamed}}); err == nil {
		t.Fatal("expected empty named window binding to be rejected")
	}
}

func TestLayerRegistryBuilder_preservesFocusTrapMetadata(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if _, err := b.RegisterLayer(LayerRegistration{
		Name:         "modal",
		Order:        2500,
		WindowBinding: WindowBinding{Kind: WindowBindingPrimary},
		FocusTrap:    true,
		FocusRestore: facet.FocusRestoreFirstFocusable,
	}); err != nil {
		t.Fatalf("register modal: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	desc, ok := reg.LookupName("modal")
	if !ok {
		t.Fatal("missing modal layer")
	}
	if !desc.FocusTrap {
		t.Fatal("focus trap flag was not preserved")
	}
	if desc.FocusRestore != facet.FocusRestoreFirstFocusable {
		t.Fatalf("focus restore = %v, want %v", desc.FocusRestore, facet.FocusRestoreFirstFocusable)
	}
}

func TestLayerID_zeroValue_isStable(t *testing.T) {
	var id LayerID
	if id != 0 {
		t.Fatalf("zero LayerID = %d, want 0", id)
	}
	if id == StandardLayerIDBackground {
		t.Fatal("zero LayerID unexpectedly matches a reserved standard layer ID")
	}
}

func TestLayerRegistryBuilder_customIDs_skip_standardReservations(t *testing.T) {
	b := NewLayerRegistryBuilder()
	if _, err := b.RegisterLayer(LayerRegistration{Name: "alpha", Order: 2500, WindowBinding: WindowBinding{Kind: WindowBindingPrimary}}); err != nil {
		t.Fatalf("register alpha: %v", err)
	}
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	alpha, ok := reg.LookupName("alpha")
	if !ok {
		t.Fatal("missing custom layer alpha")
	}
	if alpha.ID == StandardLayerIDBackground || alpha.ID == StandardLayerIDDebug {
		t.Fatalf("custom layer id %d collided with standard reservation", alpha.ID)
	}
	if alpha.Order != 2500 {
		t.Fatalf("alpha order = %d, want 2500", alpha.Order)
	}
}
