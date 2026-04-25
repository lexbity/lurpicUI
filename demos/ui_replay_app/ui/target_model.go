package ui

import "codeburg.org/lexbit/lurpicui/marks/structure"

const replayAppID = "ui_replay_app"

// TargetModel describes the replay app shell surfaces for desktop and mobile.
func TargetModel() structure.TargetModel {
	return structure.TargetModel{
		AppID:   replayAppID,
		AppName: "UI Replay",
		Surfaces: []structure.SurfaceSpec{
			{
				ID:    "header",
				Label: "Header",
				Role:  structure.SurfacePrimary,
				Notes: "Scenario identity and execution state stay visible across targets.",
			},
			{
				ID:    "sidebar",
				Label: "Sidebar",
				Role:  structure.SurfaceSecondary,
				Notes: "Scenario selection and run history collapse on narrow mobile layouts.",
			},
			{
				ID:    "content",
				Label: "Content",
				Role:  structure.SurfacePrimary,
				Notes: "Scenario playback and step visualization are the main interaction surface.",
			},
			{
				ID:    "inspector",
				Label: "Inspector",
				Role:  structure.SurfaceSecondary,
				Notes: "Diagnostics summary and state inspection move to a secondary surface on mobile.",
			},
			{
				ID:    "footer",
				Label: "Footer",
				Role:  structure.SurfaceOptional,
				Notes: "Execution hints and shortcuts are helpful but not required for the core workflow.",
			},
		},
	}
}

// TargetProfile classifies the app shell for the current viewport and platform capabilities.
func TargetProfile(v structure.Viewport, caps structure.Capabilities) structure.Profile {
	return structure.ProfileForViewport(v, caps)
}
