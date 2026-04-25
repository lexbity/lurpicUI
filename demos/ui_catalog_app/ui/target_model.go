package ui

import "codeburg.org/lexbit/lurpicui/marks/structure"

const catalogAppID = "ui_catalog_app"

// TargetModel describes the catalog app shell surfaces for desktop and mobile.
func TargetModel() structure.TargetModel {
	return structure.TargetModel{
		AppID:   catalogAppID,
		AppName: "UI Catalog",
		Surfaces: []structure.SurfaceSpec{
			{
				ID:    "header",
				Label: "Header",
				Role:  structure.SurfacePrimary,
				Notes: "Top-level app controls and density/theme selection stay visible on all targets.",
			},
			{
				ID:    "sidebar",
				Label: "Sidebar",
				Role:  structure.SurfaceSecondary,
				Notes: "Family navigation and filters collapse on narrow mobile layouts.",
			},
			{
				ID:    "content",
				Label: "Content",
				Role:  structure.SurfacePrimary,
				Notes: "Catalog browsing and grid/detail rendering remain the main interaction surface.",
			},
			{
				ID:    "inspector",
				Label: "Inspector",
				Role:  structure.SurfaceSecondary,
				Notes: "Detailed metadata and matrix summaries become a secondary sheet on mobile.",
			},
			{
				ID:    "footer",
				Label: "Footer",
				Role:  structure.SurfaceOptional,
				Notes: "Status and compare hints are useful but not required for core interaction.",
			},
		},
	}
}

// TargetProfile classifies the app shell for the current viewport and platform capabilities.
func TargetProfile(v structure.Viewport, caps structure.Capabilities) structure.Profile {
	return structure.ProfileForViewport(v, caps)
}
