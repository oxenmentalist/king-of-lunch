# Third-party notices

KING OF LUNCH uses the following open-source components. Their original license texts are included in `licenses/` and copied into the application bundle at build time.

| Component | Version | License | Included notice |
| --- | --- | --- | --- |
| [Goldmark](https://github.com/yuin/goldmark) | 1.8.6 | MIT | [goldmark.txt](licenses/goldmark.txt) |
| [Chroma](https://github.com/alecthomas/chroma) | 2.27.0 | MIT | [chroma.txt](licenses/chroma.txt) |
| [regexp2](https://github.com/dlclark/regexp2) | 2.2.1 | MIT | [regexp2.txt](licenses/regexp2.txt) |
| [Kanagawa](https://github.com/rebelot/kanagawa.nvim/tree/bb85e4bfc8d89b0e62c8fa53ccdd13d12e2f77b3) color palette | `bb85e4bfc8d89b0e62c8fa53ccdd13d12e2f77b3` | MIT | [kanagawa.txt](licenses/kanagawa.txt) |
| [Go runtime and standard library](https://go.dev/) | Selected local build toolchain; initially verified with 1.26.5 | BSD 3-Clause | [go.txt](licenses/go.txt) |

The Kanagawa Wave palette is adapted for a reading surface, including brighter comment text for readability. Chroma's upstream `COPYING` is preserved in full, including its font notice; KING OF LUNCH uses Chroma's HTML formatter and system fonts, and does not link its SVG formatter or bundled font.

Apple's AppKit and WebKit are system frameworks supplied by macOS and are not bundled with the application. KING OF LUNCH is licensed under the [MIT License](LICENSE). Third-party components retain their respective licenses listed above.
