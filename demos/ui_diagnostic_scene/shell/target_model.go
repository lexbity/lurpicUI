package shell

import "codeburg.org/lexbit/lurpicui/marks/structure"

const diagnosticSceneAppID = "ui_diagnostic_scene"

// TargetModel describes the diagnostic scene shell surfaces for desktop and mobile.
func TargetModel() structure.TargetModel {
	return structure.TargetModel{
		AppID:   diagnosticSceneAppID,
		AppName: "UI Diagnostic Scene",
		Surfaces: []structure.SurfaceSpec{
			{
				ID:    "topbar",
				Label: "Top Bar",
				Role:  structure.SurfacePrimary,
				Notes: "Metadata and command controls remain accessible across targets.",
			},
			{
				ID:    "scene-nav",
				Label: "Scene Navigation",
				Role:  structure.SurfaceSecondary,
				Notes: "Scene selection collapses into touch-friendly navigation on mobile.",
			},
			{
				ID:    "scene-host",
				Label: "Scene Host",
				Role:  structure.SurfacePrimary,
				Notes: "The active scene preview is the main interactive surface.",
			},
			{
				ID:    "diagnostics",
				Label: "Diagnostics",
				Role:  structure.SurfaceSecondary,
				Notes: "Inspector and overlays become a secondary panel or drawer on mobile.",
			},
			{
				ID:    "logs",
				Label: "Logs",
				Role:  structure.SurfaceOptional,
				Notes: "Event history is useful for debugging but can be collapsed on narrow layouts.",
			},
		},
	}
}

// TargetProfile classifies the shell for the current viewport and platform capabilities.
func TargetProfile(v structure.Viewport, caps structure.Capabilities) structure.Profile {
	return structure.ProfileForViewport(v, caps)
}
