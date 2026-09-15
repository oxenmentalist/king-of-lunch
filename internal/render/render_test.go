package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func mustRender(t testing.TB, source, path string) string {
	t.Helper()
	got, err := Render([]byte(source), path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func body(s string) string {
	_, s, _ = strings.Cut(s, "<main>\n")
	s, _, _ = strings.Cut(s, "</main>")
	return s
}

func TestMarkdownStructures(t *testing.T) {
	source := "# Hello lunch\n\nA **bold**, *emphasized*, ~~removed~~ and `inline` line.\n\n- item\n  - nested\n\n1. first\n\n> quote\n\n- [x] done\n- [ ] pending\n\n| Name | Count |\n| :--- | ---: |\n| rice | 12 |\n\n    indented <code>\n\n[section](#hello-lunch)\n\nUnicode: 日本語 🍱 café.\n"
	got := mustRender(t, source, "/tmp/lunch.md")
	for _, want := range []string{`<h1 id="hello-lunch">`, "<strong>bold</strong>", "<em>emphasized</em>", "<del>removed</del>", "<code>inline</code>", "<ul>", "<ol>", "<blockquote>", `checked="" disabled="" type="checkbox"`, `class="table-scroll" tabindex="0"`, "<table>", `style="text-align:right"`, "indented &lt;code&gt;", `href="#hello-lunch"`, "日本語 🍱 café"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in rendered body:\n%s", want, body(got))
		}
	}
}

func TestEncodingAndEmptyFiles(t *testing.T) {
	if _, err := Render([]byte{0xff, 0xfe, 0x41, 0}, "/tmp/utf16.md"); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	for _, source := range []string{"", "\xef\xbb\xbf", "\xef\xbb\xbf# Hello\r\n\r\nText\r\n"} {
		got := mustRender(t, source, "/tmp/<unsafe>.md")
		if strings.Contains(got, "\ufeff") || !strings.Contains(got, "<title>&lt;unsafe&gt;.md</title>") {
			t.Fatalf("BOM or title escaping failed: %s", got[:500])
		}
	}
}

func TestHeadingIDsAreUniqueAndSupportUnicode(t *testing.T) {
	got := body(mustRender(t, "# Lunch\n\n# Lunch\n\n# Lunch-1\n\n# Lunch\n\n# 日本語\n", "/tmp/headings.md"))
	for _, id := range []string{"lunch", "lunch-1", "lunch-1-1", "lunch-2", "日本語"} {
		if strings.Count(got, `id="`+id+`"`) != 1 {
			t.Errorf("missing or duplicate heading ID %q: %s", id, got)
		}
	}
}

func TestTableColumnAlignment(t *testing.T) {
	got := body(mustRender(t, "| Left | Center | Right |\n| :--- | :---: | ---: |\n| a | b | 42 |\n", "/tmp/table.md"))
	for _, want := range []string{
		`<th style="text-align:left">Left</th>`,
		`<th style="text-align:center">Center</th>`,
		`<th style="text-align:right">Right</th>`,
		`<td style="text-align:left">a</td>`,
		`<td style="text-align:center">b</td>`,
		`<td style="text-align:right">42</td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing aligned cell %q in %s", want, got)
		}
	}
}

func TestCodeHighlightAndFallback(t *testing.T) {
	for _, label := range []string{"go", "Go", "python", "json", "bash", "c", "cpp", "css", "html", "xml", "javascript", "markdown", "rust", "sql", "swift", "typescript", "yaml"} {
		t.Run(label, func(t *testing.T) {
			code := "const message = \"hello\"\n"
			if label == "go" {
				code = "package main\nfunc main() { println(42) }\n"
			}
			if label == "html" || label == "xml" {
				code = "<p class=\"note\">Hello</p>\n"
			}
			if label == "markdown" {
				code = "# Hello\n\n**strong**\n"
			}
			got := body(mustRender(t, "```"+label+"\n"+code+"```\n", "/tmp/code.md"))
			if !strings.Contains(got, `<span class=`) {
				t.Errorf("explicit language %s was not highlighted: %s", label, got)
			}
		})
	}
	for _, label := range []string{"", "not-a-language", "../../go", `evil\"onclick=\"x`} {
		got := body(mustRender(t, "```"+label+"\n<script>alert(1)</script>\n```", "/tmp/code.md"))
		if strings.Contains(got, "<span") || !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("plain fallback failed for %q: %s", label, got)
		}
	}
	oversized := strings.Repeat("var n = 42\n", maxHighlightSize/10+1)
	got := body(mustRender(t, "```go\n"+oversized+"```", "/tmp/big.md"))
	if strings.Contains(got, "<span") || !strings.Contains(got, oversized) {
		t.Fatal("oversized code was highlighted or truncated")
	}
}

func TestUnsafeContentHasNoActiveTargets(t *testing.T) {
	source := "<script>window.location='https://evil.test'</script>\n\n<img src=x onerror=alert(1)>\n\n<iframe src='file:///etc/passwd'></iframe>\n\n[bad](javascript:alert%281%29) [data](data:text/html,bad) [vb](vbscript:bad) [mail](mailto:x@example.com) [file](file:///etc/passwd) [remote file](file://remote/note.md) [entity](jav&#x61;script:bad)\n\n![remote](https://evil.test/tracker.png) ![SVG](data:image/svg+xml,bad)\n\n<https://example.com> <me@example.com>\n"
	got := mustRender(t, source, "/tmp/hostile.md")
	for _, bad := range []string{"<script", "<iframe", "onerror=", `href="javascript:`, `href="data:`, `href="vbscript:`, `href="mailto:`, `href="file:///etc/passwd`, `src="https:`, `src="data:image/svg`} {
		if strings.Contains(got, bad) {
			t.Errorf("active content %q survived: %s", bad, body(got))
		}
	}
	if !strings.Contains(got, contentPolicy) || !strings.Contains(got, `href="https://example.com"`) {
		t.Fatal("missing CSP or allowed external link")
	}
	if !strings.Contains(got, ">remote</span>") || !strings.Contains(got, ">SVG</span>") {
		t.Fatal("blocked image alt text lost")
	}
}

func TestLinkResolution(t *testing.T) {
	r := &documentRenderer{path: "/tmp/notes space/読み物.md"}
	cases := []struct {
		input, want string
		ok          bool
	}{
		{"#section", "#section", true},
		{"next%20note.md#heading", "kol-file://document/tmp/notes%20space/next%20note.md#heading", true},
		{"../other.markdown", "kol-file://document/tmp/other.markdown", true},
		{"日本語.md", "kol-file://document/tmp/notes%20space/%E6%97%A5%E6%9C%AC%E8%AA%9E.md", true},
		{"HTTPS://example.com/a?q=x&b=y", "https://example.com/a?q=x&b=y", true},
		{"https://example.com?a=1&amp;b=2", "https://example.com?a=1&b=2", true},
		{"file:///tmp/note.md", "kol-file://document/tmp/note.md", true},
		{"//evil.test/note.md", "", false},
		{"javascript:alert(1)", "", false},
		{"jav&#x61;script:alert(1)", "", false},
		{"https://", "", false},
		{"https://user:pass@example.com/", "", false},
		{"https:\\evil.test", "", false},
		{"note.pdf", "", false},
		{"note.md?execute=1", "", false},
	}
	for _, tc := range cases {
		got, ok := r.linkTarget([]byte(tc.input))
		if ok != tc.ok || got != tc.want {
			t.Errorf("linkTarget(%q) = %q, %v; want %q, %v", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPrintStyling(t *testing.T) {
	got := mustRender(t, "[Go](https://go.dev/) and https://example.com/\n\n```go\nfunc main() {}\n```\n", "/tmp/print.md")
	for _, want := range []string{`<a href="https://go.dev/">Go</a>`, `<a class="autolink" href="https://example.com/">https://example.com/</a>`, "@media print {\n", "#624c83"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in rendered document", want)
		}
	}
}

func writePNG(t testing.TB, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.NRGBA{R: 126, G: 156, B: 216, A: 255})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLocalImagesAndContainment(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "notes")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePNG(t, filepath.Join(dir, "local image.png"))
	writePNG(t, filepath.Join(base, "outside.png"))
	if err := os.Symlink("../outside.png", filepath.Join(dir, "escape.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("local image.png", filepath.Join(dir, "inside.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake.png"), []byte("<svg onload='bad()'></svg>"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "![local](local%20image.png) ![inside](inside.png) ![outside](../outside.png) ![escape](escape.png) ![fake](fake.png) ![absent](missing.png)"
	got := body(mustRender(t, source, filepath.Join(dir, "note.md")))
	if strings.Count(got, `src="data:image/png;base64,`) != 2 {
		t.Errorf("expected exactly two contained images: %s", got)
	}
	if strings.Count(got, `class="missing-image"`) != 4 {
		t.Errorf("expected four blocked/missing images: %s", got)
	}
	if !strings.Contains(got, `width="3" height="2"`) || !strings.Contains(got, `alt="local"`) {
		t.Fatal("image dimensions or alt text missing")
	}
}

func TestImageLimits(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "large.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxImageBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	got := body(mustRender(t, "![large](large.png)", filepath.Join(dir, "note.md")))
	if strings.Contains(got, "<img") || !strings.Contains(got, "size limit") {
		t.Fatal("oversized image was not rejected")
	}
}

func TestFIFOImageDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.png"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() {
		got, _ := Render([]byte("![pipe](pipe.png)"), filepath.Join(dir, "note.md"))
		done <- got
	}()
	select {
	case got := <-done:
		if !strings.Contains(got, "regular file") {
			t.Fatal("FIFO was not rejected as a non-regular file")
		}
	case <-time.After(time.Second):
		t.Fatal("image open blocked on a FIFO")
	}
}

func TestConcurrentRendering(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			got := mustRender(t, "# Shared\n\n```go\npackage main\n```", "/tmp/concurrent.md")
			if !strings.Contains(got, `<h1 id="shared">`) {
				t.Error("concurrent rendering lost heading")
			}
		})
	}
	wg.Wait()
}

func BenchmarkRender(b *testing.B) {
	paragraph := []byte("## Reading\n\nA lightweight **Markdown** viewer with Unicode 日本語, links to [documentation](https://example.com), and `inline code`.\n\n| Name | Value |\n| --- | ---: |\n| Lunch | 42 |\n\n")
	for _, size := range []struct {
		name string
		n    int
	}{{"100KiB", 100 << 10}, {"1MiB", 1 << 20}, {"10MiB", 10 << 20}} {
		b.Run(size.name, func(b *testing.B) {
			source := bytes.Repeat(paragraph, size.n/len(paragraph))
			b.SetBytes(int64(len(source)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Render(source, "/tmp/benchmark.md"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestDecodedControlCharactersDoNotReachNativeURLs(t *testing.T) {
	for _, target := range []string{"notes.md%00tail.md", "notes%0A.md", "notes%0D.md", "notes%09.md", "notes%5C.md"} {
		result, err := Render([]byte("[blocked]("+target+")"), filepath.Join(t.TempDir(), "doc.md"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(result, "kol-file:") {
			t.Fatalf("decoded control URL accepted: %s", target)
		}
	}
}
