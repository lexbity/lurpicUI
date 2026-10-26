package text

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"github.com/go-text/typesetting/language"
)

func serviceFixture(t *testing.T, limits Limits) (*Service, *membudget.Wallet) {
	t.Helper()
	wallet, err := membudget.NewWallet(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(testFontBytes, "Noto Sans"); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(registry, limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service, wallet
}

func TestTextServiceReuseAndBudget(t *testing.T) {
	s, wallet := serviceFixture(t, DefaultLimits(membudget.Standard))
	r := ShapeRequest{Content: "service label", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1}
	a, err := s.Shape(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Shape(r)
	if err != nil {
		t.Fatal(err)
	}
	if a.Layout != b.Layout {
		t.Fatal("cache hit did not share immutable layout")
	}
	a.Release()
	b.Release()
	hits, misses, entries, bytes := s.Stats()
	if hits != 1 || misses != 1 || entries != 1 || bytes == 0 {
		t.Fatalf("stats = %d/%d/%d/%d", hits, misses, entries, bytes)
	}
	_, _, live, _ := wallet.Snapshot()
	if live != bytes+s.registry.SourceBytes() {
		t.Fatalf("wallet live = %d, cache bytes = %d, font sources = %d", live, bytes, s.registry.SourceBytes())
	}
}

func TestFontSourcesStayChargedUntilLastLayoutLease(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	wallet, err := membudget.NewWallet(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(testFontBytes, "Noto Sans"); err != nil {
		t.Fatal(err)
	}
	_, _, live, _ := wallet.Snapshot()
	if live != registry.SourceBytes() {
		t.Fatalf("font source charge = %d, retained source bytes = %d", live, registry.SourceBytes())
	}
	service, err := NewService(registry, limits)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := service.Shape(ShapeRequest{Content: "pinned font", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, live, _ = wallet.Snapshot()
	if live == 0 || registry.FaceCount() == 0 {
		t.Fatal("service close released font data while a layout lease was live")
	}
	lease.Release()
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 || registry.FaceCount() != 0 {
		t.Fatalf("final lease release left font/service state: reserved=%d live=%d retiring=%d faces=%d", reserved, live, retiring, registry.FaceCount())
	}
}

func TestFontSourceReservationRefusesBeforeRetainingBytes(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	wallet, err := membudget.NewWallet(uint64(len(testFontBytes) - 1))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(testFontBytes, "Noto Sans"); err == nil {
		t.Fatal("font source larger than wallet was accepted")
	}
	if got := registry.SourceBytes(); got != 0 {
		t.Fatalf("retained source bytes = %d after refusal", got)
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("refused source left wallet charges: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}

func TestFontSourceNameLimitPrecedesReservation(t *testing.T) {
	wallet, err := membudget.NewWallet(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewFontRegistry(wallet, DefaultLimits(membudget.Compact).MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("n", maxFontSourceNameBytes+1)
	if err := registry.LoadFontBytes(testFontBytes, name); err == nil {
		t.Fatal("expected oversized source name to be refused")
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 || registry.SourceCount() != 0 {
		t.Fatalf("oversized name changed ownership: reserved=%d live=%d retiring=%d sources=%d", reserved, live, retiring, registry.SourceCount())
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedFontRegistryRejectsAdmissionAndService(t *testing.T) {
	wallet, err := membudget.NewWallet(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits(membudget.Compact)
	registry, err := NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(testFontBytes, "late-font"); err == nil {
		t.Fatal("closed registry accepted a font source")
	}
	if _, err := NewService(registry, limits); err == nil {
		t.Fatal("closed registry produced a text service")
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("closed registry admission changed wallet: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}

func TestTextScaleDoesNotLeakBetweenRequests(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	base, err := s.Shape(ShapeRequest{Content: "scale", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	large, err := s.Shape(ShapeRequest{Content: "scale", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	if base.Layout == large.Layout {
		t.Fatal("scale missing from cache key")
	}
	if large.Layout.Bounds.Width() <= base.Layout.Bounds.Width() {
		t.Fatal("scale request did not affect shaping")
	}
	base.Release()
	large.Release()
}

func TestTextKeyIncludesDirectionAndFontGeneration(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	base := ShapeRequest{Content: "abc", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1}
	a, err := s.Shape(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Direction = 1
	b, err := s.Shape(base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Layout == b.Layout {
		t.Fatal("direction missing from cache key")
	}
	a.Release()
	b.Release()
	before := s.misses
	if err := s.registry.LoadFontBytes(testFontBoldBytes, "Noto Sans Bold"); err != nil {
		t.Fatal(err)
	}
	c, err := s.Shape(base)
	if err != nil {
		t.Fatal(err)
	}
	c.Release()
	if s.misses != before+1 {
		t.Fatal("font generation missing from key")
	}
}

func TestTextServiceShapesRTLCombiningAndEmojiRequests(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	p := Paragraph{
		Spans: []TextSpan{{
			Text:      "مرحبا e\u0301 👩🏽‍💻",
			Style:     TextStyle{Family: "Noto Sans", Size: 14},
			Direction: 1,
		}},
		Direction: 1,
	}
	lease, err := s.Shape(ShapeRequest{Paragraph: &p, Scale: 1, MaxWidth: 240, Direction: 1, Overflow: OverflowWrap, Locale: "ar"})
	if err != nil {
		t.Fatal(err)
	}
	if lease.Layout == nil || len(lease.Layout.Lines) == 0 || lease.Layout.Source == "" {
		t.Fatal("multilingual shape returned an empty layout")
	}
	lease.Release()
	_, reserved, _, _ := s.wallet.Snapshot()
	if reserved != 0 {
		t.Fatalf("shaping scratch remained reserved: %d", reserved)
	}
}

func TestTextKeyIncludesParagraphLanguageAndScript(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	paragraph := Paragraph{Spans: []TextSpan{{Text: "abc", Style: TextStyle{Family: "Noto Sans", Size: 14}}}, MaxWidth: 200}
	paragraph.Language = language.NewLanguage("en")
	first, err := s.Shape(ShapeRequest{Paragraph: &paragraph, Scale: 1, Overflow: OverflowWrap})
	if err != nil {
		t.Fatal(err)
	}
	paragraph.Language = language.NewLanguage("ar")
	second, err := s.Shape(ShapeRequest{Paragraph: &paragraph, Scale: 1, Overflow: OverflowWrap})
	if err != nil {
		t.Fatal(err)
	}
	if first.Layout == second.Layout {
		t.Fatal("paragraph language missing from cache key")
	}
	first.Release()
	second.Release()
}

func TestTextRejectsInvalidRequestBeforeShaping(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	_, err := s.Shape(ShapeRequest{Content: "bad", Style: TextStyle{Family: "Noto Sans", Size: float32(math.NaN())}, Scale: 1})
	if err == nil {
		t.Fatal("non-finite style metric was accepted")
	}
	_, misses, entries, _ := s.Stats()
	if misses != 0 || entries != 0 {
		t.Fatalf("invalid request reached shaping/cache: misses=%d entries=%d", misses, entries)
	}
}

func TestTextRejectsOversizedFontFamilyBeforeKeyAllocation(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	_, err := s.Shape(ShapeRequest{Content: "bounded", Style: TextStyle{Family: strings.Repeat("x", MaxFontFamilyBytes+1), Size: 14}, Scale: 1})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) || resourceErr.Resource != "font family bytes" {
		t.Fatalf("font family limit error = %T %v", err, err)
	}
	_, misses, entries, _ := s.Stats()
	if misses != 0 || entries != 0 {
		t.Fatalf("oversized font family reached key/shaping/cache: misses=%d entries=%d", misses, entries)
	}
}

func TestTextScratchRefusalPrecedesShaping(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	limits.MaxSourceBytes, limits.MaxGlyphs, limits.MaxLines = 128, 100, 4
	wallet, err := membudget.NewWallet(uint64(len(testFontBytes)) + 1)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.LoadFontBytes(testFontBytes, "Noto Sans"); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(registry, limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.Shape(ShapeRequest{Content: "bounded", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) {
		t.Fatalf("scratch refusal = %T %v, want ResourceError", err, err)
	}
	_, reserved, _, _ := wallet.Snapshot()
	if reserved != 0 {
		t.Fatalf("failed scratch admission retained %d reserved bytes", reserved)
	}
	_, _, entries, _ := s.Stats()
	if entries != 0 {
		t.Fatal("failed scratch admission published a layout")
	}
}

func TestTextParagraphSpanLimitPrecedesCopyAndShape(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	limits.MaxParagraphSpans = 1
	s, _ := serviceFixture(t, limits)
	p := Paragraph{Spans: []TextSpan{
		{Text: "one", Style: TextStyle{Family: "Noto Sans", Size: 14}},
		{Text: "two", Style: TextStyle{Family: "Noto Sans", Size: 14}},
	}}
	_, err := s.Shape(ShapeRequest{Paragraph: &p, Scale: 1})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) || resourceErr.Resource != "paragraph spans" {
		t.Fatalf("span limit error = %T %v", err, err)
	}
	_, misses, entries, _ := s.Stats()
	if misses != 0 || entries != 0 {
		t.Fatalf("over-limit paragraph reached shaping/cache: misses=%d entries=%d", misses, entries)
	}
}

func BenchmarkTextServiceLabel(b *testing.B) {
	newRegistry := func(b *testing.B) *FontRegistry {
		b.Helper()
		wallet, err := membudget.NewWallet(membudget.DefaultPlan(membudget.Standard).Caps[membudget.Text])
		if err != nil {
			b.Fatal(err)
		}
		registry, err := NewFontRegistry(wallet, DefaultLimits(membudget.Standard).MaxFontFaces)
		if err != nil {
			b.Fatal(err)
		}
		if err := registry.LoadFontBytes(testFontBytes, "Noto Sans"); err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() { _ = registry.Close() })
		return registry
	}
	style := TextStyle{Family: "Noto Sans", Size: 14}
	request := ShapeRequest{Content: "Service label 0123456789", Style: style, Scale: 1}
	b.Run("fresh-engine", func(b *testing.B) {
		registry := newRegistry(b)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			layout := newShaper(registry).ShapeSimple(request.Content, style)
			if layout == nil {
				b.Fatal("shaping returned nil")
			}
		}
	})
	b.Run("reused-service", func(b *testing.B) {
		registry := newRegistry(b)
		service, err := NewService(registry, DefaultLimits(membudget.Standard))
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = service.Close() }()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			lease, err := service.Shape(request)
			if err != nil {
				b.Fatal(err)
			}
			lease.Release()
		}
	})
}

func TestOversizeShapeRefusedBeforeShape(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	limits.MaxSourceBytes = 3
	s, _ := serviceFixture(t, limits)
	_, err := s.Shape(ShapeRequest{Content: "four", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) {
		t.Fatalf("error = %T %v, want ResourceError", err, err)
	}
	if _, misses, entries, _ := s.Stats(); misses != 0 || entries != 0 {
		t.Fatal("oversized source reached shaper/cache")
	}
}

func TestTextLeasePinsCacheUntilRelease(t *testing.T) {
	limits := DefaultLimits(membudget.Standard)
	limits.MaxLayouts = 1
	s, wallet := serviceFixture(t, limits)
	a, err := s.Shape(ShapeRequest{Content: "one", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Shape(ShapeRequest{Content: "two", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	var resourceErr *ResourceError
	if !errors.As(err, &resourceErr) {
		t.Fatalf("pinned capacity error = %T %v", err, err)
	}
	if cap, _, live, _ := wallet.Snapshot(); live > cap {
		t.Fatal("wallet exceeded cap")
	}
	a.Release()
	s.Trim(0)
	b, err := s.Shape(ShapeRequest{Content: "two", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	b.Release()
	s.Trim(0)
	_, _, live, _ := wallet.Snapshot()
	if live != s.registry.SourceBytes() {
		t.Fatalf("trim retained %d bytes beyond font sources (%d)", live, s.registry.SourceBytes())
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, live, _ = wallet.Snapshot()
	if live != 0 {
		t.Fatalf("close retained %d bytes", live)
	}
}

func TestTextTwoServicesReleaseIndependently(t *testing.T) {
	first, walletA := serviceFixture(t, DefaultLimits(membudget.Standard))
	second, walletB := serviceFixture(t, DefaultLimits(membudget.Standard))
	request := ShapeRequest{Content: "runtime", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1}
	a, err := first.Shape(request)
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.Shape(request)
	if err != nil {
		t.Fatal(err)
	}
	a.Release()
	b.Release()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, liveA, _ := walletA.Snapshot()
	_, _, liveB, _ := walletB.Snapshot()
	if liveA != 0 || liveB == 0 {
		t.Fatalf("independent charges: %d, %d", liveA, liveB)
	}
}

func TestTextServiceConcurrentRequests(t *testing.T) {
	s, wallet := serviceFixture(t, DefaultLimits(membudget.Standard))
	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := s.Shape(ShapeRequest{Content: "shared concurrent label", Style: TextStyle{Family: "Noto Sans", Size: 14}, Scale: 1})
			if err != nil {
				errs <- err
				return
			}
			if lease.Layout == nil {
				errs <- errors.New("concurrent request returned nil layout")
				lease.Release()
				return
			}
			lease.Release()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	_, _, live, _ := wallet.Snapshot()
	if live == 0 {
		t.Fatal("concurrent requests lost the resident cache entry")
	}
}

func TestTextServiceBindsToOnlyOneRuntime(t *testing.T) {
	s, _ := serviceFixture(t, DefaultLimits(membudget.Standard))
	if err := s.BindRuntime(); err != nil {
		t.Fatal(err)
	}
	if err := s.BindRuntime(); err == nil {
		t.Fatal("service bound to a second runtime")
	}
}
