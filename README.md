![KING OF LUNCH — a crowned Markdown document](assets/king-of-lunch-logo.png)

# KING OF LUNCH

A small, fast Markdown viewer for macOS. Open a file, read it comfortably, and get back to work.

```sh
kol README.md
kol "notes/lunch plans.md"
```

KING OF LUNCH opens a native Mac window with a clean rendering in the **Kanagawa Wave** dark palette. It renders headings, lists, links, local images, tables, and syntax-colored code blocks. Long documents scroll vertically; wide tables and code blocks have their own horizontal scrolling regions. Text is selectable and copyable.

This is a personal, read-only tool: no editing, file watching, account, background service, or configuration window. Reload is deliberate.

## Build and install

Requires **macOS 13 or later**, **Go 1.25 or later**, and **Apple Command Line Tools**. No Apple developer account, paid membership, full Xcode installation, or Node.js is needed. Apple silicon is tested; the build script also supports native Intel builds, which have not yet been tested.

From this repository:

```sh
make build
./build/kol testdata/showcase.md
```

The build produces `build/KING OF LUNCH.app` and `build/kol`, with a local ad-hoc signature. Dependencies are pinned in `go.mod` and `go.sum`; the first build needs network access to download them. The app itself reads offline.

Install the app and launcher in your home directory:

```sh
make install
~/.local/bin/kol README.md
```

The default destinations are `~/Applications/KING OF LUNCH.app` and `~/.local/bin/kol`. The installer also writes `kol.app-path` alongside the launcher so it can locate the app. Keep that file beside `kol`. If `~/.local/bin` is on your PATH, use `kol` directly; otherwise add it using your usual shell configuration. The installer does not edit shell files or change your default file associations.

Destinations are configurable:

```sh
make install APP_DIR="$HOME/Applications" BIN_DIR="$HOME/bin"
```

Use `make install` again to update. Quit the running app before replacing it. To uninstall, remove the app, the launcher, and its `kol.app-path` file from the destinations you selected.

## Controls

| Shortcut | Action |
| --- | --- |
| ⌘O | Open a Markdown file |
| ⌘R | Reload the focused file from disk |
| ⌘+ or ⌘= | Increase font size |
| ⌘− | Decrease font size |
| ⌘0 | Reset font size |
| ⌘C / ⌘A | Copy / select all |
| ⌘W / ⌘Q | Close the window / quit |
| Ctrl-U / Ctrl-D | Scroll up / down half a window |
| G (Shift-G) | Go to the bottom of the document |
| gg | Go to the top of the document |
| :/ | Open document search (type `:` then `/`) |
| Enter / Shift-Enter | Find forward / backward and close search |
| n / N (Shift-N) | Find next / previous match |
| Esc | Clear the search term and selection, and close search |

Search is literal, case-insensitive, and wraps at either end of the document. Matches are selected and scrolled into view. Each window remembers its search until cleared with Esc, reloaded, or closed. Esc also clears search while the search bar is closed.

Font size ranges from 10–32 px in 2 px steps, starting at 16 px. Each window keeps its own zoom until closed. Reload preserves zoom and the nearest available scroll position. If the file cannot be read, the previous successful rendering stays visible and an error explains what happened.

Click a wide table or code block, then use the left/right arrow keys to scroll it. Horizontal wheel gestures and Shift-wheel also scroll within that region.

Open several documents in separate windows. Opening the same normalized path again focuses its existing window. Paths with spaces and Unicode work; use `kol -- -notes.md` for a filename beginning with a dash.

## Open from Finder

After installation, use **Open With → KING OF LUNCH** on a `.md` or `.markdown` file. To make it your default viewer, select a Markdown file, choose **Get Info → Open with → KING OF LUNCH → Change All…**. The app does not change your default automatically.

## Markdown support

CommonMark with GitHub-style tables, strikethrough, task lists, and autolinks. Headings can be linked with `#heading-name`, including links to headings in another file. Local Markdown links open in the viewer; clicked HTTP(S) links open in your default browser.

Fenced code recognizes Bash/shell, C, C++, CSS, Go, HTML/XML, JavaScript, JSON, Markdown, Python, Rust, SQL, Swift, TypeScript, and YAML, including common aliases. Unknown or unlabelled fences remain plain code. Blocks over 64 KiB and blocks that exceed the document's highlighting budget remain readable without highlighting.

Local PNG, JPEG, and GIF images are supported inside the document's directory or its descendants. Missing or blocked images retain their alt text. Remote images, images outside that directory, SVG, arbitrary raw HTML, embedded scripts, and other link schemes are excluded. Images are bounded to 8 MiB and 24 megapixels each, with 24 MiB of image data per document. Markdown files must be UTF-8 (a BOM is accepted), with a 64 MiB file limit.

## Verification and packaging

```sh
make test
make check
make package
```

`make check` runs the separate native diagnostic in a graphical macOS session. It opens temporary documents and tests the actual AppKit/WebKit view, including reload, font zoom, local images, navigation, scrolling, and large files. It is not shipped with the app. See [the validation report](docs/validation.md) for measured results and remaining platform coverage.

`make package` writes a versioned ZIP to `dist/`, suitable for a GitHub Release. The current build uses ad-hoc signing for personal use. Downloaded builds may trigger Gatekeeper; Developer ID signing and notarization are optional future distribution work.

To use an extracted ZIP, keep `kol` beside `KING OF LUNCH.app` and run `./kol <filename>`, or open the app directly. Running a packaged build does not require Go or Command Line Tools.

The implementation uses Go, a thin Objective-C/cgo bridge to AppKit, system WebKit, Goldmark, and Chroma. No browser engine is bundled. Colors are based on [Kanagawa by rebelot](https://github.com/rebelot/kanagawa.nvim); dependency licenses are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). See [plan.md](plan.md) for the original design and implementation decisions.

Licensed under the [MIT License](LICENSE). Copyright © 2026 oxenmentalist.
