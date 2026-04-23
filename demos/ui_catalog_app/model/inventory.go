package model

// BuildMetadata holds build and backend metadata for the catalog.
type BuildMetadata struct {
	Version      string
	Commit       string
	BuildTime    string
	GoVersion    string
	Backend      string
	ThemeEngine  string
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

// NewStandardCatalog creates the canonical inventory with all expected entries.
// In Phase 1, entries are created with placeholder coverage status.
func NewStandardCatalog() *Catalog {
	c := NewCatalog()
	
	// Basic family
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "basic.image",
		DisplayName:       "Image",
		Family:            FamilyBasic,
		Subcategory:       "media",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   false,
		Coverage:          CoveragePlaceholder,
		Notes:             "Bitmap display with fit modes and tint.",
	})
	_ = c.AddEntry(&CatalogEntry{
		ID:                "basic.text",
		DisplayName:       "Text",
		Family:            FamilyBasic,
		Subcategory:       "typography",
		ConstructionClass: ConstructionPrimitive,
		Interactive:       false,
		ThemeSensitive:    true,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Text rendering with baseline and alignment.",
	})
	
	// Structure family
	_ = c.AddEntry(&CatalogEntry{
		ID:                "structure.group",
		DisplayName:       "Group",
		Family:            FamilyStructure,
		Subcategory:       "container",
		ConstructionClass: ConstructionComposed,
		Interactive:       false,
		ThemeSensitive:    false,
		LayoutSensitive:   true,
		Coverage:          CoveragePlaceholder,
		Notes:             "Content grouping without layout.",
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
	_ = c.AddEntry(&CatalogEntry{
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
	})
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
		Coverage:          CoveragePlaceholder,
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
		Coverage:          CoveragePlaceholder,
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
		Coverage:          CoveragePlaceholder,
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
		Coverage:          CoveragePlaceholder,
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
		Coverage:          CoveragePlaceholder,
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
	
	return c
}
