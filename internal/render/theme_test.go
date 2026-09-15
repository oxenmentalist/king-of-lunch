package render

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestDocumentSupportsLiveAppearanceChanges(t *testing.T) {
	got := mustRender(t, "```go\npackage main\nconst count = 42\n```", "/tmp/theme.md")
	if !strings.Contains(got, `<meta name="color-scheme" content="light dark">`) {
		t.Fatal("document must allow both WebKit appearances")
	}
	if strings.Count(got, "@media (prefers-color-scheme: light)") != 2 {
		t.Fatal("document and syntax palettes must both follow effective appearance")
	}
	// Every syntax selector must be overridden in light mode, including inherited
	// token subtypes such as literal strings and numbers.
	dark, light, ok := strings.Cut(highlightCSS, "@media (prefers-color-scheme: light)")
	if !ok {
		t.Fatal("missing light syntax palette")
	}
	light, _, _ = strings.Cut(light, "@media print")
	selectors := regexp.MustCompile(`(?m)^([^{}\n]+)\s*\{`)
	for _, match := range selectors.FindAllStringSubmatch(dark, -1) {
		if !strings.Contains(light, strings.TrimSpace(match[1])+" {") {
			t.Errorf("light palette does not override %s", match[1])
		}
	}
}

func luminance(c chroma.Colour) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= .04045 {
			return s / 12.92
		}
		return math.Pow((s+.055)/1.055, 2.4)
	}
	return .2126*linear(c.Red()) + .7152*linear(c.Green()) + .0722*linear(c.Blue())
}

func TestLightSyntaxContrast(t *testing.T) {
	background := luminance(kanagawaLotus.Get(chroma.Background).Background)
	for token := range chroma.StandardTypes {
		color := kanagawaLotus.Get(token).Colour
		if !color.IsSet() {
			continue
		}
		foreground := luminance(color)
		ratio := (math.Max(foreground, background) + .05) / (math.Min(foreground, background) + .05)
		if ratio < 4.5 {
			t.Errorf("%s (%s) has insufficient light code contrast: %.2f", token, color, ratio)
		}
	}
}

func TestPrintPaletteOverridesScreenAppearance(t *testing.T) {
	_, paper, ok := strings.Cut(highlightCSS, "@media print {")
	if !ok || !strings.Contains(paper, "#f5f3ea") || strings.Contains(paper, "#e7dba0") {
		t.Fatal("printing must retain its paper palette after screen appearance rules")
	}
	screen, _, _ := strings.Cut(highlightCSS, "@media print {")
	selectors := regexp.MustCompile(`(?m)^([^{}\n]+)\s*\{`)
	for _, match := range selectors.FindAllStringSubmatch(screen, -1) {
		selector := strings.TrimSpace(match[1])
		if !strings.HasPrefix(selector, "@") && !strings.Contains(paper, selector+" {") {
			t.Errorf("print palette does not override %s", selector)
		}
	}
}
