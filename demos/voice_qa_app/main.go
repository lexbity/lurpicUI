package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/voice_qa_app/voiceqa"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

func main() {
	var (
		windowWidth   = flag.Int("width", 1680, "window width")
		windowHeight  = flag.Int("height", 1120, "window height")
		fakeAudio     = flag.Bool("fake-audio", false, "use the in-process fake audio backend")
		inputDevice   = flag.String("input", "", "input device id")
		outputDevice  = flag.String("output", "", "output device id")
		monitorDevice = flag.String("monitor", "", "monitor device id")
	)
	flag.Parse()

	useFakeAudio := *fakeAudio || (*inputDevice == "" && *outputDevice == "" && *monitorDevice == "")
	if useFakeAudio && !*fakeAudio {
		fmt.Fprintln(os.Stderr, "voiceqa: no devices specified; using fake-audio backend")
	}
	fmt.Fprintf(os.Stderr, "voiceqa: starting (fake-audio=%t input=%q output=%q monitor=%q)\n", useFakeAudio, *inputDevice, *outputDevice, *monitorDevice)
	host := voiceqa.NewHost(voiceqa.HostOptions{
		FakeAudio:       useFakeAudio,
		InputDeviceID:   *inputDevice,
		OutputDeviceID:  *outputDevice,
		MonitorDeviceID: *monitorDevice,
	})
	defer func() { _ = host.Stop() }()

	if err := host.Start(); err != nil {
		log.Fatalf("voiceqa: start failed: %v", err)
	}
	fmt.Fprintf(os.Stderr, "voiceqa: host started; entering GUI loop\n")

	config := app.DefaultConfig("Voice QA", *windowWidth, *windowHeight)
	config.Fonts = defaultFontSources()
	config.Theme = voiceQADarkTheme()

	if err := app.Run(config, func(ctx app.BuildContext) facet.FacetImpl {
		fmt.Fprintln(os.Stderr, "voiceqa: building root facet")
		shaper := text.NewShaper(ctx.FontRegistry)
		shaper.SetContentScale(ctx.ContentScale)
		return voiceqa.NewRootFacet(ctx.Theme, shaper, host)
	}); err != nil {
		log.Fatal(err)
	}
}

func defaultFontSources() []app.FontSource {
	candidates := []string{
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return []app.FontSource{{Path: path, Name: "Noto Sans"}}
		}
	}
	return nil
}

type voiceQADarkThemeContext struct {
	tokens theme.Tokens
}

func voiceQADarkTheme() theme.Context {
	return voiceQADarkThemeContext{tokens: theme.DarkTokens()}
}

func scaleTextStyle(style text.TextStyle, scale float32) text.TextStyle {
	if scale <= 0 {
		return style
	}
	style.Size *= scale
	style.LineHeight *= scale
	return style
}

func (c voiceQADarkThemeContext) Color(t theme.ColorToken) gfx.Color {
	switch t {
	case theme.ColorBackground:
		return c.tokens.Color.Background
	case theme.ColorSurface:
		return c.tokens.Color.Surface
	case theme.ColorSurfaceVariant:
		return c.tokens.Color.SurfaceVariant
	case theme.ColorPrimary:
		return c.tokens.Color.Primary
	case theme.ColorOnPrimary:
		return c.tokens.Color.OnPrimary
	case theme.ColorText:
		return c.tokens.Color.OnSurface
	case theme.ColorTextSecondary:
		return c.tokens.Color.OnSurfaceVariant
	case theme.ColorTextDisabled:
		return c.tokens.Color.OnSurfaceVariant.WithAlpha(c.tokens.Color.DisabledOpacity)
	case theme.ColorBorder:
		return c.tokens.Color.SurfaceVariant.WithAlpha(0.6)
	case theme.ColorBorderStrong:
		return c.tokens.Color.OnSurfaceVariant.WithAlpha(0.8)
	case theme.ColorSelection:
		return c.tokens.Color.Primary.WithAlpha(c.tokens.Color.SelectedOverlay)
	case theme.ColorCaret:
		return c.tokens.Color.Primary
	case theme.ColorError:
		return c.tokens.Color.Error
	case theme.ColorSuccess:
		return c.tokens.Color.Success
	case theme.ColorWarning:
		return c.tokens.Color.Warning
	default:
		return gfx.Color{}
	}
}

func (c voiceQADarkThemeContext) Spacing(t theme.SpacingToken) float32 {
	switch t {
	case theme.SpacingXS:
		return c.tokens.Spacing.XS
	case theme.SpacingS:
		return c.tokens.Spacing.SM
	case theme.SpacingM:
		return c.tokens.Spacing.MD
	case theme.SpacingL:
		return c.tokens.Spacing.LG
	case theme.SpacingXL:
		return c.tokens.Spacing.XL
	case theme.SpacingXXL:
		return c.tokens.Spacing.XXL
	default:
		return 0
	}
}

func (c voiceQADarkThemeContext) TextStyle(t theme.TextToken) text.TextStyle {
	scale := float32(1.18)
	switch t {
	case theme.TextBodyS:
		return scaleTextStyle(c.tokens.Typography.BodySmall, scale)
	case theme.TextLabelM:
		return scaleTextStyle(c.tokens.Typography.LabelMedium, scale)
	case theme.TextLabelS:
		return scaleTextStyle(c.tokens.Typography.LabelSmall, scale)
	case theme.TextHeadingS:
		return scaleTextStyle(c.tokens.Typography.HeadlineSmall, scale)
	case theme.TextMonoM:
		return scaleTextStyle(c.tokens.Typography.DataLabel, scale)
	case theme.TextMonoS:
		return scaleTextStyle(c.tokens.Typography.DataAnnotation, scale)
	case theme.TextBodyM:
		fallthrough
	default:
		return scaleTextStyle(c.tokens.Typography.BodyMedium, scale)
	}
}

func (c voiceQADarkThemeContext) Radius(t theme.RadiusToken) float32 {
	switch t {
	case theme.RadiusNone:
		return c.tokens.Radius.None
	case theme.RadiusS:
		return c.tokens.Radius.SM
	case theme.RadiusM:
		return c.tokens.Radius.MD
	case theme.RadiusL:
		return c.tokens.Radius.LG
	default:
		return 0
	}
}
