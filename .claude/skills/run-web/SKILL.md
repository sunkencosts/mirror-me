---
name: run-web
description: Screenshot a page with headless Playwright/Chromium to verify a frontend change actually rendered. Use when asked to "check it in the browser", "screenshot the page", or "verify this frontend change".
---

The dev stack (`dev.sh`) is always running in tmux session `mirrorleague`: Vite on :5173 (window "web") and the Go API via `air` on :8080 (window "server"). Don't spend a turn checking with `curl` — assume it's up. Playwright + Chromium are already installed in `web/` (devDependency `playwright`, browser cached at `~/.cache/ms-playwright`).

**Never `pkill`/`kill` vite or air** — that's the user's running dev server, not something this skill owns. If a server isn't up, tell the user rather than starting/stopping anything yourself.

## Quick screenshot of a single page (no auth)

```bash
cd web
node scripts/screenshot.mjs /rankings/rookies rookies
# -> web/parity-out/rookies.png, prints console errors if any
```

`scripts/screenshot.mjs <path> [out-name]` navigates to `APP_URL` (default `http://localhost:5173`) + `<path>`, waits for `load` + a short settle delay, screenshots full page to `web/parity-out/<out-name>.png`, and prints any browser console errors.

## Pages that require a logged-in session

Don't try to reuse the user's real browser session/cookies — fragile and unnecessary. The Go API has a `/dev/login` endpoint (`api/internal/handlers/dev.go`, registered when `APP_ENV != production`, which is the case under `dev.sh`) that mints a session cookie for a dev user without OAuth.

For an arbitrary page, set `DEV_LOGIN` and `screenshot.mjs` will log in first:

```bash
DEV_LOGIN=dev_user node scripts/screenshot.mjs /1182073403987832832/lineups lineups
```

For the standard set of pages (Home/Lineups/Members) against real Sleeper-backed data, `scripts/verify-live.mjs` already does this:

```bash
node scripts/verify-live.mjs [leagueId]
```

## Crawl the whole site

To check every page for problems (console errors, uncaught exceptions, failed requests, broken images, error-boundary screens), use `crawl.mjs`. It logs in, reads the user's league bookmarks from `/league-bookmarks`, and visits every global + per-league route (Lineups/Teams/Stats/Best Setters), screenshotting each to `web/parity-out/crawl/`.

```bash
cd web
node scripts/crawl.mjs                                    # dev_user — has no bookmarked leagues
DEV_USER_ID=<uuid> node scripts/crawl.mjs                  # crawl a real user's leagues
```

To find a real user with bookmarked leagues, query the dev DB:

```bash
psql "$(grep DATABASE_URL api/.env | cut -d= -f2-)" -c "select distinct user_id from league_bookmarks;"
```

## Gotchas

- `waitUntil: "networkidle"` never resolves against Vite's dev server (the HMR websocket keeps the connection open) — use `"load"` plus a short `waitForTimeout` instead.
- React controlled inputs: use Playwright `fill`/`type`, not raw DOM `.value =`.
- Always read the screenshot (or check console errors) — a blank/error page is a failed run, not a successful one. A 401 on an unauthenticated page load is often expected (the shell's "who am I" call) and not a real failure.
