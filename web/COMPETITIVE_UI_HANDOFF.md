# Competitive UI — Frontend Hand-off

Status + spec for building the **leaderboard** and **per-week results** UI. The backend
for all of this is **built, tested, and live on the dev stack** (scoring, optimal lineup,
efficiency/edge, grading, both leaderboards, current-week inference). This doc is the
frontend's contract + guidance. Backend design rationale lives in
`SCORING_LEADERBOARD_PLAN.md`; this is the UI surface of it.

> **Follow the established redesign rules** (`web/FRONTEND_REDESIGN.md`): port the
> prototype's `styles.css` rules **verbatim** into a global stylesheet imported in
> `src/main.tsx`, and emit the prototype's exact class names (`.kpi`, `.panel`, `.tbl`,
> `.podium`, …). Build from the prototype, not from prose.

---

## 0. Prerequisite (don't skip)

The leaderboard/table CSS is **not ported yet** — it's the "Step 0" work listed in
`FRONTEND_REDESIGN.md`: `.tbl` + `.rk/.who/.scorebar`, `.top3-row/.top3-card`,
`.podium/.pod`, `.kpi`, `.filters/.fchip`. Port those from the prototype's `styles.css`
into a new global sheet (e.g. `src/leaderboard.css`) imported in `main.tsx` **before**
building the page. ⚠️ Confirm the prototype source `design_handoff_mirror_league/` is
present in the repo — it's named as the source of truth but was not found during this
hand-off's authoring; locate/restore it first.

---

## 1. What to build (3 surfaces)

| Surface | Route | Backend | Current state |
|---|---|---|---|
| **Global leaderboard** | `/leaderboard` | `GET /leaderboard?season=` | `LeaderboardPage.tsx` = `ComingSoon` stub |
| **Per-league leaderboard** | `/:leagueId/stats` or a new `/:leagueId/leaderboard` | `GET /league/{leagueId}/leaderboard?season=` | `LeagueStatsPage.tsx` = stub |
| **Per-week results card** | on the Lineups page (or a results view) | `GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare` | not built; lineups page currently shows a *client-side* power score |

`BestSettersPage.tsx` is a related stub — "best setters" is essentially the per-league
leaderboard by efficiency, so it may collapse into the per-league board.

---

## 2. API contracts

All reads use the existing `fetchJson<T>` from `src/api.ts` (sends `credentials:"include"`).
Wrap in TanStack Query (`useQuery`) like the existing real pages (`LineupsPage`, `TeamsPage`,
`MyLeaguesPage`). Use `async/await`, full variable names.

### 2a. Leaderboards (public — no auth needed to view)

```
GET /leaderboard?season=2026                       → LeaderboardRow[]   (global, 3-week gate)
GET /league/{leagueId}/leaderboard?season=2026      → LeaderboardRow[]   (per-league, instant)
```

```ts
// add to src/types.ts
export interface LeaderboardRow {
  user_id: string;
  username: string;
  rank: number;            // 1-based among ranked rows; 0 for provisional rows
  mean_efficiency: number; // 0..1 — THE sort key, the headline number
  edge: number;            // mean (you − manager)/optimal; can be negative
  win_rate: number;        // 0..1 — wins/(wins+losses), ties excluded (secondary)
  weeks_played: number;    // always show this next to efficiency
  provisional: boolean;    // true → not yet ranked (global board only)
}
```

- Rows arrive **already sorted**: ranked rows first (by `mean_efficiency` desc), then
  provisional rows (global only) with `rank: 0`. Render in array order.
- **Global board**: users with `< 3` graded weeks are `provisional: true` — show them in a
  separate "Almost there" group, not interleaved with ranked rows.
- **Per-league board**: no provisional gate — everyone is ranked from week 1
  (`provisional` will be false).
- Empty board returns `200` with `[]` (e.g. a league nobody mirrored). Render an empty
  state, not an error.
- Only signed-in users appear (anonymous play doesn't rank).

### 2b. Per-week results / compare (requires auth)

```
GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare   → CompareResponse
```

Behind `requireAuth` — the auth cookie rides along via `credentials:"include"`; the user
is taken from the JWT (no `user_id` query param). `404` if the signed-in user has no
submitted lineup for that `(roster, week)` — render "You didn't set a lineup this week",
not an error.

```ts
export interface ScoredPlayer extends Player { points: number; }
export interface ScoredLineup {
  lineup_id?: string;
  starters: ScoredPlayer[];
  total_points: number;
}
export interface CompareResponse {
  roster_id: number;
  week: number;
  official: ScoredLineup;        // the real manager's lineup
  user: ScoredLineup;            // the signed-in user's lineup
  winner: "user" | "official" | "tie";
  optimal_points: number;        // best-possible from the roster
  user_efficiency: number;       // 0..1
  official_efficiency: number;   // 0..1
  edge: number;                  // user_efficiency − official_efficiency (the brag)
  final: boolean;                // false for the live/current week
}
```

---

## 3. Page specs

### Global leaderboard (`/leaderboard`)
Lead with the **brag**, in this priority:
1. **Mean efficiency** as the headline number per row (e.g. `96%`). This is the rank.
2. **Edge vs managers** prominently (e.g. `+5.5%`, green; negative red) — "how much better
   than the real managers you set lineups." This is the app's core brag.
3. **Weeks played** always shown beside efficiency (e.g. `96% · 12 wks`) so a small sample
   reads as less impressive — guards against cherry-picking.
4. **Win rate / record** as a secondary column.

Suggested layout (reuse prototype classes):
- `.podium/.pod` for the top 3 ranked users.
- `.tbl` with `.rk` (rank), `.who` (avatar + username), and columns for efficiency, edge,
  win-rate, weeks. Use `.scorebar` to visualize efficiency if the prototype has it.
- A **"X weeks to qualify"** group below the ranked table for `provisional` rows.
- Season selector (`?season=`) — default to the current season. A `.filters/.fchip` row.
- Keep the existing `ScoringInfoBox` (with `global`) as an explainer.
- Avatars: `PlayerAvatar` ring or manager monogram via `avatarBg`/`initials`
  (`src/utils/avatar.ts`).

### Per-league leaderboard (`/:leagueId/...`)
Same `LeaderboardRow` shape and row component as global — just a different fetch URL and
**no provisional group** (instant ranking is the point: leaguemates setting each other's
lineups see results from week 1). Decide whether this is its own route, a tab on League
Stats, or the "Best Setters" page.

### Per-week results card (compare)
On the Lineups page (or a dedicated results view), once a week is over:
- Headline: **"You beat {manager} · 96% · +8%"** (or "lost to" / "tied"). Use `winner`.
- Show **`user.total_points` vs `official.total_points`**, and the **`edge`**.
- Per-player rows from `user.starters` / `official.starters` (`ScoredPlayer` carries
  `points` and the full `Player` for name/avatar/rarity).
- A **departed starter** comes back as a `ScoredPlayer` with `points: 0` and an empty/minimal
  player object → render "off this roster at kickoff — 0 pts" so the zero isn't mysterious.
- If **`final` is false** (live/current week), badge it **"Not final"** and make clear it
  doesn't count toward the leaderboard yet — but still show the live scores.

---

## 4. Conventions checklist

- Data: `fetchJson<LeaderboardRow[]>("/leaderboard?season=2026")` inside `useQuery`.
- `credentials:"include"` is already handled by `api.ts` (needed for compare's auth).
- Types in `src/types.ts`; `async/await` (no `.then` chains); full variable names.
- Icons via `<Icon name="…" />`; avatars via `PlayerAvatar` / `src/utils/avatar.ts`.
- CSS: porting prototype classes verbatim (Step 0); global sheet imported in `main.tsx`.
- Routes in `src/App.tsx`; nav model in `src/components/shell/routes.ts`.
- Verify visually with `node scripts/verify-live.mjs` (needs the dev stack); screenshots
  land in `web/parity-out/`.

---

## 5. Testing against dev

The backend is live on `:8080`. Seed sample standings (no live NFL data needed):

```bash
PGPASSWORD=mirrorleague psql -h localhost -p 5433 -U mirrorleague -d mirrorleague \
  -f api/seeds/dev_leaderboard.sql
```

Then the page can fetch:
- `GET http://localhost:8080/leaderboard?season=2026` — ranked board incl. one provisional
  user (`fiona_fresh`, 2 weeks).
- `GET http://localhost:8080/league/dev-std/leaderboard?season=2026` — same users, instant
  ranking (fiona ranked here).

The seed produces a realistic spread (efficiencies ~0.89–0.99, edges ±, a tie, wins/losses)
so every visual state has data. For the **compare** card you'll need a dev login
(`GET /dev/login` when `APP_ENV=development`) plus a submitted lineup for a scored past week.

---

## 6. Open decisions for the builder

1. **Where the per-league board lives** — its own `/:leagueId/leaderboard` route, a tab on
   League Stats, or the "Best Setters" page (they're the same data).
2. **Season selector source** — hardcode current season, read from a league, or a small
   config/endpoint. Backend defaults to `2026` and accepts `?season=`.
3. **Podium vs table-only** — does the global board get a top-3 podium, or just the table?
4. **Compare entry point** — surface per-week results inline on the existing Lineups card,
   or as a separate "Results" view? The lineups page already shows a client-side power
   score; decide how the *real* compare result coexists with or replaces it.
5. **Tie rendering** — rare, but ensure "Tie" renders cleanly (it won't crash; just style).
