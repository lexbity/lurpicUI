package templates

import "codeburg.org/lexbit/lurpicui/gfx"

// PreviewCard captures one theme-density slice for preview/demo screens.
type PreviewCard struct {
	ThemeName    string
	Density      DensityMode
	Control      gfx.Size
	Input        gfx.Size
	Navigation   gfx.Size
	Notification gfx.Size
	Chart        gfx.Size
	Notes        string
}

// PreviewMatrix returns the canonical theme/density preview data.
func PreviewMatrix(themes ...TemplateTheme) []PreviewCard {
	if len(themes) == 0 {
		themes = []TemplateTheme{UneNuit(), Sythique(), Notes()}
	}
	cards := make([]PreviewCard, 0, len(themes)*3)
	for _, theme := range themes {
		for _, density := range []DensityMode{DensityCompact, DensityRegular, DensityTouchspread} {
			ctx := theme.ResolveInputs(density)
			cards = append(cards, PreviewCard{
				ThemeName: theme.Name,
				Density:   density,
				Control: gfx.Size{
					W: 96,
					H: ctx.Metrics.Control.Height,
				},
				Input: gfx.Size{
					W: ctx.Metrics.Input.TextFieldMinWidth,
					H: ctx.Metrics.Input.SwitchTrackHeight,
				},
				Navigation: gfx.Size{
					W: ctx.Metrics.Navigation.DrawerMinWidth,
					H: ctx.Metrics.Navigation.TabHeight,
				},
				Notification: gfx.Size{
					W: ctx.Metrics.Notification.DialogMinWidth,
					H: ctx.Metrics.Notification.SnackbarMinHeight,
				},
				Chart: gfx.Size{
					W: 240,
					H: ctx.Metrics.Notification.ProgressLinearThickness,
				},
				Notes: "preview data for phase-5 baseline calibration",
			})
		}
	}
	return cards
}
