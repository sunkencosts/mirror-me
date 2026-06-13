# Frontend Redesign — Hand-off Notes

Status + next steps for finishing the design-handoff redesign. **Frontend only** for
now (no new Go endpoints). Source of truth for the look is
`design_handoff_mirror_league/` (prototype HTML/CSS/JS + `README.md`).

## Done

- **Increment 1 — Shell + tokens + routing.** Sidebar/topbar/mobile drawer + bottom
  tabs, design tokens (`src/index.css`), 7-route structure (`src/App.tsx`), scoped URLs
  (`/:leagueId/lineups` etc.), legacy redirects.
- **Increment 2 — Lineups.** `RosterCard` team card (power meter, score strip, player
  rows, override popover + mobile bottom-sheet), `LeagueSummary` hero + week stepper.
- **Increment 3 — My Leagues.** `home-hero`, connect form, `lg-cards` grid
  (`LeagueBookmarks`).
- **Members** page is real (derived from `/league/:id/rosters`).

Verified live against the real backend (`dev.sh` stack). Build/lint green.

## Remaining (4 stub pages)

All four are still `ComingSoon` stubs:
`src/pages/{LeagueStatsPage,BestSettersPage,LeaderboardPage,RookieRankingsPage}.tsx`.

## Conventions established (follow these)

- **CSS:** port the prototype's `styles.css` rules **verbatim** into a global stylesheet
  imported in `src/main.tsx`; React components emit the prototype's exact class names
  (`.kpi`, `.panel`, `.tbl`, `.podium`, …). CSS-module hashing means no collisions.
  Existing global sheets: `shell.css`, `lineups.css`, `home.css`. The stats/tables/charts
  CSS is **not ported yet** — that's step 0 for the pages below.
- **Icons:** `<Icon name="…" />` from `src/components/icons.tsx` (CSS-sized).
- **Avatars:** `PlayerAvatar` (real headshot in `.pav` ring) or, for manager monograms,
  `avatarBg`/`initials` from `src/utils/avatar.ts` + `RARITY_RING`/`TIER_RING`.
- **Verify:** `node scripts/verify-live.mjs` screenshots real pages (needs `dev.sh`
  stack: vite `:5173` + Go `:8080` + db). Screenshots land in `web/parity-out/`.

## Step 0 (shared): port the remaining prototype CSS

From `design_handoff_mirror_league/styles.css`, port these sections into a new
`src/stats.css` (and import in `main.tsx`): `.kpi-row/.kpi`, `.grid-2`, `.chart` (bar
chart), `.dist` (distribution rows), `.tbl` + `.rk/.who/.scorebar`, `.top3-row/.top3-card`,
`.podium/.pod`, `.filters/.fchip`, rookie `.tier-*/.rk-card/.rk-list`, `.method/.mcard`.
(`.infobox` and `.panel` are already in `shell.css`.) Roughly lines 354–465 + 441–464.

## Per-page

### League Stats — `LeagueStatsPage`
Prototype: `page-more.js` `stats()`. Layout: KPI row (4) → `grid-2` [weekly bar chart |
roster-power distribution] → standings `.tbl`.
- **Real now (frontend-only):** roster-power distribution + the power column — use
  `computePowerScore` over `/league/:id/rosters` (already done on Members).
- **Needs cross-week data:** KPIs (avg power ok; win-rate/pts-per-wk not), the
  "your vs real manager" weekly chart, and standings pts/best/trend. Either loop
  `/league/:id/week/N` for N=1..currentWeek on the client and aggregate, or defer to a
  future `/league/:id/stats` endpoint. Recommend: build layout + distribution now, leave
  the cross-week panels behind an empty/"needs more weeks" state.

### Best Setters — `BestSettersPage`
Prototype: `page-more.js` `setters()`. `ScoringInfoBox` (already built) → `.podium`
(render order 2nd/1st/3rd) → Manager Score ladder `.tbl`.
- **Blocked on backend:** Manager Score = cross-week win-rate vs real manager + shrinkage
  (`adjusted = (wins + k/2)/(total + k)`, k≈8), min 4 weeks. No endpoint exists.
- **Frontend-only path:** build `Podium` + ladder table components against a typed
  `ManagerScore[]` shape; render empty state until the endpoint lands. Don't fake data.

### Leaderboard (global) — `LeaderboardPage`
Prototype: `page-more.js` `leaderboard()`. `ScoringInfoBox global` → `.filters` chips →
`.top3-row` cards → ladder `.tbl` + "Load more".
- **Blocked on backend:** cross-league Manager Score aggregation. Same as Best Setters but
  global. Build UI against the typed shape; empty state for now.

### Rookie Rankings — `RookieRankingsPage`
Prototype: `page-more.js` `rookies()`. `home-hero` banner → ranked `.rk-list` panel →
`.method` 3-card methodology (static copy).
- **Data:** backend already ingests a rankings CSV (`RankingsCSVURL` via
  `admin/sync-players`) that sets player `rarity`, but `/players` (`SlimPlayer`) has **no**
  rookie flag / projection / movement. Needs a small endpoint (e.g. `/rankings/rookies`)
  or extending the players payload.
- **Frontend-only path:** build hero + methodology (fully static, no data) now; the ranked
  list needs the endpoint — stub the list until then.

## Backend gaps (for when frontend-only ends)

- **Manager Score** service (per-week lineup-vs-official compare, shrinkage, ≥4 weeks) →
  powers Best Setters (league) + Leaderboard (global). Biggest piece.
- **Stats aggregation** endpoint (optional; otherwise client-side multi-week fetch).
- **Rookie rankings** endpoint exposing projection/movement from the existing CSV.

## Loose ends

- `design_handoff_mirror_league/mirror-league` is a self-referential symlink I added so
  the prototype HTML (which references `mirror-league/…`) renders; keep it for the parity
  harness / opening the prototype.
- Pre-existing lint debt (`useBlockStatements`, one `noNonNullAssertion`) in
  `main.tsx` / `LineupsPage` / `PlayerSearch` predates the redesign — not introduced here.
- Nothing committed yet — Increments 1–3 are uncommitted on `main`.
