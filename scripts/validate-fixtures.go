// Run with: go run scripts/validate-fixtures.go [output-directory]
// Produces independent acceptance fixtures without adding large files to Git.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/validate-fixtures.go [output-directory]")
		os.Exit(2)
	}
	var dir string
	var err error
	if len(os.Args) == 2 {
		dir, err = filepath.Abs(os.Args[1])
		must(err)
		must(os.MkdirAll(dir, 0700))
	} else {
		dir, err = os.MkdirTemp("", "kol-validation-")
		must(err)
	}
	must(os.MkdirAll(filepath.Join(dir, "documents", "images"), 0700))
	must(os.MkdirAll(filepath.Join(dir, "outside"), 0700))
	write := func(name string, contents []byte) {
		must(os.WriteFile(filepath.Join(dir, name), contents, 0600))
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jBhcAAAAASUVORK5CYII=")
	must(err)
	write("documents/images/allowed.png", png)
	write("outside/forbidden.png", png)
	must(os.Symlink(filepath.Join(dir, "outside", "forbidden.png"), filepath.Join(dir, "documents", "images", "escape.png")))
	write("documents/next document.md", []byte("# Linked document\n\nLocal Markdown navigation worked.\n"))
	write("documents/empty.md", nil)
	write("documents/invalid-utf8.md", []byte{'#', ' ', 0xff, 0xfe, '\n'})
	write("documents/bom-crlf.md", append([]byte{0xef, 0xbb, 0xbf}, []byte("# BOM and CRLF\r\n\r\nUnicode: 日本語 café 👑\r\n")...))
	write("documents/unreadable.md", []byte("# This file must be unreadable\n"))
	must(os.Chmod(filepath.Join(dir, "documents", "unreadable.md"), 0000))
	for _, name := range []string{"lunch plans 日本語.md", "-leading-dash.md", "literal $(touch SHOULD_NOT_EXIST).md", "quote'and\"double.md", "line\nbreak.md"} {
		write(filepath.Join("documents", name), []byte("# Filename edge case\n\n"+name+"\n"))
	}
	write("documents/acceptance.md", []byte(`# Independent acceptance document

Readable prose with **bold**, *emphasis*, ~~strikethrough~~, inline `+"`code`"+`, Unicode 日本語 café 👑, and <https://example.com>.

## Lists and quote

- Parent item
  - Nested child
- [x] Completed task
- [ ] Pending task

1. First
2. Second

> A block quote with **emphasis**.

## Links and local image

[Same-page anchor](#syntax-and-overflow), [next document](next%20document.md), [external browser](https://example.com), [blocked application](javascript:alert(1)), [blocked file](images/allowed.png).

![Allowed local image](images/allowed.png)
![Blocked remote image](https://127.0.0.1:9/should-never-load.png)
![Blocked parent image](../outside/forbidden.png)
![Blocked symlink image](images/escape.png)

## Syntax and overflow

`+"```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\t// Colored keyword, string, number, and comment.\n\tfmt.Println(\"Lunch\", 42)\n}\n```"+`

`+"```unknown-language\n<literal> & \"unknown\" should stay readable, with no guessed lexer\n```"+`

`+"```\n"+strings.Repeat("long_unbroken_line_", 120)+"\n```"+`

| Left | Center | Right | Wide column |
| :--- | :----: | ----: | ----------- |
| Alphabet | beta | 123 | `+strings.Repeat("wide_table_cell_", 80)+` |
| Second | Middle | 456 | final |

    indented <code> & text

---

### Raw HTML must not execute

<script>document.title = "UNSAFE SCRIPT EXECUTED"; fetch("https://127.0.0.1:9/unsafe")</script>
<img src="https://127.0.0.1:9/raw.png" onerror="document.title='UNSAFE EVENT EXECUTED'">
<iframe src="https://example.com"></iframe>

Unclosed *emphasis and [malformed](, followed by normal prose.
`))
	paragraph := "## Reading sample\n\nA modest paragraph with **emphasis**, an [internal link](#reading-sample), and `inline code`. This fixture repeats prose to exercise long-document layout without expensive highlighting.\n\n"
	for _, size := range []int{100 * 1024, 1024 * 1024, 10 * 1024 * 1024} {
		contents := strings.Repeat(paragraph, size/len(paragraph)+1)[:size]
		write(fmt.Sprintf("documents/prose-%d.md", size), []byte(contents))
	}
	write("documents/oversized-code.md", []byte("# Oversized code fence\n\n```go\n"+strings.Repeat("func lunch() { println(\"keep readable\") }\n", 30000)+"```\n"))
	must(json.NewEncoder(os.Stdout).Encode(map[string]string{"directory": dir, "documents": filepath.Join(dir, "documents"), "acceptance": filepath.Join(dir, "documents", "acceptance.md")}))
}
