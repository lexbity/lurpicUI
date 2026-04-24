package shell

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	engdiag "codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	diag "codeburg.org/lexbit/ui_diagnostic_scene/diagnostics"
	bundle "codeburg.org/lexbit/ui_diagnostic_scene/export"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// RootLogEntry represents a single log line at the root level
type RootLogEntry struct {
	Ordinal  int
	Category string
	Message  string
	Time     string
}

// RootFacet is the main shell facet containing all UI panels
type RootFacet struct {
	facet.Facet

	// Core dependencies
	baseTheme   theme.Context
	activeTheme theme.Context
	shaper      *text.Shaper
	registry    *scene.Registry
	adapter     *diag.Adapter

	themeMode    ThemeMode
	densityMode  DensityMode
	densityScale float32
	backend      string
	platform     string
	buildInfo    string
	stressMode   bool

	// State
	selectedSceneID store.Binding[string]
	logs            []RootLogEntry
	logOrdinal      int
	maxLogs         int

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
		baseTheme:       th,
		activeTheme:     newShellThemeContext(th, ThemeModeDefault),
		shaper:          shaper,
		registry:        registry,
		adapter:         diag.NewAdapter(),
		themeMode:       ThemeModeDefault,
		densityMode:     DensityModeNormal,
		densityScale:    DensityModeNormal.Scale(),
		backend:         "software",
		platform:        runtime.GOOS + "/" + runtime.GOARCH,
		buildInfo:       readBuildInfoSummary(),
		selectedSceneID: store.NewBinding(""),
		maxLogs:         200,
	}

	r.buildUI()
	r.setupSignals()
	r.syncShellTheme()
	r.syncDiagnostics()
	return r
}

func (r *RootFacet) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

func (r *RootFacet) OnAttach(ctx facet.AttachContext) {
	if r.topBar != nil {
		r.Base().AddChildRuntime(r.topBar.Base())
	}
	if r.contentSplit != nil {
		r.Base().AddChildRuntime(r.contentSplit.Base())
	}
	if r.bottomPanel != nil {
		r.Base().AddChildRuntime(r.bottomPanel.Base())
	}
	r.syncShellTheme()
	r.syncDiagnostics()
}

func (r *RootFacet) OnDetach()     {}
func (r *RootFacet) OnDeactivate() {}

func (r *RootFacet) OnActivate() {
	if r.leftNav != nil && r.registry != nil && r.leftNav.SelectedScene() == "" {
		defs := r.registry.GetAll()
		if len(defs) > 0 {
			r.leftNav.SelectScene(defs[0].ID)
		}
	}
}

func (r *RootFacet) buildUI() {
	// Create layout structure:
	// Column:
	//   - TopBar (fixed height)
	//   - ContentSplit (flex)
	//   - BottomPanel (fixed height for logs)

	// Top metadata bar
	r.topBar = NewTopBarFacet(r.activeTheme, r.shaper)

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
	r.leftNav = NewSceneNavFacet(r.activeTheme, r.shaper, r.registry)

	// Scene host (where scenes render)
	r.sceneHost = NewSceneHostFacet(r.activeTheme, r.shaper)

	leftSplit.SetFirst(r.leftNav)
	leftSplit.SetSecond(r.sceneHost)

	// Right side: diagnostics panel
	r.rightPanel = NewDiagnosticsPanelFacet(r.activeTheme, r.shaper, r.adapter)

	r.contentSplit.SetFirst(leftSplit)
	r.contentSplit.SetSecond(r.rightPanel)

	// Bottom logs panel
	r.bottomPanel = NewLogsPanelFacet(r.activeTheme, r.shaper)

	// Wire up scene selection
	r.leftNav.OnSceneSelected.Subscribe(func(id string) {
		r.selectedSceneID.Set(id)
		r.OnSceneSelected.Emit(id)
	})
}

func (r *RootFacet) setupSignals() {
	r.OnSceneSelected.Subscribe(func(id string) {
		r.sceneHost.LoadScene(id, r.registry)
		r.syncDiagnostics()
		r.log("scene load", "Selected scene: "+id)
	})

	r.OnReset.Subscribe(func(_ struct{}) {
		r.sceneHost.Reset()
		r.syncDiagnostics()
		r.log("scene reset", "Scene reset")
	})

	if r.topBar != nil {
		r.topBar.OnReset.Subscribe(func(_ struct{}) {
			r.OnReset.Emit(struct{}{})
		})
		r.topBar.OnThemeNext.Subscribe(func(_ struct{}) {
			r.nextTheme()
		})
		r.topBar.OnDensityNext.Subscribe(func(_ struct{}) {
			r.nextDensity()
		})
		r.topBar.OnToggleBounds.Subscribe(func(_ struct{}) {
			r.toggleBounds()
		})
		r.topBar.OnToggleHit.Subscribe(func(_ struct{}) {
			r.toggleHitRegions()
		})
		r.topBar.OnToggleFocus.Subscribe(func(_ struct{}) {
			r.toggleFocus()
		})
		r.topBar.OnToggleStress.Subscribe(func(_ struct{}) {
			r.toggleStress()
		})
	}
}

func (r *RootFacet) log(category, message string) {
	r.logOrdinal++
	entry := RootLogEntry{
		Ordinal:  r.logOrdinal,
		Category: category,
		Message:  message,
		Time:     "", // Will be populated by panel
	}
	r.logs = append(r.logs, entry)
	if r.maxLogs > 0 && len(r.logs) > r.maxLogs {
		r.logs = r.logs[len(r.logs)-r.maxLogs:]
	}
	// Convert to shell.LogEntry for the panel
	panelEntry := LogEntry{
		Ordinal:  entry.Ordinal,
		Category: entry.Category,
		Message:  entry.Message,
		Time:     entry.Time,
	}
	r.bottomPanel.AppendLog(panelEntry)
}

func (r *RootFacet) syncDiagnostics() {
	if r == nil || r.adapter == nil {
		return
	}
	if r.sceneHost != nil {
		r.adapter.SetInspector(engdiag.NewInspector(r.sceneHost.MountedRoot()))
	}
	r.adapter.SetSceneSummary(r.sceneSummary())
	if r.rightPanel != nil {
		r.rightPanel.SetOverlayEnabled(r.rightPanel.overlayEnabled)
		r.rightPanel.SetShowBounds(r.rightPanel.showBounds)
		r.rightPanel.SetShowHitRegions(r.rightPanel.showHitRegions)
		r.rightPanel.SetShowFocus(r.rightPanel.showFocus)
	}
}

func (r *RootFacet) syncShellTheme() {
	if r == nil {
		return
	}
	if r.activeTheme == nil {
		r.activeTheme = r.baseTheme
	}
	if r.topBar != nil {
		r.topBar.theme = r.activeTheme
		r.topBar.SetStatus(r.themeMode.String(), r.densityMode.String(), r.backend, r.platform, r.buildInfo)
	}
	if r.leftNav != nil {
		r.leftNav.theme = r.activeTheme
		r.leftNav.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	if r.sceneHost != nil {
		r.sceneHost.theme = r.activeTheme
		r.sceneHost.ApplyTheme(r.activeTheme)
		r.sceneHost.ApplyDensity(r.densityScale)
	}
	if r.rightPanel != nil {
		r.rightPanel.theme = r.activeTheme
		r.rightPanel.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	if r.bottomPanel != nil {
		r.bottomPanel.theme = r.activeTheme
		r.bottomPanel.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (r *RootFacet) nextTheme() {
	if r == nil {
		return
	}
	r.themeMode = nextThemeMode(r.themeMode)
	r.activeTheme = newShellThemeContext(r.baseTheme, r.themeMode)
	r.syncShellTheme()
	r.syncDiagnostics()
	r.log("state mutation", "Theme cycled to "+r.themeMode.String())
}

func (r *RootFacet) nextDensity() {
	if r == nil {
		return
	}
	r.densityMode = nextDensityMode(r.densityMode)
	r.densityScale = r.densityMode.Scale()
	r.syncShellTheme()
	r.syncDiagnostics()
	r.log("state mutation", "Density set to "+r.densityMode.String())
}

func (r *RootFacet) toggleBounds() {
	if r == nil || r.rightPanel == nil {
		return
	}
	r.rightPanel.SetShowBounds(!r.rightPanel.showBounds)
	r.syncDiagnostics()
	r.log("diagnostics toggle", fmt.Sprintf("Bounds overlay %t", r.rightPanel.showBounds))
}

func (r *RootFacet) toggleHitRegions() {
	if r == nil || r.rightPanel == nil {
		return
	}
	r.rightPanel.SetShowHitRegions(!r.rightPanel.showHitRegions)
	r.syncDiagnostics()
	r.log("diagnostics toggle", fmt.Sprintf("Hit regions overlay %t", r.rightPanel.showHitRegions))
}

func (r *RootFacet) toggleFocus() {
	if r == nil || r.rightPanel == nil {
		return
	}
	r.rightPanel.SetShowFocus(!r.rightPanel.showFocus)
	r.syncDiagnostics()
	r.log("diagnostics toggle", fmt.Sprintf("Focus overlay %t", r.rightPanel.showFocus))
}

func (r *RootFacet) toggleStress() {
	if r == nil || r.sceneHost == nil {
		return
	}
	scene := r.sceneHost.CurrentScene()
	if scene == nil {
		return
	}
	toggler, ok := scene.(interface{ SetStressMode(bool) })
	if !ok {
		r.log("state mutation", "Stress mode unavailable for "+r.sceneHost.CurrentSceneID())
		return
	}
	r.stressMode = !r.stressMode
	toggler.SetStressMode(r.stressMode)
	r.sceneHost.RebuildCurrentScene()
	r.syncDiagnostics()
	r.log("state mutation", fmt.Sprintf("Stress mode %t", r.stressMode))
}

func (r *RootFacet) sceneSummary() diag.SceneCapabilitySummary {
	if r == nil || r.sceneHost == nil || r.registry == nil {
		return diag.SceneCapabilitySummary{}
	}
	id := r.sceneHost.CurrentSceneID()
	if id == "" {
		return diag.SceneCapabilitySummary{}
	}
	def, ok := r.registry.Get(id)
	if !ok {
		return diag.SceneCapabilitySummary{SceneID: id}
	}
	caps := scene.CapabilitySet{}
	if sc := r.sceneHost.CurrentScene(); sc != nil {
		caps = sc.Capabilities()
	}
	return diag.SceneCapabilitySummary{
		SceneID:             def.ID,
		SceneName:           def.DisplayName,
		HasStressControls:   caps.HasStressControls,
		SupportsScreenshot:  caps.SupportsScreenshot,
		SupportsSnapshot:    caps.SupportsSnapshot,
		SupportsThemeSwitch: caps.SupportsThemeSwitch,
		SupportsDensity:     caps.SupportsDensity,
		HasCustomLogs:       caps.HasCustomLogs,
		Families:            append([]string(nil), def.Families...),
		Description:         def.Description,
	}
}

// SelectedSceneID returns the currently selected scene binding
func (r *RootFacet) SelectedSceneID() store.Binding[string] {
	return r.selectedSceneID
}

// Registry returns the scene registry
func (r *RootFacet) Registry() *scene.Registry {
	return r.registry
}

// ExportBundle assembles a deterministic bug-report bundle for the active app state.
func (r *RootFacet) ExportBundle(runID string) bundle.Bundle {
	if runID == "" {
		runID = "manual"
	}
	manifest := bundle.Manifest{
		RunID:     runID,
		SceneID:   "",
		SceneName: "",
		Theme:     r.themeMode.String(),
		Density:   r.densityMode.String(),
		Backend:   r.backend,
		Platform:  r.platform,
		BuildInfo: r.buildInfo,
	}
	sceneSnapshot := bundle.SceneSnapshot{}
	if summary := r.sceneSummary(); summary.SceneID != "" || summary.SceneName != "" {
		manifest.SceneID = summary.SceneID
		manifest.SceneName = summary.SceneName
		sceneSnapshot = bundle.SceneSnapshot{
			SceneID:     summary.SceneID,
			SceneName:   summary.SceneName,
			Description: summary.Description,
			Families:    append([]string(nil), summary.Families...),
			Capabilities: map[string]any{
				"stressControls":      summary.HasStressControls,
				"supportsScreenshot":  summary.SupportsScreenshot,
				"supportsSnapshot":    summary.SupportsSnapshot,
				"supportsThemeSwitch": summary.SupportsThemeSwitch,
				"supportsDensity":     summary.SupportsDensity,
				"hasCustomLogs":       summary.HasCustomLogs,
			},
		}
	}
	if r.sceneHost != nil {
		hostSnapshot := r.sceneHost.SnapshotState()
		if sceneSnapshot.SceneID == "" {
			sceneSnapshot.SceneID = hostSnapshot.SceneID
			sceneSnapshot.SceneName = hostSnapshot.SceneName
		}
		if sceneSnapshot.State == nil {
			sceneSnapshot.State = hostSnapshot.State
		}
		if current := r.sceneHost.CurrentScene(); current != nil {
			if logger, ok := current.(interface{ GetLogs() []string }); ok {
				sceneSnapshot.Logs = append([]string(nil), logger.GetLogs()...)
			}
		}
	}

	diagSnapshot := bundle.DiagnosticsSnapshot{}
	if r.adapter != nil {
		snap := r.adapter.Snapshot()
		diagSnapshot = bundle.DiagnosticsSnapshot{
			Scene:        snap.Scene,
			Overlays:     snap.Overlays,
			Focus:        snap.Focus,
			Hit:          snap.Hit,
			Invalidation: snap.Invalidation,
			Anchor:       snap.Anchor,
			Render:       snap.Render,
			Frames:       append([]diag.FrameStatsView(nil), snap.Frames...),
		}
	}

	return bundle.Build(bundle.Input{
		Manifest:    manifest,
		Scene:       sceneSnapshot,
		Diagnostics: diagSnapshot,
		Logs:        r.bundleLogs(),
	})
}

func (r *RootFacet) bundleLogs() []bundle.LogEntry {
	if len(r.logs) == 0 {
		return nil
	}
	out := make([]bundle.LogEntry, 0, len(r.logs))
	for _, entry := range r.logs {
		out = append(out, bundle.LogEntry{
			Ordinal:  entry.Ordinal,
			Category: normalizeLogCategory(entry.Category),
			Message:  entry.Message,
			Time:     entry.Time,
		})
	}
	return out
}

func normalizeLogCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "scene load", "scene reset", "state mutation", "diagnostics toggle":
		return strings.ToLower(strings.TrimSpace(category))
	case "scene":
		return "scene load"
	case "overlay":
		return "diagnostics toggle"
	case "system":
		return "state mutation"
	default:
		return strings.ToLower(strings.TrimSpace(category))
	}
}

func readBuildInfoSummary() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	if info.Main.Path == "" {
		return version
	}
	return info.Main.Path + "@" + version
}
