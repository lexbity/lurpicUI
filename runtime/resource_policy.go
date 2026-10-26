package runtime

import (
	"errors"
	fmt "fmt"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

// ResourcePolicy is the validated startup partition for framework-managed
// resources. Owner wallets remain independent for the lifetime of a runtime.
type ResourcePolicy struct {
	Plan       membudget.Plan
	TextLimits text.Limits
}

// WorkerCounts defines the fixed worker counts for a profile.
// Per FR-10: Compact uses 2 general + 1 decode, Standard uses 4 general + 2 decode.
type WorkerCounts struct {
	GeneralWorkers int
	DecodeWorkers  int
}

func DefaultResourcePolicy(profile membudget.Profile) ResourcePolicy {
	return ResourcePolicy{Plan: membudget.DefaultPlan(profile), TextLimits: text.DefaultLimits(profile)}
}

// ProfileWorkerCounts returns the fixed worker counts for a profile.
func ProfileWorkerCounts(profile membudget.Profile) WorkerCounts {
	if profile == membudget.Compact {
		return WorkerCounts{GeneralWorkers: 2, DecodeWorkers: 1}
	}
	return WorkerCounts{GeneralWorkers: 4, DecodeWorkers: 2}
}

func (p ResourcePolicy) Validate() error {
	if err := p.Plan.Validate(); err != nil {
		return err
	}
	if p.Plan.Caps[membudget.Frame] < 4*framePacketCapacity(p.Plan.Profile)+frameSealScratchBytes {
		return errors.New("runtime: frame wallet must fit four packet groups and scratch")
	}
	if p.TextLimits.MaxSourceBytes <= 0 || p.TextLimits.MaxGlyphs <= 0 || p.TextLimits.MaxLines <= 0 || p.TextLimits.MaxParagraphSpans <= 0 || p.TextLimits.MaxLayouts <= 0 || p.TextLimits.MaxGlyphEntries <= 0 || p.TextLimits.MaxFontFaces <= 0 {
		return errors.New("runtime: text resource limits must all be positive")
	}
	maxLayouts, maxGlyphEntries, maxFaces := 8192, 32768, 64
	if p.Plan.Profile == membudget.Compact {
		maxLayouts, maxGlyphEntries, maxFaces = 2048, 8192, 16
	}
	if p.TextLimits.MaxLayouts > maxLayouts || p.TextLimits.MaxGlyphEntries > maxGlyphEntries || p.TextLimits.MaxFontFaces > maxFaces {
		return fmt.Errorf("runtime: text metadata limits exceed %s profile", p.Plan.Profile)
	}
	if p.TextLimits.MaxParagraphSpans > text.DefaultLimits(p.Plan.Profile).MaxParagraphSpans {
		return errors.New("runtime: paragraph span limit exceeds configured text reservation")
	}
	defaults := text.DefaultLimits(p.Plan.Profile)
	if p.TextLimits.MaxSourceBytes > defaults.MaxSourceBytes || p.TextLimits.MaxGlyphs > defaults.MaxGlyphs || p.TextLimits.MaxLines > defaults.MaxLines {
		return errors.New("runtime: text request limits exceed configured scratch reservation")
	}
	if p.Plan.Caps[membudget.Text] < 1<<20 {
		return errors.New("runtime: text wallet must reserve at least 1 MiB")
	}
	return nil
}
