# mirrorleague — Current State Overview

> **Purpose of this doc:** an accurate, code-derived snapshot of what mirrorleague
> actually does *today*, written before starting new feature work. Generated from a
> scan of the backend (`api/`), frontend (`web/`), migrations, and route definitions.
>
> **Note on drift:** `CLAUDE.md`'s "Current Status" section is stale — it describes the
> app as mid-Step-1 with a single rosters endpoint. In reality the project is well past
> Steps 1–3: Google auth, persisted lineups, kickoff lock enforcement, player rarity
> data, and a multi-page React app all exist. Treat *this* file as the source of truth
> for capabilities; treat `CLAUDE.md` as the source of truth for *patterns/conventions*.

---

## What the app does (in one paragraph)

A user pulls up any public Sleeper fantasy football league, picks a team in it, and
sets their *own* starting lineup from that team's roster for a given week. Lineups lock
at the kickoff of the week's first game. After games are scored, the app compares the
user's lineup against the real manager's official lineup using Sleeper's per-player
points and declares a winner. Players are decorated with a "rarity" tier derived from
dynasty rankings, which drives a client-side roster "power score." Users sign in with
Google (anonymous usage works first, then merges into the account on login) and can
bookmark leagues they follow.

---

## Architecture

| Layer | Tech | Location |
|---|---|---|
| Backend | Go (net/http, std lib router) | `api/` |
| Frontend | React 19 + Vite + React Router + TanStack Query | `web/` |
| Database | Postgres | schema in `api/migrations/000001_init.up.sql` |
| External data | Sleeper API (leagues/rosters/matchups/players), DynastyProcess CSV (rankings) | `api/internal/sleeper`, `sync.go` |
| Auth | Google OAuth → JWT in httpOnly cookie | `api/internal/googleauth`, `jwtauth`, `handlers/auth*.go` |
| Deploy | Frontend → Cloudflare Pages; Go binary + Postgres on a Raspberry Pi via Cloudflare Tunnel; self-hosted GitHub Actions runner | `DEPLOY.md`, `.github/workflows/deploy.yml` |

Backend follows the Mat Ryer / Grafana HTTP-service pattern (`main`→`run`→`NewServer`→
`addRoutes`, handler-maker funcs with narrow interfaces, `encode[T]` helper, e2e tests
that boot the real server). See `CLAUDE.md` for the canonical pattern descriptions.

---

## Backend API surface

All routes are registered in `api/cmd/server/routes.go`. (`api/api.md` documents most of
them but is **missing the auth routes and the compare route** — see note at bottom.)

### Auth (`/auth/*`)
| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/auth/google` | — | Start Google OAuth; sets state cookie, redirects to Google |
| GET | `/auth/google/callback` | — | OAuth callback; creates/loads user, sets `auth_token` JWT cookie (30d), redirects to frontend |
| GET | `/auth/me` | required | Returns the current `{id, email, username}` from the JWT |
| POST | `/auth/merge` | required | Reassigns anonymous-ID data (lineups/bookmarks) to the logged-in user |
| DELETE | `/auth/logout` | — | Clears the auth cookie |
| GET | `/dev/login` | dev only | Issues a JWT without Google (only when `APP_ENV=development`) |

- New OAuth users get an auto-generated handle like `bold_hawk42` (adjective_noun + 2 digits).
- JWT is signed with `JWT_SECRET`; `RequireAuth` middleware injects claims into context.

### League bookmarks (`/league-bookmarks`)
User's saved references to Sleeper leagues, with optional labels. Keyed by
`(user_id, league_id, source)`. `user_id` is currently a client-generated UUID (anonymous)
or the authenticated user's id after merge. **These routes are not behind `requireAuth`.**
| Method | Path | Purpose |
|---|---|---|
| POST | `/league-bookmarks` | Upsert a bookmark (updates label if it exists) |
| GET | `/league-bookmarks?user_id=` | List a user's bookmarks |
| PATCH | `/league-bookmarks/{leagueId}` | Update label |
| DELETE | `/league-bookmarks/{leagueId}?user_id=` | Remove bookmark |

### Lineups (`/lineups`)
The core "set your own lineup" feature. A lineup is a set of starter player IDs for a
`(user, league, roster, week, source)`.
| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/lineups` | required | Create a lineup. Validates starters belong to the roster; derives & stores `season` from the league; **rejects with 409 `lineup locked`** if the week's first kickoff has passed |
| PATCH | `/lineups/{id}` | required | Update starters (owner-only, else 403); also 409 if locked |
| GET | `/lineups?user_id=&league_id=&week_number=[&roster_id=]` | — | List lineups; echoes `locked` per row |
| GET | `/lineups/{id}` | — | Fetch one lineup; echoes `locked` |

### Leagues / rosters / matchups (Sleeper passthrough + enrichment)
| Method | Path | Purpose |
|---|---|---|
| GET | `/league/{leagueId}` | League metadata from Sleeper (name, season, scoring_settings, roster_positions, settings) |
| GET | `/league/{leagueId}/rosters` | Rosters with resolved player objects (name, team, image, rarity) and team names; includes starters/reserve/taxi |
| GET | `/league/{leagueId}/week/{week}` | Week matchups **enveloped** with lock state: `{ locked, locks_at, matchups: [...] }`. This is the editor's source of truth for whether edits are allowed |
| GET | `/league/{leagueId}/week/{week}/roster/{rosterId}/compare?user_id=` | **The payoff endpoint.** Scores the user's submitted lineup vs the official lineup using Sleeper's per-player points; returns both scored lineups + winner (`official`/`user`/`tie`). 404 if no lineup submitted |

### Players & admin & health
| Method | Path | Purpose |
|---|---|---|
| GET | `/players` | Active fantasy players (slim shape) for the lineup picker / search |
| POST | `/admin/sync-players` | Behind `ADMIN_SECRET`. Fetches full Sleeper player map + DynastyProcess rankings CSV in parallel, computes rarity tiers, upserts all players, invalidates roster cache |
| GET | `/healthz` | Pings DB; 200/503. Used by load balancer and test `waitForReady` |
| `/` | SPA fallback | Serves `web/dist` (built frontend) |

---

## Lineup locking (kickoff freeze)

Implemented per `LINEUP_LOCK_PLAN.md`:
- A NFL-global `week_locks(season, week, locks_at)` table holds the **first-kickoff time**
  of each week (stored UTC). Seeded by hand for **2026**, weeks 1–11 and 13–17 (weeks 12
  and 18 left fail-open pending confirmed kickoff times — see the migration comment).
- Backend write-gate is authoritative: `POST`/`PATCH /lineups` reject with **409
  `lineup locked`** once `now() >= locks_at`.
- **Fail open:** a missing `week_locks` row means *not locked* (logged as a warning).
- The whole week locks atomically at one timestamp — no per-player kickoff logic.
- Reads surface lock state two ways: the `{ locked, locks_at, matchups }` envelope on
  week-matchups, and a `locked` flag echoed on `Lineup` reads.

> **Known gap:** 2026 weeks 12 (Thanksgiving) and 18 (flex/TBD) are intentionally
> unseeded (fail-open) until their real first-game kickoffs are confirmed. The seeded
> Thursday times were derived from a published TNF schedule — spot-check before relying
> on locks for real grading, as TNF times occasionally shift.

---

## Player rarity & scoring

- **Rarity** (`mythic` · `orange` · `purple` · `blue` · `green` · `grey`) is assigned in
  `sync-players` from DynastyProcess dynasty rankings: players are ranked within each
  position, converted to a percentile, then bucketed. Rank #1 per position = `mythic`.
- **Power score** is computed *client-side* (`web/src/scoring.ts`): each starter's rarity
  points × a position multiplier (QB 1.4, RB/WR 1.2, TE 1.0, K/DEF 0.7), normalized to a
  0–10 score, then mapped to S/A/B/C/D tiers (absolute or relative-to-league).
- **Matchup scoring** (the compare endpoint) uses Sleeper's real `player_points` for the
  week — not the power score. Power score is a roster-strength heuristic; compare is the
  actual fantasy result.

---

## Frontend

React SPA (`web/src`), routes in `web/src/App.tsx`, nav model in
`web/src/components/shell/routes.ts`. Two scopes: **global** and **league**.

| Route | Page | Status |
|---|---|---|
| `/` | My Leagues (home) | ✅ real — bookmarked leagues, connect a league |
| `/leaderboard` | Global Leaderboard | 🚧 **ComingSoon stub** |
| `/rankings/rookies` | Rookie Rankings 2026 | ✅ real — static data in `web/src/data/rookieRankings2026.ts`, filters by position + scoring mode |
| `/{leagueId}/lineups` | Lineups editor | ✅ real — pick starters, week selector, lock-aware, power score |
| `/{leagueId}/teams` | Teams | ✅ real — roster cards |
| `/{leagueId}/stats` | League Stats | 🚧 **ComingSoon stub** |
| `/{leagueId}/best-setters` | Best Setters | 🚧 **ComingSoon stub** (shows scoring info box) |
| `/league/:leagueId[/week/:week]` | — | Legacy redirect to new URL structure |

- Auth context (`web/src/context/AuthContext.tsx`) manages the anonymous user-id and the
  authenticated session; a client UUID is generated and stored locally for anonymous use,
  then merged on login via `/auth/merge`.
- Data fetching uses TanStack Query; `fetch` always sends `credentials: "include"` so the
  auth cookie rides along.

**Stub pages that still need building:** Global Leaderboard, League Stats, Best Setters.
(These align with the open ideas in `todo.md`.)

---

## Data model (Postgres)

From `api/migrations/000001_init.up.sql`:
- **`players`** — synced reference data: id, name, team, positions, active, `rarity`.
- **`lineups`** — user's chosen starters. Unique on `(user_id, league_id, roster_id, week_number, source)`. Stores `season`.
- **`week_locks`** — `(season, week) → locks_at` first-kickoff times (seed TODO).
- **`league_bookmarks`** — `(user_id, league_id, source) → label`.
- **`users`** — OAuth users: `oauth_provider/oauth_id`, email, unique username.

`source` distinguishes whose decision a row represents (e.g. `mirror` vs `user`).

---

## External integrations

- **Sleeper API** (`api/internal/sleeper/client.go`): `GetLeague`, `GetRosters`
  (joins league users for team names, resolves player objects, builds CDN image URLs),
  `GetWeekMatchups`. Roster data is cached with an `InvalidateRosters()` hook. Read-only,
  no auth token. Player map is large — synced via the admin endpoint, not per-request.
- **DynastyProcess rankings CSV** (`RANKINGS_CSV_URL`, default
  `dynastyprocess/data` on GitHub): source of dynasty ranks → rarity.
- **Google OAuth** for sign-in.
- *Not yet integrated:* Tank01/RapidAPI (planned Step-3/4 stat source — compare currently
  relies entirely on Sleeper's own per-player points), Anthropic API (planned "roast"
  feature).

---

## Configuration (`api/pkg/config/config.go`)

Env vars (with defaults): `PORT` (8080), `DATABASE_URL`, `SLEEPER_BASE_URL`,
`RANKINGS_CSV_URL`, `MIGRATIONS_URL`, `CURRENT_WEEK`, `APP_ENV`, `FRONTEND_URL`
(localhost:5173), `JWT_SECRET`, `ADMIN_SECRET`, `LOG_FILE`, and the Google OAuth set
(`GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL` + overridable auth/token/userinfo URLs).

---

## Dev workflow

- `make db` + `make migrate-up`, then `curl -X POST .../admin/sync-players` to seed players.
- `./dev.sh` opens a tmux session (air-watched Go server :8080, vite :5173, terminal, claude).
- `make test` runs the Go e2e suite against `mirrorleague_test` (boots the real server,
  fakes Sleeper with `httptest`, truncates `lineups`/`players`/`league_bookmarks`/`week_locks`).
- TDD is required for new endpoints (tests written first).

---

## Build-status summary

| Capability | State |
|---|---|
| Import/browse a Sleeper league + rosters | ✅ Done |
| View a week's matchups + official lineups/points | ✅ Done |
| Set your own lineup from a roster | ✅ Done |
| Lock lineups at first kickoff | ✅ Code done; 2026 seeded (wks 12 & 18 pending) |
| Compare user vs official lineup → winner | ✅ Done (uses Sleeper points) |
| Google sign-in + anonymous→account merge | ✅ Done |
| Bookmark leagues | ✅ Done |
| Player rarity tiers + roster power score | ✅ Done |
| Rookie rankings page | ✅ Done (static 2026 data) |
| Global leaderboard | 🚧 Stub |
| League stats page | 🚧 Stub |
| "Best setters" (manager efficiency tracking) | 🚧 Stub — the stated end-goal of the lock work |
| Tank01 stats / AI roast | ❌ Not started |

---

## Doc-maintenance follow-ups

1. ✅ `CLAUDE.md` "Current Status" updated to reflect the real build state.
2. ✅ `api/api.md` now documents the `/auth/*` routes and the `.../compare` route.
3. ◐ `api/migrations/000001_init.up.sql` now seeds `week_locks` for 2026 weeks 1–11 and
   13–17. Remaining: confirm and add weeks 12 (Thanksgiving) and 18 (flex/TBD).
</content>
</invoke>
