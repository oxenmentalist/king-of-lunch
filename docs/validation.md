# Independent validation

This report records measured behavior for KING OF LUNCH. The final independent native suite passed **47 checks**, including installed horizontal input handlers, Unicode cross-document anchors, actual WKWebView navigation, and 1 MiB/10 MiB stress documents. Every closed test window was followed through native controller deallocation; a separate lifetime run also confirmed seven such releases. The scope of desktop interaction checks is stated explicitly below.

## Theme settings validation (2026-09-15)

The theme update passed **105 native assertions**, including 23 theme assertions, five repeated opens/closes, and the 1 MiB/10 MiB stress fixtures. It also passed Go race tests, `go vet`, the signed app build, and renderer tests for complete light syntax overrides and a minimum 4.5:1 contrast ratio for light code tokens.

The native suite exercises the actual View → Theme actions, verifies exclusive menu checkmarks and saved preferences, and checks the computed document and syntax colors in existing and newly opened WebKit windows. Switching themes preserves submitted search, DOM selection, scroll, and font zoom; reload retains appearance. System mode leaves the application appearance unset. Diagnostic window appearance changes exercise live light/dark inheritance without changing the user's OS setting; actual OS schedule changes were not tested.

A desktop pass inspected the light and dark reading surfaces and native title bars, selected Dark through the menu, quit and relaunched the built app, and confirmed that Dark persisted. The app was then returned to System. Testing remains limited to the macOS/Apple silicon environment below.

After integration with printing PR #1, `make test` and `make check` passed all **109 native assertions**. Printing from explicit light and dark themes produced 52-page PDFs while retaining each on-screen palette and zoom. Renderer coverage verifies that paper syntax rules override both screen palettes.

## Environment

- macOS 26.6.2 (25G83), Apple M3 Pro (`arm64`), 36 GiB RAM.
- Go 1.26.5, Apple Command Line Tools Clang.
- Local source build and ad-hoc signing; no Apple developer credentials.

## Reproducible fixtures

Run `go run scripts/validate-fixtures.go` to create an isolated temporary fixture directory. The generator prints a JSON object with the paths. It includes normal GFM, raw HTML and unsafe URLs, permitted and blocked image paths, whitespace/Unicode/leading-dash filename cases, BOM/CRLF, empty and invalid UTF-8 files, an unreadable file, 100 KiB/1 MiB/10 MiB prose, and an oversized code fence. The generator does not modify the repository or launch the app. Pass a new empty output directory to retain fixtures at a chosen location.

The separate developer executable `cmd/kol-check` runs the real AppKit/WKWebView application, reads its rendered DOM through an isolated content world, and drives the same reload/zoom actions as the menus. It creates and modifies only its own temporary files. Build it with `go build -trimpath -ldflags='-s -w' -o /tmp/kol-check ./cmd/kol-check` and run `/tmp/kol-check -stress` in a graphical macOS session. Results are JSON on stdout; progress is on stderr. `-startup-only` runs initial rendering checks and exits; `-lifecycle-only` checks repeated native controller deallocation; `-idle-seconds 30` pauses with a representative 100 KiB document for external process profiling. This diagnostic is not included in the installed app or release ZIP.

## Acceptance matrix

| Area | Independent check | Result |
| --- | --- | --- |
| Source build | Clean package build with Go + Command Line Tools; bundle metadata and signature | Passed; final race tests, go vet, and strict signatures also passed; packaging agent verified custom install/reinstall and ZIP extraction |
| CLI failures | No/extra argument, unknown option, leading dash without `--`, missing path, directory, unreadable file, named pipe, help | All 10 actual subprocess checks passed |
| CLI paths | Different working directory; spaces, Unicode, leading dash, quotes, shell substitution characters | All 3 packaged-launcher subprocess cases accepted with exit 0 and empty stderr; source uses native URLs, with no shell interpretation |
| Finder / Launch Services | Document registration; repeated file focuses one window; distinct files open independently | Live Finder opens passed both cold and warm; registration/default-preservation checks and native duplicate/symlink opens passed; manually changing the system-wide default was not exercised |
| Markdown | GFM structure, Unicode, known and unknown code fences | Actual WKWebView checks passed, including nested lists, task lists, quote, table, BOM/CRLF and empty files |
| Content boundary | HTML/script suppression, blocked schemes and network images, image directory and symlink containment | Actual WKWebView checks passed; allowed image loaded, outside/symlink images did not |
| Native view | Actual WKWebView DOM, image loading, colors, vertical and horizontal overflow | Passed, including narrow window, table alignment, and 7 code-token colors; desktop check observed vertical scrolling and native Right-arrow scrolling of table and code |
| Horizontal input | Installed handlers for arrows, horizontal wheel, Shift-wheel, modifiers, and boundaries | All 12 assertions passed across table/code; ordinary vertical events and modified arrows remain unhandled, and boundary input is not prevented |
| Links | Same-page anchors and local Markdown links use native navigation policy | Passed, including percent-encoded Unicode anchors in newly opened and already-open documents |
| Reload | Disk changes only on explicit reload; zoom and scroll preserved; failed reload retains prior contents | Passed; scroll restored exactly from 550 px to 550 px; invalid UTF-8 and deleted-file errors retained prior contents |
| Zoom | Native actions; per-window state; 10–32 px limits; 16 px reset | Native action checks passed; desktop check independently observed ⌘+, ⌘−, ⌘=, and ⌘0, plus ⌘R preserving scroll |
| Native file dialog | Open dialog and standard text-editing commands | Live Open dialog and Edit → Cut/Paste checks passed |
| Lifecycle | Rapid reloads and repeated opens/closes | Seven open/close cycles, duplicate and symlink opens, and eight rapid reloads passed; separate run observed deallocation of all seven closed controllers |
| Performance | Release size; 100 KiB render timing and memory; 1 MiB/10 MiB stress; idle CPU | Measured below; startup tail and warm-open tail exceed initial engineering targets |
| Accessibility | Keyboard access to overflow; native menus; VoiceOver order | Native accessibility tree exposed headings, table, and focused overflow region; full VoiceOver audio/navigation was not exercised |

## Observations and limitations

The validation agent operates independently of production implementation; native UI checks are coordinated to avoid competing with another agent controlling the desktop.

Independent source review caught two production issues before completion: Objective-C lifetime handling needed ARC and autorelease pools, and opening a named pipe before checking file type could block a renderer. The implementation now uses ARC with explicit property ownership and bounded autorelease pools, and opens files/images nonblocking before rejecting non-regular files. Regression tests cover named-pipe Markdown and image targets. Review also prompted coalescing rapid reloads per document and limiting active rendering workers to two across the app.

One diagnostic run lost application focus while external profiling tools requested access. Its test-only action hook relied on the key window and therefore stopped delivering commands. The hook now passes the selected document explicitly through the same action handlers; normal keyboard/menu behavior remains subject to the separate native UI check. An earlier fixture path assertion also needed canonicalization from `/var` to `/private/var`; this was a diagnostic assertion issue, not an application path error.

Actual navigation checks caught WebKit-origin issues that HTML string tests could not detect: same-page anchors needed a stable private base URL, and `file://` links from that origin were blocked before the application received a navigation callback. The implementation now handles anchors in its isolated control world and routes local Markdown links through a private `kol-file` URL. Both paths passed the final native suite without granting WebKit filesystem access.

Horizontal scrolling was verified independently of the overflow CSS: a table container measured 668 px wide with 11,438 px content, and a code block measured 670 px wide with 16,574 px content. Both accepted focus and changed `scrollLeft` from 0 to 150 px. The final app adds fixed controls in an isolated `WKUserScript`, while document JavaScript remains disabled. Actual desktop Right-arrow presses moved both the table and wide code block, with moved content and native scrollbars visible. The additional alignment selectors are compatibility hardening; Goldmark's current inline alignment styles already worked.

The diagnostic independently dispatched keyboard and wheel events inside the actual isolated WebKit world to exercise the installed handlers. For both table and code, Right moved 48 px, Left returned, and Shift/Cmd/Ctrl/Option-modified arrows preserved the current selection without preventing default handling. Horizontal wheel and Shift-wheel moved content; ordinary vertical wheel events remained unhandled. Keys and wheel events at either horizontal boundary were not prevented. These checks establish handler behavior; physical trackpad input was not independently established. The desktop pass also verified ⌘+ (Command-Shift-equals) increasing the font, ⌘− decreasing it, and ⌘0 resetting it.

After the Mac was unlocked, the desktop pass completed cold and warm Finder opens, the native Open dialog, and Cut/Paste menu behavior. Testing covered this Apple silicon/macOS 26.6.2 host; macOS 13 itself, Intel hardware, and VoiceOver audio were not tested. No system-wide file default or security setting was changed.

## Measurements

The final source build measured an **8,180 KiB app bundle plus a 1,720 KiB launcher**, approximately **9.67 MiB combined**. Both Mach-O binaries declare macOS 13.0 minimum and SDK 26.5, and strict ad-hoc signature validation passed. Installing twice into temporary destinations containing spaces and Unicode preserved the launcher's absolute app-path sidecar and left `LSHandlers` defaults unchanged; the temporary registration was removed afterward. The release ZIP is regenerated from the final build and documentation.

For a rendered 100 KiB fixture, `footprint --noCategories` measured **101 MB combined physical footprint** across the diagnostic application and its newly launched WebContent, GPU, Networking, and audio helper processes. The individual readings were approximately 34 MB app, 44 MB WebContent, 14 MB GPU, 4.3 MB Networking, and 5.2 MB audio helper. Before/after process inventories identified these helpers. All five were at **0.0% CPU** in the idle snapshot. Summed RSS was approximately 229 MiB; RSS includes shared pages and is not the physical-footprint metric used for the 150 MiB engineering target. This was one representative idle sample, not a peak-memory or leak guarantee.

The baseline below used the stripped developer diagnostic before the final input-handler and cross-document-fragment additions. Its renderer and shared nonpersistent WebKit data store match the final app. The approximately 100 KiB fixture includes prose, lists, a small highlighted Go block, a very wide code line, a wide table, and local/blocked images. Warm opens use distinct files and close each completed window. Stress fixtures contain repeated Markdown paragraphs.

| Measurement | Samples | Median | p95 / observed maximum |
| --- | --- | ---: | ---: |
| Fresh process → first 100 KiB render | 419, 726, 511, 489, 405 ms | **489 ms** | **726 ms** |
| Warm 100 KiB open | 135, 147, 152, 150, 142, 150, 147 ms | **147 ms** | **152 ms** |
| 100 KiB reload | 1 | **44 ms** | — |
| 1 MiB stress render | 1 | **338 ms** | — |
| 10 MiB stress render | 1 | **2.83 s** | — |

Fresh-process timing comes from an external parent stopwatch starting before process creation and stopping at the first-render notification, so it includes Go startup and native setup. Each run creates fresh application/WebKit processes; filesystem and operating-system caches were already warm from building and prior use. The samples were consecutive on an otherwise normally running desktop, not a rebooted or isolated machine. p95 uses nearest rank; with five/seven samples it equals the observed maximum and is not a statistically strong tail estimate.

The final 47-check regression run also completed successfully with warm-open samples of 154, 186, 154, 164, 144, 167, and 185 ms: **164 ms median and 186 ms observed p95**. Reload remained **44 ms**; the 1 MiB and 10 MiB documents rendered in **435 ms** and **3.78 s**. Race tests and vet were also running during that final validation window, so these timings describe the loaded desktop rather than an isolated benchmark.

Size, representative physical footprint, and reload timing meet the initial engineering targets. The earlier fresh-process median was below 500 ms, but its 726 ms observed tail was above it. Warm opening varies around the 150 ms target: the shared-store baseline measured 147 ms median, while the final regression run measured 164 ms. These are measured limitations, not unqualified performance-pass claims. Both stress documents rendered successfully; stress-memory peaks were not measured.
