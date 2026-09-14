<p align="center">
  <img src="assets/king-of-lunch-logo.png" alt="KING OF LUNCH — a crowned Markdown document" width="320">
</p>

# KING OF LUNCH

A small, fast Markdown viewer for macOS. Open a file, read it comfortably, and get back to work.

**Status: planning.** The app and command below are not implemented yet. See [plan.md](plan.md) for the proposed approach and review notes.

## The experience

Open a Markdown file from your terminal:

```sh
kol README.md
kol "notes/lunch plans.md"
```

KING OF LUNCH will open a native Mac window with a clean rendering in the **Kanagawa Wave** dark palette. Headings, lists, links, images, tables, and syntax-colored code blocks will be readable without configuration. Long documents scroll vertically; wide tables and code blocks scroll horizontally without clipping their content.

| Shortcut | Action |
| --- | --- |
| ⌘R | Reload the file from disk |
| ⌘+ | Increase font size |
| ⌘− | Decrease font size |
| ⌘0 | Reset font size |

Reload is deliberate: files will not be watched for changes. This is a viewer, with no editing, workspace, account, or background service.

## Open from Finder

The planned app will support opening `.md` files directly from Finder. Once a release is installed, select a Markdown file, choose **Get Info → Open with → KING OF LUNCH → Change All…** to make it your default viewer. The app will not change your default automatically.

## Installation

No release is available yet. The intended distribution is `KING OF LUNCH.app` with a small native `kol` command-line launcher. End users will not need Go, Node.js, or developer tools. Installation and build instructions will be added when they have been tested.

Colors are based on [Kanagawa by rebelot](https://github.com/rebelot/kanagawa.nvim).
