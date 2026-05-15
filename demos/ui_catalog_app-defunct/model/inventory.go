package model

import "sort"

// BuildMetadata holds build and backend metadata for the catalog.
type BuildMetadata struct {
	Version     string
	Commit      string
	BuildTime   string
	GoVersion   string
	Backend     string
	ThemeEngine string
}

// DefaultBuildMetadata returns metadata for development builds.
func DefaultBuildMetadata() BuildMetadata {
	return BuildMetadata{
		Version:     "0.1.0-dev",
		Commit:      "unknown",
		BuildTime:   "unknown",
		GoVersion:   "unknown",
		Backend:     "software",
		ThemeEngine: "legacy",
	}
}

func canonicalVariants(items ...Variant) []Variant {
	if len(items) == 0 {
		return nil
	}
	out := make([]Variant, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func canonicalStates(items ...State) []State {
	if len(items) == 0 {
		return nil
	}
	out := make([]State, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func canonicalStrings(items ...string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	sort.Strings(out)
	return out
}

func withInventoryMatrix(entry *CatalogEntry, variants []Variant, states []State, missingVariants, missingStates, unsupportedVariants, unsupportedStates []string) *CatalogEntry {
	if entry == nil {
		return nil
	}
	entry.Variants = variants
	entry.States = states
	entry.MissingVariants = missingVariants
	entry.MissingStates = missingStates
	entry.UnsupportedVariants = unsupportedVariants
	entry.UnsupportedStates = unsupportedStates
	return entry
}

// NewStandardCatalog creates the canonical inventory with all expected entries.
// The inventory is intentionally deterministic so exports and tests stay stable.
func NewStandardCatalog() *Catalog {
	c := NewCatalog()

	// Basic family
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.rect",
		DisplayName:       "Rectangle",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Fill and stroke variants with theme integration.",
	}, canonicalVariants(
		Variant{ID: "rect.filled", Label: "Filled", ThemeClass: "surface"},
		Variant{ID: "rect.outline", Label: "Outline", ThemeClass: "border"},
		Variant{ID: "rect.raised", Label: "Raised", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "hovered", Label: "Hovered"},
		State{ID: "selected", Label: "Selected"},
	), canonicalStrings("rect.rounded"), canonicalStrings("dragging"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.ellipse",
		DisplayName:       "Ellipse",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Circular and elliptical forms.",
	}, canonicalVariants(
		Variant{ID: "ellipse.circle", Label: "Circle", ThemeClass: "surface"},
		Variant{ID: "ellipse.oval", Label: "Oval", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "focused", Label: "Focused"},
	), canonicalStrings("ellipse.arc"), canonicalStrings("animation"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.polygon",
		DisplayName:       "Polygon",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Multi-point closed shapes.",
	}, canonicalVariants(
		Variant{ID: "polygon.triangle", Label: "Triangle", ThemeClass: "surface"},
		Variant{ID: "polygon.pentagon", Label: "Pentagon", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "selected", Label: "Selected"},
	), canonicalStrings("polygon.star"), canonicalStrings("edit-mode"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.polyline",
		DisplayName:       "Polyline",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Multi-point open paths.",
	}, canonicalVariants(
		Variant{ID: "polyline.smooth", Label: "Smooth", ThemeClass: "surface"},
		Variant{ID: "polyline.sharp", Label: "Sharp", ThemeClass: "border"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "hovered", Label: "Hovered"},
	), canonicalStrings("polyline.dashed"), canonicalStrings("editing"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.line",
		DisplayName:       "Line",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Single segment geometry.",
	}, canonicalVariants(
		Variant{ID: "line.solid", Label: "Solid", ThemeClass: "border"},
		Variant{ID: "line.dashed", Label: "Dashed", ThemeClass: "border"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "selected", Label: "Selected"},
	), canonicalStrings("line.wide"), canonicalStrings("snapping"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.path",
		DisplayName:       "Path",
		Family:            FamilyBasic,
		Subcategory:       "geometry",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Complex vector paths with fill rules.",
	}, canonicalVariants(
		Variant{ID: "path.open", Label: "Open", ThemeClass: "border"},
		Variant{ID: "path.closed", Label: "Closed", ThemeClass: "surface"},
		Variant{ID: "path.filled", Label: "Filled", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "editing", Label: "Editing"},
		State{ID: "selected", Label: "Selected"},
	), canonicalStrings("path.curved"), canonicalStrings("path.boolean"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.image",
		DisplayName:       "Image",
		Family:            FamilyBasic,
		Subcategory:       "media",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   false,
		Coverage:          CoverageThemeDependent,
		Notes:             "Bitmap display with fit modes and tint.",
	}, canonicalVariants(
		Variant{ID: "image.fit", Label: "Fit", ThemeClass: "surface"},
		Variant{ID: "image.fill", Label: "Fill", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "loaded", Label: "Loaded"},
		State{ID: "empty", Label: "Empty"},
	), canonicalStrings("image.tint"), canonicalStrings("loading"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "basic.text",
		DisplayName:       "Text",
		Family:            FamilyBasic,
		Subcategory:       "typography",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoverageLayoutDependent,
		Notes:             "Text rendering with baseline and alignment.",
	}, canonicalVariants(
		Variant{ID: "text.body", Label: "Body", ThemeClass: "surface"},
		Variant{ID: "text.caption", Label: "Caption", ThemeClass: "surface-variant"},
		Variant{ID: "text.heading", Label: "Heading", ThemeClass: "primary"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "selected", Label: "Selected"},
		State{ID: "disabled", Label: "Disabled"},
	), canonicalStrings("text.inline-code"), canonicalStrings("caret"), nil, nil))

	// Structure family
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "structure.group",
		DisplayName:       "Group",
		Family:            FamilyStructure,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoverageLayoutDependent,
		Notes:             "Content grouping without layout.",
	}, canonicalVariants(
		Variant{ID: "group.stack", Label: "Stack", ThemeClass: "surface"},
		Variant{ID: "group.flow", Label: "Flow", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "expanded", Label: "Expanded"},
		State{ID: "collapsed", Label: "Collapsed"},
	), canonicalStrings("group.overlay"), canonicalStrings("layout-pass"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "structure.clip",
		DisplayName:       "Clip",
		Family:            FamilyStructure,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Clipping boundaries for content.",
	}, canonicalVariants(
		Variant{ID: "clip.rectangle", Label: "Rectangle", ThemeClass: "border"},
		Variant{ID: "clip.rounded", Label: "Rounded", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "enabled", Label: "Enabled"},
		State{ID: "disabled", Label: "Disabled"},
	), canonicalStrings("clip.mask"), canonicalStrings("overflow"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "structure.transform",
		DisplayName:       "Transform",
		Family:            FamilyStructure,
		Subcategory:       "modifier",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Coordinate space transformations.",
	}, canonicalVariants(
		Variant{ID: "transform.translate", Label: "Translate", ThemeClass: "surface"},
		Variant{ID: "transform.scale", Label: "Scale", ThemeClass: "surface-variant"},
		Variant{ID: "transform.rotate", Label: "Rotate", ThemeClass: "primary"},
	), canonicalStates(
		State{ID: "idle", Label: "Idle"},
		State{ID: "animating", Label: "Animating"},
	), canonicalStrings("transform.matrix"), canonicalStrings("motion"), nil, nil))
	_ = c.AddEntry(withInventoryMatrix(&CatalogEntry{
		ID:                "structure.viewport",
		DisplayName:       "Viewport Host",
		Family:            FamilyStructure,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Scrollable viewport container.",
	}, canonicalVariants(
		Variant{ID: "viewport.scroll", Label: "Scroll", ThemeClass: "surface"},
		Variant{ID: "viewport.clip", Label: "Clip", ThemeClass: "surface-variant"},
	), canonicalStates(
		State{ID: "at-start", Label: "At Start"},
		State{ID: "at-end", Label: "At End"},
	), canonicalStrings("viewport.overflow"), canonicalStrings("scroll-position"), nil, nil))
	_ = c.AddEntry(&CatalogEntry{
		ID:                "structure.anchor",
		DisplayName:       "Anchor Proxy",
		Family:            FamilyStructure,
		Subcategory:       "reference",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Attachment point export and reference.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "structure.layer",
		DisplayName:       "Layer Mount",
		Family:            FamilyStructure,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Hosted content layer mounting.",
	})

	// Annotation family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.label",
		DisplayName:       "Label",
		Family:            FamilyAnnotation,
		Subcategory:       "text",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Annotated text with normal and compact forms.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.connector",
		DisplayName:       "Connector",
		Family:            FamilyAnnotation,
		Subcategory:       "link",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Connecting lines with routing modes.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.callout",
		DisplayName:       "Callout",
		Family:            FamilyAnnotation,
		Subcategory:       "overlay",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Directional annotation overlays.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.handle",
		DisplayName:       "Handle",
		Family:            FamilyAnnotation,
		Subcategory:       "control",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoverageThemeDependent,
		Notes:             "Interactive control handles with hit areas.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.symbol",
		DisplayName:       "Symbol",
		Family:            FamilyAnnotation,
		Subcategory:       "glyph",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Reusable symbol instances.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.icon",
		DisplayName:       "Icon",
		Family:            FamilyAnnotation,
		Subcategory:       "glyph",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Themed icon graphics.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.badge",
		DisplayName:       "Badge",
		Family:            FamilyAnnotation,
		Subcategory:       "indicator",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Status and count indicators.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.rule",
		DisplayName:       "Rule",
		Family:            FamilyAnnotation,
		Subcategory:       "divider",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Dividing lines and rules.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "annotation.area",
		DisplayName:       "Area",
		Family:            FamilyAnnotation,
		Subcategory:       "region",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoverageThemeDependent,
		Notes:             "Highlight and selection areas.",
	})

	// UI Input family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.button",
		DisplayName:       "Button",
		Family:            FamilyUIInput,
		Subcategory:       "action",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoverageLayoutDependent,
		Notes:             "Pressable action buttons with all interaction states.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.checkbox",
		DisplayName:       "Checkbox",
		Family:            FamilyUIInput,
		Subcategory:       "toggle",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoverageThemeDependent,
		Notes:             "Binary toggle with checked/unchecked states.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.switch",
		DisplayName:       "Switch",
		Family:            FamilyUIInput,
		Subcategory:       "toggle",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoverageLayoutDependent,
		Notes:             "Sliding toggle switch.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.slider",
		DisplayName:       "Slider",
		Family:            FamilyUIInput,
		Subcategory:       "range",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Value selection with drag state.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.select",
		DisplayName:       "Select",
		Family:            FamilyUIInput,
		Subcategory:       "choice",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Dropdown selection with open/closed states.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.textinput",
		DisplayName:       "Text Input",
		Family:            FamilyUIInput,
		Subcategory:       "text",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Editable text with empty, filled, placeholder, and selection states.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uiinput.radiogroup",
		DisplayName:       "Radio Group",
		Family:            FamilyUIInput,
		Subcategory:       "choice",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Mutually exclusive option selection.",
	})

	// UI Navigation family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.tabs",
		DisplayName:       "Tabs",
		Family:            FamilyUINav,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Tab navigation with standard and compact variants.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.breadcrumbs",
		DisplayName:       "Breadcrumbs",
		Family:            FamilyUINav,
		Subcategory:       "path",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Hierarchical path with overflow behavior.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.drawer",
		DisplayName:       "Drawer",
		Family:            FamilyUINav,
		Subcategory:       "overlay",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Side panel with mode and edge variations.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.menu",
		DisplayName:       "Menu",
		Family:            FamilyUINav,
		Subcategory:       "overlay",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Contextual menu with density variations.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.pagination",
		DisplayName:       "Pagination",
		Family:            FamilyUINav,
		Subcategory:       "control",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Page navigation with current and overflow states.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.scrollbar",
		DisplayName:       "Scrollbar",
		Family:            FamilyUINav,
		Subcategory:       "control",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Scroll indicator with orientation variants.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinav.speeddial",
		DisplayName:       "Speed Dial",
		Family:            FamilyUINav,
		Subcategory:       "action",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Floating action button with expanded/collapsed states.",
	})

	// UI Notification family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinotification.dialog",
		DisplayName:       "Dialog",
		Family:            FamilyUINotification,
		Subcategory:       "modal",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Modal dialog with variants.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinotification.snackbar",
		DisplayName:       "Snackbar",
		Family:            FamilyUINotification,
		Subcategory:       "transient",
		ConstructionClass: ConstructionComposed,
		Interactive:       true,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Transient notification with variants.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "uinotification.progress",
		DisplayName:       "Progress",
		Family:            FamilyUINotification,
		Subcategory:       "indicator",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Progress indicators with variants.",
	})

	// Chart family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "chart.axis",
		DisplayName:       "Axis",
		Family:            FamilyChart,
		Subcategory:       "scale",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Chart axis with orientation and density variants.",
	})

	setMatrix := func(id string, variants []Variant, states []State, missingVariants, missingStates, unsupportedVariants, unsupportedStates []string) {
		if entry, ok := c.GetEntry(id); ok {
			entry.Variants = variants
			entry.States = states
			entry.MissingVariants = missingVariants
			entry.MissingStates = missingStates
			entry.UnsupportedVariants = unsupportedVariants
			entry.UnsupportedStates = unsupportedStates
		}
	}

	setMatrix("annotation.label",
		canonicalVariants(
			Variant{ID: "label.inline", Label: "Inline", ThemeClass: "surface"},
			Variant{ID: "label.compact", Label: "Compact", ThemeClass: "surface-variant"},
		),
		canonicalStates(
			State{ID: "idle", Label: "Idle"},
			State{ID: "selected", Label: "Selected"},
			State{ID: "muted", Label: "Muted"},
		),
		canonicalStrings("label.emphasis"),
		canonicalStrings("caret"),
		nil,
		nil,
	)
	setMatrix("annotation.icon",
		canonicalVariants(
			Variant{ID: "icon.filled", Label: "Filled", ThemeClass: "surface"},
			Variant{ID: "icon.outline", Label: "Outline", ThemeClass: "border"},
		),
		canonicalStates(
			State{ID: "idle", Label: "Idle"},
			State{ID: "hovered", Label: "Hovered"},
		),
		canonicalStrings("icon.monochrome"),
		canonicalStrings("badge"),
		nil,
		nil,
	)
	setMatrix("uiinput.button",
		canonicalVariants(
			Variant{ID: "button.solid", Label: "Solid", ThemeClass: "primary"},
			Variant{ID: "button.outline", Label: "Outline", ThemeClass: "border"},
			Variant{ID: "button.text", Label: "Text", ThemeClass: "surface"},
		),
		canonicalStates(
			State{ID: "idle", Label: "Idle"},
			State{ID: "hovered", Label: "Hovered"},
			State{ID: "pressed", Label: "Pressed"},
			State{ID: "disabled", Label: "Disabled"},
			State{ID: "focused", Label: "Focused"},
		),
		canonicalStrings("button.icon-only", "button.split"),
		canonicalStrings("loading"),
		nil,
		nil,
	)
	setMatrix("uiinput.checkbox",
		canonicalVariants(
			Variant{ID: "checkbox.checked", Label: "Checked", ThemeClass: "primary"},
			Variant{ID: "checkbox.indeterminate", Label: "Indeterminate", ThemeClass: "surface-variant"},
		),
		canonicalStates(
			State{ID: "unchecked", Label: "Unchecked"},
			State{ID: "checked", Label: "Checked"},
			State{ID: "disabled", Label: "Disabled"},
		),
		canonicalStrings("checkbox.radio"),
		canonicalStrings("tri-state"),
		nil,
		nil,
	)
	setMatrix("uinav.tabs",
		canonicalVariants(
			Variant{ID: "tabs.top", Label: "Top", ThemeClass: "primary"},
			Variant{ID: "tabs.sidebar", Label: "Sidebar", ThemeClass: "surface"},
			Variant{ID: "tabs.compact", Label: "Compact", ThemeClass: "surface-variant"},
		),
		canonicalStates(
			State{ID: "active", Label: "Active"},
			State{ID: "inactive", Label: "Inactive"},
			State{ID: "overflow", Label: "Overflow"},
			State{ID: "disabled", Label: "Disabled"},
		),
		canonicalStrings("tabs.underline"),
		canonicalStrings("overflow"),
		nil,
		nil,
	)
	setMatrix("uinotification.dialog",
		canonicalVariants(
			Variant{ID: "dialog.modal", Label: "Modal", ThemeClass: "surface"},
			Variant{ID: "dialog.sheet", Label: "Sheet", ThemeClass: "surface-variant"},
			Variant{ID: "dialog.alert", Label: "Alert", ThemeClass: "primary"},
		),
		canonicalStates(
			State{ID: "open", Label: "Open"},
			State{ID: "dismissed", Label: "Dismissed"},
			State{ID: "stacked", Label: "Stacked"},
		),
		canonicalStrings("dialog.fullscreen"),
		canonicalStrings("motion"),
		nil,
		nil,
	)
	setMatrix("chart.axis",
		canonicalVariants(
			Variant{ID: "axis.horizontal", Label: "Horizontal", ThemeClass: "surface"},
			Variant{ID: "axis.vertical", Label: "Vertical", ThemeClass: "surface-variant"},
		),
		canonicalStates(
			State{ID: "idle", Label: "Idle"},
			State{ID: "zoomed", Label: "Zoomed"},
		),
		canonicalStrings("axis.log", "axis.reversed"),
		canonicalStrings("pan", "zoom"),
		nil,
		nil,
	)

	return c
}
