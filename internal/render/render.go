// Package render converts local Markdown into a self-contained, offline HTML
// reading surface. Each call owns its parser and resource budget and is safe to
// run concurrently with other calls.
package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

//go:embed theme.css
var themeCSS string

const contentPolicy = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src 'none'; connect-src 'none'; font-src 'none'; media-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"

// Render returns a complete HTML document for UTF-8 Markdown. path identifies
// the source file and supplies the base directory for local images and links.
// Raw HTML, remote images, and non-HTTP/non-Markdown links are omitted. Missing
// or blocked images preserve their alternative text instead of failing a file.
func Render(source []byte, path string) (string, error) {
	if !utf8.Valid(source) {
		return "", fmt.Errorf("unsupported text encoding: save the file as UTF-8")
	}
	source = bytes.TrimPrefix(source, []byte{0xef, 0xbb, 0xbf})
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve document path: %w", err)
	}
	r := &documentRenderer{path: abs, images: make(map[string]localImage), highlightRemaining: 60 * time.Millisecond}
	defer r.close()
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(r, 100))),
	)
	var out bytes.Buffer
	out.Grow(len(source) + len(themeCSS) + len(highlightCSS) + 1024)
	out.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><meta http-equiv=\"Content-Security-Policy\" content=\"")
	out.WriteString(contentPolicy)
	out.WriteString("\"><meta name=\"color-scheme\" content=\"dark\"><title>")
	out.WriteString(html.EscapeString(filepath.Base(abs)))
	out.WriteString("</title><style>")
	out.WriteString(highlightCSS)
	out.WriteString(themeCSS)
	out.WriteString("</style></head><body><main>\n")
	context := parser.NewContext(parser.WithIDs(newHeadingIDs()))
	if err := md.Convert(source, &out, parser.WithContext(context)); err != nil {
		return "", fmt.Errorf("render Markdown: %w", err)
	}
	out.WriteString("</main></body></html>\n")
	return out.String(), nil
}

func (r *documentRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.link)
	reg.Register(ast.KindAutoLink, r.autoLink)
	reg.Register(ast.KindImage, r.image)
	reg.Register(ast.KindFencedCodeBlock, r.code)
	reg.Register(ast.KindCodeBlock, r.code)
	reg.Register(extast.KindTable, r.table)
}

func (r *documentRenderer) table(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<div class=\"table-scroll\" tabindex=\"0\" role=\"region\" aria-label=\"Table, scroll horizontally if needed\"><table>\n")
	} else {
		_, _ = w.WriteString("</table></div>\n")
	}
	return ast.WalkContinue, nil
}

func (r *documentRenderer) link(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		if target, ok := r.linkTarget(n.Destination); ok {
			_, _ = w.WriteString(`<a href="` + html.EscapeString(target) + `"`)
			if len(n.Title) > 0 {
				_, _ = w.WriteString(` title="` + html.EscapeString(unescape(n.Title)) + `"`)
			}
			_, _ = w.WriteString(`>`)
		} else {
			_, _ = w.WriteString(`<span class="blocked-link">`)
		}
	} else if _, ok := r.linkTarget(n.Destination); ok {
		_, _ = w.WriteString("</a>")
	} else {
		_, _ = w.WriteString("</span>")
	}
	return ast.WalkContinue, nil
}

func (r *documentRenderer) autoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	label := html.EscapeString(string(n.Label(source)))
	if n.AutoLinkType == ast.AutoLinkEmail {
		_, _ = w.WriteString(label)
	} else if target, ok := r.linkTarget(n.URL(source)); ok {
		_, _ = w.WriteString(`<a class="autolink" href="` + html.EscapeString(target) + `">` + label + `</a>`)
	} else {
		_, _ = w.WriteString(label)
	}
	return ast.WalkSkipChildren, nil
}

func (r *documentRenderer) image(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := unescape(n.Text(source))
	img := r.localImage(n.Destination)
	if img.data == "" {
		if strings.TrimSpace(alt) == "" {
			alt = "Image unavailable"
		}
		_, _ = w.WriteString(`<span class="missing-image" role="img" aria-label="` + html.EscapeString(alt) + `" title="Image unavailable: ` + html.EscapeString(img.reason) + `">` + html.EscapeString(alt) + `</span>`)
	} else {
		_, _ = fmt.Fprintf(w, `<img src="%s" alt="%s" width="%d" height="%d" loading="lazy"`, img.data, html.EscapeString(alt), img.width, img.height)
		if len(n.Title) > 0 {
			_, _ = w.WriteString(` title="` + html.EscapeString(unescape(n.Title)) + `"`)
		}
		_, _ = w.WriteString(`>`)
	}
	return ast.WalkSkipChildren, nil
}

func (r *documentRenderer) code(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	var code bytes.Buffer
	for i := 0; i < node.Lines().Len(); i++ {
		line := node.Lines().At(i)
		code.Write(line.Value(source))
	}
	language := ""
	if fence, ok := node.(*ast.FencedCodeBlock); ok {
		language = strings.ToLower(string(fence.Language(source)))
	}
	_, _ = w.WriteString(`<pre tabindex="0" aria-label="Code, scroll horizontally if needed"><code class="chroma">`)
	if highlighted, ok := r.highlight(code.String(), language); ok {
		_, _ = w.WriteString(highlighted)
	} else {
		_, _ = w.WriteString(html.EscapeString(code.String()))
	}
	_, _ = w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

func unescape(b []byte) string {
	return html.UnescapeString(string(util.UnescapePunctuations(b)))
}
