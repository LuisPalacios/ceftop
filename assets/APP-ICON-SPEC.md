# App icon spec — `assets/app-<key>.svg`

Every logo shown in the discovered-apps bar and in the tree header must follow this spec so the set looks like one family. The reference files are `app-chrome.svg`, `app-slack.svg`, and `app-docker-desktop.svg`; when in doubt, open one of them next to the new file.

## Canvas

| Property | Value |
|---|---|
| Root element | `<svg width="100%" height="100%" viewBox="0 0 150 150" xmlns="http://www.w3.org/2000/svg">` |
| Aspect ratio | 1 : 1 |
| Coordinate space | 150 × 150 user units |
| Safe area | 120 × 120 box at `x=15 y=15` (a 15-unit / 10 % margin on every side) |
| Background | none, transparent |

The artwork lives inside the safe area and touches it on its longest axis. A square mark fills the whole 120 × 120 box. A wide or tall mark spans 120 units on its longer side and is centered on the other. Nothing except a deliberately soft glow may cross into the 15-unit margin.

The reference files carry a hidden guide rectangle that marks the safe area. Keep it; it is invisible at runtime and makes later edits easier:

```xml
<rect width="120" height="120" x="15" y="15"
      style="fill: none; stroke-width: 0.2px; stroke: rgb(84, 84, 84); pointer-events: none; visibility: hidden;"/>
```

## Colors

The icons render on the app background `#0b1220` (very dark navy) and, when the app is not running, through a grayscale filter at 55 % opacity. Rules:

- Use the brand's official colors for the mark itself.
- Replace black or near-black fills (`#000000` … `#222222`) with white `#ffffff` or the brand's light variant. The ChatGPT mark in `app-chatgpt.svg` is white for this reason.
- Do not use dark navy, dark gray, or pure black anywhere in the mark. Anything below roughly 3 : 1 contrast against `#0b1220` disappears.
- No background tile behind the mark unless the tile is part of the brand (Slack, Chrome, and Docker use the bare mark; VS Code keeps its blue glyph without a tile).
- The icon must still read as a shape in grayscale. Do not rely on color alone to separate parts.

## Geometry and detail

The icon is displayed at 24 px in the tree header and about 21 px in the apps bar. Design for that size:

- No stroke or gap thinner than 2 user units (they vanish at 24 px).
- Prefer flat fills. Gradients are fine when they are part of the brand (Chrome, Edge).
- Drop drop-shadows, blurs, and filters unless the brand mark is unreadable without them.
- Convert any text to paths.

## File hygiene

- One `<svg>` root, UTF-8, with the XML declaration `<?xml version="1.0" encoding="utf-8"?>` on the first line.
- No embedded raster images, no external references, no `<script>`, no fonts.
- Target under 8 KB; simplify paths when a vendor SVG is heavier.
- Editor metadata (`xmlns:bx`, `<bx:grid>`) from Boxy SVG is tolerated but optional.

## Naming

Files are named `app-<key>.svg`. The key is the app's browser-process executable name, lowercased, with `.exe` removed and spaces turned into hyphens:

| Browser process | Key | File |
|---|---|---|
| `chrome.exe`, `Google Chrome` | `chrome` | `app-chrome.svg` |
| `Docker Desktop.exe` | `docker-desktop` | `app-docker-desktop.svg` |
| `Code.exe`, `Code` | `code` | `app-code.svg` |
| `msedgewebview2.exe` | `msedgewebview2` | `app-msedgewebview2.svg` |

Matching is fuzzy (see `pkg/icons`): case, separators, `.exe`, and platform words such as `for mac` or `win` are ignored, and a shorter key matches a longer name that contains it (`chrome` matches `Google Chrome`). Still use the full canonical key above so the file is self-describing. `app-default.svg` is reserved for the fallback icon.

## Placement

Drop the finished file in `assets/`. The frontend build copies every `assets/app-*.svg` into the bundled set; no code change or registry entry is needed.

## Trademarks

Logos belong to their owners. Ship them only as vendor-published brand assets used to identify the app, never altered beyond the recoloring this spec requires.
