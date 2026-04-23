package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// RootLogEntry represents a single log line at the root level
type RootLogEntry struct {
	Category string
	Message  string
	Time     string
}

// RootFacet is the main shell facet containing all UI panels
type RootFacet struct {
	facet.Facet

	// Core dependencies
	theme    theme.Context
	shaper   *text.Shaper
	registry *scene.Registry

	// State
	selectedSceneID store.Binding[string]
	logs            []RootLogEntry

	// Layout
	rootLayout   *layout.ColumnLayout
	topBar       *TopBarFacet
	contentSplit *layout.SplitLayout
	leftNav      *SceneNavFacet
	sceneHost    *SceneHostFacet
	rightPanel   *DiagnosticsPanelFacet
	bottomPanel  *LogsPanelFacet

	// Signals
	OnSceneSelected signal.Signal[string]
	OnReset         signal.Signal[struct{}]
}

// NewRootFacet constructs the diagnostic scene shell
func NewRootFacet(th theme.Context, shaper *text.Shaper, registry *scene.Registry) *RootFacet {
	r := &RootFacet{
		Facet:           facet.NewFacet(),
		theme:           th,
		shaper:          shaper,
		registry:        registry,
		selectedSceneID: store.NewBinding(""),
	}

	r.buildUI()
	r.setupSignals()
	return r
}

func (r *RootFacet) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

func (r *RootFacet) OnAttach(ctx facet.AttachContext) {}
func (r *RootFacet) OnDetach()                        {}
func (r *RootFacet) OnActivate()                      {}
func (r *RootFacet) OnDeactivate()                    {}

func (r *RootFacet) buildUI() {
	// Create layout structure:
	// Column:
	//   - TopBar (fixed height)
	//   - ContentSplit (flex)
	//   - BottomPanel (fixed height for logs)

	// Top metadata bar
	r.topBar = NewTopBarFacet(r.theme, r.shaper)

	// Middle content area - horizontal split for left nav + scene + right diagnostics
	r.contentSplit = layout.NewSplitLayout(layout.SplitHorizontal, 0.15)
	r.contentSplit.DividerWidth = 4
	r.contentSplit.MinFirstSize = 150
	r.contentSplit.MinSecondSize = 400

	// Left side: nested split for scene nav + scene host
	leftSplit := layout.NewSplitLayout(layout.SplitHorizontal, 0.35)
	leftSplit.DividerWidth = 4
	leftSplit.MinFirstSize = 100
	leftSplit.MinSecondSize = 200

	// Scene navigation
	r.leftNav = NewSceneNavFacet(r.theme, r.shaper, r.registry)

	// Scene host (where scenes render)
	r.sceneHost = NewSceneHostFacet(r.theme, r.shaper)

	leftSplit.SetFirst(r.leftNav)
	leftSplit.SetSecond(r.sceneHost)

	// Right side: diagnostics panel
	r.rightPanel = NewDiagnosticsPanelFacet(r.theme, r.shaper)

	r.contentSplit.SetFirst(leftSplit)
	r.contentSplit.SetSecond(r.rightPanel)

	// Bottom logs panel
	r.bottomPanel = NewLogsPanelFacet(r.theme, r.shaper)

	// Wire up scene selection
	r.leftNav.OnSceneSelected.Subscribe(func(id string) {
		r.selectedSceneID.Set(id)
		r.OnSceneSelected.Emit(id)
	})
}

func (r *RootFacet) setupSignals() {
	r.OnSceneSelected.Subscribe(func(id string) {
		r.sceneHost.LoadScene(id, r.registry)
		r.log("scene", "Selected scene: "+id)
	})

	r.OnReset.Subscribe(func(_ struct{}) {
		r.sceneHost.Reset()
		r.log("system", "Scene reset")
	})
}

func (r *RootFacet) log(category, message string) {
	entry := RootLogEntry{
		Category: category,
		Message:  message,
		Time:     "", // Will be populated by panel
	}
	r.logs = append(r.logs, entry)
	// Convert to shell.LogEntry for the panel
	panelEntry := LogEntry{
		Category: entry.Category,
		Message:  entry.Message,
		Time:     entry.Time,
	}
	r.bottomPanel.AppendLog(panelEntry)
}

// SelectedSceneID returns the currently selected scene binding
func (r *RootFacet) SelectedSceneID() store.Binding[string] {
	return r.selectedSceneID
}

// Registry returns the scene registry
func (r *RootFacet) Registry() *scene.Registry {
	return r.registry
}
