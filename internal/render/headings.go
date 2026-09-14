package render

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
)

// Goldmark's default ID collection restarts at suffix 1 for every duplicate
// heading, making repetitive large documents quadratic. Keep the next suffix
// per base so duplicate headings cost constant amortized work. Unicode letters
// also remain useful, linkable anchors rather than collapsing to "heading".
type headingIDs struct {
	used map[string]bool
	next map[string]int
}

func newHeadingIDs() *headingIDs {
	return &headingIDs{used: make(map[string]bool), next: make(map[string]int)}
}

func (ids *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	var slug strings.Builder
	for _, r := range strings.TrimSpace(string(value)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			slug.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r) || r == '-' || r == '_':
			slug.WriteByte('-')
		}
	}
	base := slug.String()
	if base == "" {
		base = "heading"
		if kind != ast.KindHeading {
			base = "id"
		}
	}
	if !ids.used[base] {
		ids.used[base] = true
		ids.next[base] = 1
		return []byte(base)
	}
	for i := max(ids.next[base], 1); ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if !ids.used[candidate] {
			ids.used[candidate] = true
			ids.next[base] = i + 1
			return []byte(candidate)
		}
	}
}

func (ids *headingIDs) Put(value []byte) { ids.used[string(value)] = true }
