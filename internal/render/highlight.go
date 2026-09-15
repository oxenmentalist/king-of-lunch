package render

import (
	"bytes"
	"time"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
)

var kanagawa = chroma.MustNewStyle("king-of-lunch-kanagawa", chroma.StyleEntries{
	chroma.Background:          "#DCD7BA bg:#16161D",
	chroma.Text:                "#DCD7BA",
	chroma.Error:               "#E82424",
	chroma.Comment:             "#9A9A88 italic",
	chroma.CommentPreproc:      "#E6C384",
	chroma.Keyword:             "#957FB8",
	chroma.KeywordType:         "#7AA89F",
	chroma.Operator:            "#C0A36E",
	chroma.Punctuation:         "#9CABCA",
	chroma.NameBuiltin:         "#7E9CD8",
	chroma.NameClass:           "#7AA89F",
	chroma.NameFunction:        "#7E9CD8",
	chroma.NameTag:             "#7E9CD8",
	chroma.NameAttribute:       "#E6C384",
	chroma.NameDecorator:       "#7AA89F",
	chroma.NameConstant:        "#FFA066",
	chroma.LiteralString:       "#98BB6C",
	chroma.LiteralStringEscape: "#7FB4CA",
	chroma.LiteralNumber:       "#D27E99",
	chroma.GenericDeleted:      "#E46876",
	chroma.GenericInserted:     "#98BB6C",
	chroma.GenericHeading:      "#7E9CD8 bold",
	chroma.GenericStrong:       "bold",
	chroma.GenericEmph:         "italic",
})

// Lotus uses the same token categories as Wave so changing appearance never
// leaves a highlighted token with a color from the other palette.
var kanagawaLotus = chroma.MustNewStyle("king-of-lunch-kanagawa-lotus", chroma.StyleEntries{
	chroma.Background:          "#545464 bg:#E7DBA0",
	chroma.Text:                "#545464",
	chroma.LineNumbers:         "#625E4C",
	chroma.LineNumbersTable:    "#625E4C",
	chroma.Error:               "#A5343C",
	chroma.Comment:             "#625E4C italic",
	chroma.CommentPreproc:      "#6F5935",
	chroma.Keyword:             "#624C83",
	chroma.KeywordType:         "#49624E",
	chroma.Operator:            "#6F5935",
	chroma.Punctuation:         "#555465",
	chroma.NameBuiltin:         "#34548A",
	chroma.NameClass:           "#49624E",
	chroma.NameFunction:        "#34548A",
	chroma.NameTag:             "#34548A",
	chroma.NameAttribute:       "#6F5935",
	chroma.NameDecorator:       "#49624E",
	chroma.NameConstant:        "#8E4B32",
	chroma.LiteralString:       "#4F613A",
	chroma.LiteralStringEscape: "#2D626B",
	chroma.LiteralNumber:       "#864D70",
	chroma.GenericDeleted:      "#A5343C",
	chroma.GenericInserted:     "#4F613A",
	chroma.GenericHeading:      "#34548A bold",
	chroma.GenericStrong:       "bold",
	chroma.GenericEmph:         "italic",
})

// Paper uses Kanagawa Lotus, the light variant, so tokens stay legible in ink.
var kanagawaPrint = chroma.MustNewStyle("king-of-lunch-kanagawa-print", chroma.StyleEntries{
	chroma.Background:          "#1F1F28 bg:#F5F3EA",
	chroma.Text:                "#1F1F28",
	chroma.Error:               "#C84053",
	chroma.Comment:             "#716E61 italic",
	chroma.CommentPreproc:      "#836F4A",
	chroma.Keyword:             "#624C83",
	chroma.KeywordType:         "#597B75",
	chroma.Operator:            "#836F4A",
	chroma.Punctuation:         "#545464",
	chroma.NameBuiltin:         "#4D699B",
	chroma.NameClass:           "#597B75",
	chroma.NameFunction:        "#4D699B",
	chroma.NameTag:             "#4D699B",
	chroma.NameAttribute:       "#77713F",
	chroma.NameDecorator:       "#597B75",
	chroma.NameConstant:        "#CC6D00",
	chroma.LiteralString:       "#6F894E",
	chroma.LiteralStringEscape: "#4E8CA2",
	chroma.LiteralNumber:       "#B35B79",
	chroma.GenericDeleted:      "#C84053",
	chroma.GenericInserted:     "#6F894E",
	chroma.GenericHeading:      "#4D699B bold",
	chroma.GenericStrong:       "bold",
	chroma.GenericEmph:         "italic",
})

var formatter = chromahtml.New(chromahtml.WithClasses(true), chromahtml.PreventSurroundingPre(true), chromahtml.WithCSSComments(false))

var highlightCSS = func() string {
	var out bytes.Buffer
	_ = formatter.WriteCSS(&out, kanagawa)
	out.WriteString("\n@media (prefers-color-scheme: light) {\n")
	_ = formatter.WriteCSS(&out, kanagawaLotus)
	out.WriteString("}\n")
	out.WriteString("@media print {\n")
	_ = formatter.WriteCSS(&out, kanagawaPrint)
	out.WriteString("}\n")
	return out.String()
}()

// Only explicit fence labels select a lexer. This also prevents a newly added
// upstream lexer from unexpectedly becoming part of the supported surface.
var languages = map[string]string{
	"bash": "bash", "sh": "bash", "shell": "bash", "zsh": "bash",
	"c": "c", "cpp": "cpp", "c++": "cpp", "cc": "cpp",
	"css": "css", "go": "go", "golang": "go",
	"html": "html", "xml": "xml", "javascript": "javascript", "js": "javascript",
	"json": "json", "markdown": "markdown", "md": "markdown",
	"python": "python", "py": "python", "rust": "rust", "rs": "rust",
	"sql": "sql", "swift": "swift", "typescript": "typescript", "ts": "typescript",
	"yaml": "yaml", "yml": "yaml",
}

func (r *documentRenderer) highlight(code, language string) (result string, ok bool) {
	name, supported := languages[language]
	if !supported || len(code) > maxHighlightSize || r.highlightRemaining <= 0 {
		return "", false
	}
	start := time.Now()
	defer func() {
		r.highlightRemaining -= time.Since(start)
		// A lexer or formatter panic must never prevent reading a document.
		if recover() != nil {
			result, ok = "", false
		}
	}()
	lexer := lexers.Get(name)
	if lexer == nil {
		return "", false
	}
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", false
	}
	tokens := make([]chroma.Token, 0, len(code)/4)
	for {
		token := iterator()
		if token == chroma.EOF {
			break
		}
		tokens = append(tokens, token)
		if time.Since(start) > r.highlightRemaining {
			return "", false
		}
	}
	var out bytes.Buffer
	if err := formatter.Format(&out, kanagawa, chroma.Literator(tokens...)); err != nil {
		return "", false
	}
	return out.String(), true
}
