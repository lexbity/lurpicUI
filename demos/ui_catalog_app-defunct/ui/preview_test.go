package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	catalogstore "codeburg.org/lexbit/ui_catalog/store"
)

type previewRuntimeStub struct {
	added []layout.ChildAttachment
}

func (s *previewRuntimeStub) AddFacet(_, child facet.FacetImpl, attachment layout.ChildAttachment) {
	_ = child
	s.added = append(s.added, attachment)
}

func (s *previewRuntimeStub) RemoveFacet(child facet.FacetImpl) {
	_ = child
}

func (s *previewRuntimeStub) RequestFrame() {}

func TestPreviewFacet_buildSceneChildrenReturnsDirectRoots(t *testing.T) {
	p := NewPreviewFacet(theme.Default(), nil)
	entry := &model.CatalogEntry{
		ID:          "annotation.connector",
		DisplayName: "Connector",
		Family:      model.FamilyAnnotation,
	}

	scene := p.buildSceneChildren(entry)
	if len(scene) != 4 {
		t.Fatalf("scene children = %d, want 4 direct roots", len(scene))
	}

	if _, ok := scene[0].(*basic.Rect); !ok {
		t.Fatalf("scene child type = %T, want direct mark root", scene[0])
	}
}

func TestPreviewFacet_usesTopLeftAttachment(t *testing.T) {
	att := previewChildAttachment(0)
	if att.LayerID != previewSceneLayerID {
		t.Fatalf("layer id = %d, want %d", att.LayerID, previewSceneLayerID)
	}
	if att.Placement.FreeAnchor != layout.FreeTopLeft {
		t.Fatalf("free anchor = %v, want top-left", att.Placement.FreeAnchor)
	}
}

func TestPreviewFacet_SyncSceneAttachesDirectRoots(t *testing.T) {
	resetCatalogStores(t)
	catalogstore.SelectEntry("annotation.connector")

	p := NewPreviewFacet(theme.Default(), nil)
	rt := &previewRuntimeStub{}
	p.runtime = rt
	p.syncScene()

	if len(rt.added) != 4 {
		t.Fatalf("added attachments = %d, want 4 direct roots", len(rt.added))
	}
	for i, att := range rt.added {
		if att.LayerID != previewSceneLayerID {
			t.Fatalf("attachment %d layer id = %d, want %d", i, att.LayerID, previewSceneLayerID)
		}
		if att.Placement.FreeAnchor != layout.FreeTopLeft {
			t.Fatalf("attachment %d free anchor = %v, want top-left", i, att.Placement.FreeAnchor)
		}
	}
}
