# Brand assets

These two SVGs are the single source of truth for the app's logo and favicon.
**To change either, just edit the SVG file here — no code changes needed.**

| File | Where it shows | Notes |
|------|----------------|-------|
| `logo.svg` | Sidebar brand mark (top-left) | Rendered via `<img>` in `src/components/shell/Sidebar.tsx`. Should be a self-contained, square icon (it carries its own background/tile). |
| `favicon.svg` | Browser tab + PWA icon | Referenced from `index.html` (`<link rel="icon">`) and `public/site.webmanifest`. |

## How it works
- This folder lives under `public/`, so Vite serves the files verbatim at
  `/brand/logo.svg` and `/brand/favicon.svg` and copies them into `dist/` on build.
- Nothing imports the SVG markup into JS, so swapping a file is enough — in dev it
  hot-reloads; in prod it ships on the next build.

## Tips when replacing
- Keep the files **square** (current viewBox is `0 0 80 80`) so they aren't distorted.
- The sidebar tile is clipped to a rounded square (`border-radius` in `shell.css`); a
  self-contained tile background in the SVG keeps the rounded look consistent.
- Browsers cache favicons aggressively — hard-refresh (or bump the filename) if an
  updated `favicon.svg` doesn't appear in the tab.
