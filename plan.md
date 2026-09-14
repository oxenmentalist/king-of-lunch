# KING OF LUNCH implementation plan

Status: implementation authorized and built. Go plus Apple Command Line Tools was accepted during review. This document preserves the design and acceptance criteria; measured results and platform coverage are recorded in [docs/validation.md](docs/validation.md).

## Outcome and scope

Build an extremely lightweight, read-only Markdown viewer for macOS. `kol <filename>` opens a rendered document in a native window. Finder can open `.md` files with the app, and the user can select it as their default handler. ⌘R rereads the file; ⌘+, ⌘−, and ⌘0 adjust font size. Rendering includes tables and syntax-colored fenced code blocks, with vertical and horizontal scrolling wherever needed.

No editing, file watching, sidebar, tabs, library, plugins, cloud features, embedded terminal, search feature, export, or preferences window in v1. Standard selection, copy, window controls, and menus remain available.

## Recommended tools

**Go + a thin Objective-C/cgo bridge to AppKit and system WKWebView.** Go is the agreed application language; the owner has Go and Apple Command Line Tools on every target Mac. Personal use and local source builds are the primary delivery path. This is a native compiled Mac application with a WebKit document surface. Finder integration requires an `.app` bundle containing its executable, metadata, icon, and assets, accompanied by a small native `kol` launcher.

| Tool | Purpose and rationale |
| --- | --- |
| Go | File handling, document state, Markdown rendering, CLI logic, and tests using the owner's existing toolchain. |
| Objective-C through cgo + AppKit | A small project-owned bridge for windows, menu actions, file-open events, and application lifecycle. Keep application logic in Go and expose a narrow C API. |
| WKWebView | System HTML/CSS layout for tables, code, links, selection, accessibility, and overflow. No bundled browser engine or local HTTP server. |
| Goldmark | Go Markdown parser and HTML renderer with GFM extensions for tables, strikethrough, autolinks, and task lists. Pin a maintained version in `go.mod`/`go.sum`. |
| Chroma | Go syntax highlighting that emits HTML and CSS before display, with explicit Kanagawa token mapping. No JavaScript highlighter, CDN, or Node toolchain. Measure linked lexer size in the spike. |
| Go modules, `go build`, and a small Makefile/packaging script | Build with cgo and Apple's Clang/SDK, embed rendering assets with `go:embed`, assemble the app bundle, and apply local ad-hoc signing as needed. No Swift, Xcode project, or CMake dependency. |
| `go test`, Go benchmarks, and macOS profiling tools | Focused parser/state tests, native integration checks, startup timing, and aggregate memory measurement. Instruments is optional if full Xcode is available; the normal build requires only Command Line Tools. |

**Toolchain tradeoff.** Swift would make macOS calls more direct, but Go better fits the owner's existing workflow. The cost is a small native bridge, explicit thread/memory ownership across cgo, and Go runtime overhead. This is not a pure-Go build: `CGO_ENABLED=1` and Apple's Command Line Tools are required on each build Mac. Local native builds are the default; cross-compilation is not a v1 goal. No Apple developer account or paid membership is required for this local workflow.

**Why not NSTextView?** It could reduce rendering-process overhead, but polished GFM tables, rich layout, and highlighting would require more custom work. WebKit is the preferred starting point, conditional on the performance gate below. Electron is outside the lightweight brief; a cross-platform framework adds little value for a Mac-only app.

## Reading surface

The audience is someone opening local notes or project documentation from Terminal or Finder. The document is the focal point. Use a standard resizable window, filename in the title bar, and native menus; no persistent toolbar is necessary. Keep the supplied logo in documentation and derive a properly sized app icon during packaging, rather than placing branding above every document.

Use **Kanagawa Wave** consistently, with system UI text and a system monospace font for code. Starting tokens: background `#1F1F28`, foreground `#DCD7BA`, code background `#16161D`, links/functions `#7E9CD8`, strings `#98BB6C`, keywords `#957FB8`, and numbers `#D27E99`. Verify contrast and token mapping on real rendered content before release. Use comfortable margins, a readable prose measure, clear heading levels, and subtle table separators; preserve native focus indicators and selectable text.

Prose wraps to the available width. Code retains whitespace and does not wrap by default. Wide code blocks and tables get horizontal scroll containers, while the document scrolls vertically. Ensure any remaining oversized content is reachable rather than clipped. Respect macOS scrollbar preferences and trackpad gestures. Verify keyboard access to scrollable regions and VoiceOver reading order.

## Behavior and architecture

1. **Opening.** A single file-open path handles CLI and Finder URLs. One window per file; reopening the same normalized file URL focuses its existing window. Distinct files get distinct windows. Launching the app without a file shows the native Open dialog; ⌘O can open another document.
2. **CLI.** A small Go launcher using the native bridge resolves the argument against the caller's working directory, validates that it is a readable regular file, and opens it with this app through `NSWorkspace`. Use file URLs rather than shell command interpolation. Support spaces, Unicode, absolute/relative paths, and `--` for names beginning with a dash. Invalid arguments, missing app, or unreadable paths produce a concise stderr message and nonzero status. Success means the open request was accepted, not that asynchronous rendering finished. Do not route through the current default `.md` app. Locate this app by bundle identifier, with an explicit packaged-location fallback.
3. **State.** Go owns each document’s file URL, zoom, and render generation; the native window controller owns the WebKit view. Lock the startup goroutine to the main OS thread before entering AppKit. Read, parse, and highlight in Go workers; dispatch UI changes to the main queue. Pass copied strings and opaque numeric IDs across the bridge, never retained Go pointers. Define allocation/free and callback ownership explicitly, keep native callbacks nonblocking, and discard stale results after reload or window closure.
4. **Rendering.** Read UTF-8, tolerating a BOM and ordinary line-ending differences; clearly report unsupported encoding. Parse with GFM extensions, generate a fixed HTML shell and theme CSS, and highlight recognized language fences. Initial languages: Bash/shell, C, C++, CSS, Go, HTML/XML, JavaScript, JSON, Markdown, Python, Rust, SQL, Swift, TypeScript, and YAML. Unknown or unlabelled fences remain legible plain code; avoid costly automatic language guessing.
5. **Reload.** ⌘R explicitly rereads disk and refreshes relative images. Preserve zoom and restore the closest practical scroll position, clamped when the document shrinks. An empty file is a valid empty document. If reload fails, retain the last successful rendering and show a native error identifying the file. Do not install filesystem watchers or polling timers.
6. **Zoom.** Menu actions apply to the focused document even when the web view has keyboard focus. Start at 16 px body text, use 2 px steps with 10–32 px bounds, and reset to 16 px. Scale headings, tables, and code proportionally using relative units, rather than page magnification. Accept ⌘= as well as ⌘+ on applicable keyboard layouts. Zoom is per-window and lasts for that window's lifetime; persistence is outside v1.
7. **Finder.** Declare Markdown document types with Viewer role and an appropriate imported type conforming to plain text. Verify existing system Markdown identifiers before finalizing metadata. Handle open events both at cold launch and while running. Register as an eligible handler without claiming exclusive ownership or changing user defaults.

## Local content handling

Documents must not execute scripts. Disable raw HTML in the Markdown parser for v1, escape generated attributes, reject unsafe URL schemes, and apply a restrictive content security policy. Highlight in Go before loading HTML; disable document JavaScript and expose no page-to-native scripting bridge. App-owned zoom, anchor navigation, scroll restoration, and horizontal keyboard/wheel controls execute in an isolated WebKit content world. These fixed controls are bundled with the app; Markdown cannot supply scripts. Raw HTML, including HTML tables and embedded widgets, is intentionally outside the supported Markdown subset and should be documented in the release README.

Resolve relative images against the document directory with bounded file access, including symlink containment checks. Start with images inside that directory or its descendants; parent-directory images are an explicit limitation to validate during the spike. Block remote image loads in v1 so opening a document remains offline. Preserve internal anchor navigation, open clicked HTTP(S) links in the default browser, and route clicked local Markdown links through the app's open path. Other local targets and schemes remain blocked. Missing or blocked images should preserve useful alt text. Validate the WKWebView loading strategy and resource boundary before building the rest of the viewer.

## Delivery and size

Proposed baseline: macOS 13+, first measured on Apple silicon. Produce an arm64 app initially; add a universal arm64/x86_64 artifact only after testing on both architectures. The minimum OS and Intel support are reviewable assumptions, not established requirements.

Build locally with Go and Apple Command Line Tools. Provide a single documented build command that compiles, packages `KING OF LUNCH.app` plus the native `kol` launcher, and performs ad-hoc signing as needed without an Apple account. Provide an explicit install target with configurable app and CLI destinations, defaulting to user-writable locations such as `~/Applications` and `~/.local/bin`. Do not silently modify shell profiles or default file associations. Plain `go build` can compile the executable, but bundle assembly is also required for the supported Finder installation; do not promise `go install` alone as the complete installation flow.

GitHub source hosting and local builds are primary. Downloadable ZIP binaries can be published through GitHub Releases later; running those binaries requires no Go toolchain. Developer ID signing and notarization are optional future release work for smoother downloaded-app launching, not a prerequisite for development, personal installation, or publishing release assets. Unnotarized downloads may require a Gatekeeper override. Use a stable bundle identifier for local installation as well as distribution. Preserve upstream licenses and pin dependency versions; the owner selected the MIT License for the project.

Small download size does not imply small runtime memory: WebKit uses helper processes. Initial engineering targets, **not measured promises**, are an uncompressed arm64 app plus launcher under 15 MiB, cold first render of a 100 KiB fixture under 500 ms, warm open/reload under 150 ms, and combined app/WebKit idle memory under 150 MiB for that fixture. Record hardware, OS, release-build settings, first-run conditions, median and p95 over repeated runs, and all related processes. Check for near-zero idle CPU and no retained document views after repeated open/close cycles. If targets fail, report the evidence and revisit the renderer before feature work expands.

## Implementation sequence and acceptance

1. **Rendering and performance spike.** Build the minimal Go/cgo/AppKit bridge and one window rendering a fixture with prose, a wide table, highlighted code, and a local image. Exercise CSS font zoom and restricted resource loading. Measure the release bundle, cold/warm timing, and full WebKit memory footprint. Gate: acceptable rendering and measured lightweight behavior; otherwise revisit architecture with the owner.
2. **Document lifecycle and controls.** Add the common open path, reload, per-window zoom, native menus, error handling, and multiple-window behavior. Gate: all required shortcuts work with content focused; reload preserves zoom and does not silently update before ⌘R.
3. **CLI and Finder integration.** Build the launcher and bundle metadata; test a fresh app launch and an already-running app, filename edge cases, and user-selected default handling. Gate: both opening paths reach the correct document and CLI errors are actionable.
4. **Rendering verification and packaging.** Finish palette/highlight mapping, scrolling, icon generation, and reproducible release packaging. Test the source-build and user-local install workflow using only Go and Command Line Tools, without developer credentials. Add installation instructions only after testing them. Gate: the acceptance matrix below passes and performance remains within agreed targets.

| Area | Evidence required |
| --- | --- |
| Markdown | Fixtures for headings, emphasis, nested lists, blockquotes, links, images, task lists, aligned tables, fenced/indented code, Unicode, and malformed syntax; parser/output assertions for structure and escaping. |
| Code and overflow | Known language colors, unknown language fallback, long unbroken lines, wide tables, narrow windows, and both font-size bounds; inspect rendered output and keyboard/trackpad scrolling. |
| Files and reload | Empty/missing/unreadable/deleted/replaced files, invalid encoding, rapid reloads, multiple windows, and preservation of the last successful rendering on failure. |
| Integration | CLI from another working directory; paths with spaces, Unicode, and leading dashes; Finder cold/warm opens; default-handler selection on a test file; no default changed during installation. |
| Content boundaries | Raw script HTML, unsafe links, remote images, escaping paths, and symlinks cannot execute or fetch unintended content; permitted local images and external browser links work. |
| Lightweight behavior | Release measurements for a typical 100 KiB document plus 1 MiB and 10 MiB stress fixtures; bound highlighting work on oversized blocks and retain readable plain code if it exceeds the budget. |
| Native usability | Selection/copy, focused-window shortcuts, menu enabled states, VoiceOver, resize, and correct scrollbar behavior. |

## Self-review

- All requested features map to an implementation step and acceptance check. The application, source-build scripts, fixtures, and native diagnostic are now implemented.
- The key tradeoff is explicit: Go fits the personal source-build workflow; a small cgo/AppKit bridge supplies native integration; system WebKit gives reliable rich layout but adds process/memory overhead. Go runtime and linked highlighter size must also be measured. The first milestone measures whether that tradeoff satisfies “extremely lightweight.”
- Packaging reconciles the native-binary request with Finder's app-bundle requirements. The launcher hands off via native APIs, avoiding shell quoting and wrong-default-app failures.
- Rendering scope is bounded: GFM tables and common code languages are included; arbitrary HTML, remote images, and images outside the document directory are excluded initially. These limitations should be reviewed against expected documents.
- No numerical performance claim is presented as tested. The minimum OS, architecture support, default font sizing, dependency selection, and bundle identifier were originally proposed as stated above. The project is now licensed under MIT.
- Go plus Apple Command Line Tools is the implemented toolchain. Goldmark 1.8.6 and Chroma 2.27.0 are pinned. Local images are embedded into HTML after bounded reads, so the WebKit view receives no filesystem access. A private document base URL enables internal anchors without a server, and app-private link URLs route local Markdown clicks through the native delegate. Windows share a nonpersistent WebKit data store to reduce warm-open overhead. Reload work is coalesced per document and limited to two active renderers across the app.
- Implementation adds a 64 MiB document limit, 8 MiB/24-megapixel individual image limits, and a 24 MiB total image budget. Large or slow code blocks fall back to plain text. These bounds preserve responsiveness on accidental oversized input.
- Public signing/notarization and Intel/older-macOS runtime testing remain outside the completed local Apple-silicon validation.
- The implementation was retained after measurement: the app plus launcher is about 9.7 MiB; representative combined physical footprint was 101 MB. Earlier median cold and warm opens reached the initial targets, while observed tail samples and the final warm-open median were slower. These measurements support the chosen architecture; the timing variation remains a documented performance limitation.

## Technical references

- [Apple: WKWebView](https://developer.apple.com/documentation/webkit/wkwebview) — system document rendering surface.
- [Apple: application file-open events](https://developer.apple.com/documentation/appkit/nsapplicationdelegate/application(_:open:)) — Finder and running-app URL delivery.
- [Apple: CFBundleDocumentTypes](https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundledocumenttypes) — declaring supported document types.
- [Goldmark](https://github.com/yuin/goldmark) and [GFM specification](https://github.github.io/gfm/) — Go parser, HTML output, and Markdown extensions.
- [Chroma](https://github.com/alecthomas/chroma) — Go syntax highlighting and HTML formatting.
- [Kanagawa palette](https://github.com/rebelot/kanagawa.nvim/blob/master/lua/kanagawa/colors.lua) — source color values.
