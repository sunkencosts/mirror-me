# API Reference

> **Keep this file up to date.** Whenever you add, remove, or change a route in `cmd/server/routes.go`, update this file in the same commit.

Base path: `/`

---

## Auth

Cookie-based Google OAuth. A successful login sets an `auth_token` JWT cookie
(httpOnly, 30-day max-age). Routes marked **auth required** read the JWT via the
`RequireAuth` middleware; unauthenticated requests get `401`.

### `GET /auth/google`
Starts the OAuth flow. Sets a short-lived `oauth_state` cookie and `302`-redirects to
Google's consent screen.

### `GET /auth/google/callback`
Google redirect target. Validates the `state` cookie, exchanges the code for the user's
profile, creates-or-loads the user (auto-generating a handle like `bold_hawk42` for new
users), sets the `auth_token` cookie, then `302`-redirects to `FRONTEND_URL`.

### `GET /auth/me` — auth required
**Response** `200 OK`
```json
{ "id": "uuid", "email": "string", "username": "string" }
```
`401` if not authenticated.

### `POST /auth/merge` — auth required
Reassigns anonymous-ID data (lineups, bookmarks) to the logged-in user. Call once right
after first login.

**Request body**
```json
{ "anonymous_id": "uuid" }
```
**Response** `204 No Content`. `400` if `anonymous_id` is missing.

### `DELETE /auth/logout`
Clears the `auth_token` cookie.

**Response** `204 No Content`.

### `GET /dev/login` — development only
Only registered when `APP_ENV=development`. Issues a valid `auth_token` without Google,
then redirects to `FRONTEND_URL`. Used for local testing.

---

## League Bookmarks

A user's saved references to Sleeper leagues, with optional labels. `user_id` is a
client-generated UUID stored locally by the frontend for anonymous use, or the
authenticated user's id after `POST /auth/merge`. These routes use `OptionalAuth`: if the
request carries a valid `auth_token`, the JWT subject is used as `user_id` and any
client-supplied `user_id` is ignored; only unauthenticated (anonymous) requests fall back to
the body/query `user_id`.

### `POST /league-bookmarks`
Save a league bookmark (upserts — if the league is already saved, the label is updated).

**Request body**
```json
{
  "user_id": "uuid",
  "league_id": "string",
  "label": "string"
}
```

**Response** `200 OK`
```json
{
  "user_id": "uuid",
  "league_id": "string",
  "label": "string",
  "created_at": "RFC3339"
}
```

---

### `GET /league-bookmarks`
List all bookmarks for a user.

**Query params**
| Param | Type | Required |
|---|---|---|
| `user_id` | UUID | yes |

**Response** `200 OK`
```json
[{ /* LeagueBookmark */ }]
```
Returns an empty array if the user has no bookmarks.

---

### `PATCH /league-bookmarks/{leagueId}`
Update the label on an existing bookmark.

**Path params**
- `leagueId` — Sleeper league ID

**Request body**
```json
{
  "user_id": "uuid",
  "label": "string"
}
```

**Response** `200 OK` — same shape as `POST /league-bookmarks`  
**404** if no bookmark exists for that `user_id` + `leagueId`

---

### `DELETE /league-bookmarks/{leagueId}`
Remove a bookmark.

**Path params**
- `leagueId` — Sleeper league ID

**Query params**
| Param | Type | Required |
|---|---|---|
| `user_id` | UUID | yes |

**Response** `204 No Content`  
**404** if no bookmark exists for that `user_id` + `leagueId`

---

## Lineups

### `POST /lineups`
Create a new lineup for a user.

**Request body**
```json
{
  "user_id": "uuid",
  "league_id": "string",
  "source": "string",
  "roster_id": 1,
  "week_number": 1,
  "starters": ["player_id", "..."]
}
```
- `source` — required; identifies who submitted the lineup (e.g. `"mirror"`, `"user"`)
- `starters` — all player IDs must belong to the specified roster

**Response** `201 Created` — a `Lineup` (see Shared Types).
Sets `Location: /lineups/{id}` header.

**409 Conflict** — `lineup locked` if the week's first game has already kicked off.
The lock is enforced server-side; see `GET /league/{leagueId}/week/{week}` for the
lock state the UI uses to disable editing.

---

### `PATCH /lineups/{id}`
Update the starters for an existing lineup.

**Path params**
- `id` — UUID of the lineup

**Request body**
```json
{
  "user_id": "uuid",
  "starters": ["player_id", "..."]
}
```
- `user_id` must match the lineup's owner or `403 Forbidden` is returned
- `starters` validated against the roster on the existing lineup

**Response** `200 OK` — a `Lineup` (see Shared Types).

**409 Conflict** — `lineup locked` if the week has already locked at first kickoff.

---

### `GET /lineups`
List lineups matching the given filters.

**Query params**
| Param | Type | Required | Notes |
|---|---|---|---|
| `user_id` | UUID | yes | |
| `league_id` | string | yes | |
| `week_number` | int | yes | |
| `roster_id` | int | no | filters to a specific roster |

**Response** `200 OK`
```json
[{ /* Lineup */ }]
```

---

### `GET /lineups/{id}`
Fetch a single lineup by ID.

**Path params**
- `id` — UUID of the lineup

**Response** `200 OK` — same shape as `POST /lineups`  
**404** if not found

---

## League

### `GET /league/{leagueId}`
Fetch league metadata from Sleeper.

**Path params**
- `leagueId` — Sleeper league ID

**Response** `200 OK` — `League` object (name, season, scoring_settings, roster_positions, settings, etc.)

---

### `GET /league/{leagueId}/rosters`
Fetch all rosters for a league from Sleeper.

**Path params**
- `leagueId` — Sleeper league ID

**Response** `200 OK`
```json
[
  {
    "roster_id": 1,
    "owner_id": "string",
    "team_name": "string",
    "players": [{ /* Player */ }],
    "starters": [{ /* Player */ }],
    "reserve": [{ /* Player */ }],
    "taxi": [{ /* Player */ }]
  }
]
```

---

### `GET /league/{leagueId}/week/{week}`
Fetch each team's matchup for a week, enveloped with the lock state for that week.

**Path params**
- `leagueId` — Sleeper league ID
- `week` — NFL week number (≥ 1)

**Response** `200 OK`
```json
{
  "locked": false,
  "locks_at": "RFC3339",
  "matchups": [{ /* WeekMatchup */ }]
}
```
- `locked` — true once `now >= locks_at` (the week's first kickoff)
- `locks_at` — kickoff of the week's first game (UTC); omitted when no lock is seeded
  for the league's season + week (treated as not locked, fail open)

This is the lineup editor's source of truth for whether edits are still allowed.

---

### `GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare` — auth required
Score the **authenticated user's** submitted lineup against the roster's official lineup
and the roster's optimal lineup for the week, using Sleeper's per-player points, and
declare a winner. The user is taken from the JWT (`auth_token` cookie or
`Authorization: Bearer`), not a query param.

**Path params**
- `leagueId` — Sleeper league ID
- `week` — NFL week number (≥ 1)
- `rosterId` — roster within the league (≥ 1)

**Response** `200 OK`
```json
{
  "roster_id": 1,
  "week": 1,
  "official": {
    "starters": [{ /* Player */, "points": 12.3 }],
    "total_points": 110.4
  },
  "user": {
    "lineup_id": "uuid",
    "starters": [{ /* Player */, "points": 9.1 }],
    "total_points": 118.7
  },
  "winner": "user",
  "optimal_points": 132.0,
  "user_efficiency": 0.90,
  "official_efficiency": 0.84,
  "edge": 0.06,
  "final": false
}
```
- `winner` — `"official"`, `"user"`, or `"tie"`
- `official.total_points` uses Sleeper's `custom_points` when present, else `points`
- `optimal_points` — the roster's best-possible legal lineup; `0` when the week is
  excluded (unknown/exotic slots, or the roster can't fill every slot)
- `*_efficiency` — `total / optimal_points`, clamped to `[0, 1]`; `0` when excluded
- `edge` — `user_efficiency - official_efficiency`
- `final` — `false` for the live/current week, `true` once the week has passed
- **401** missing/invalid auth
- **400** invalid week / roster_id
- **404** roster not in this week's matchups, or no lineup submitted for the week

---

## Leaderboards

Aggregate cached `week_results` (written by `POST /admin/grade`) into per-user standings
for a season, sorted by mean lineup efficiency. Only authenticated users with a username
appear. Rows are returned ranked first (1-based `rank`), then provisional rows (`rank: 0`,
`provisional: true`) that have not met the minimum-weeks gate.

### `GET /leaderboard`
Global board pooled across all of a user's mirrors. Provisional below **3** graded weeks.

**Query params**
| Param | Type | Required | Default |
|---|---|---|---|
| `season` | string | no | configured `CURRENT_SEASON` |

**Response** `200 OK`
```json
[
  {
    "user_id": "uuid",
    "username": "alice",
    "rank": 1,
    "mean_efficiency": 0.91,
    "edge": 0.05,
    "win_rate": 0.75,
    "weeks_played": 6,
    "provisional": false
  }
]
```
- `mean_efficiency` — mean of `user_total / optimal_total` over scored weeks
- `edge` — mean of per-week (`user_efficiency - official_efficiency`)
- `win_rate` — wins / (wins + losses); ties excluded
- `weeks_played` — count of scored weeks (`optimal_total > 0`)

### `GET /league/{leagueId}/leaderboard`
Same shape, scoped to one league, with **no** minimum-weeks gate (ranked from week 1).

---

## Analytics

### `POST /collect`
Records one first-party page-view event, fired by the SPA on each route change. Public and
unauthenticated — anonymous visitors are the point — but stamps `user_id` from the
`auth_token` cookie/JWT when present, so one `visitor_id`'s history reveals the anon →
sign-up funnel. Sets **no cookie**: `visitor_id` reuses the frontend's existing
`localStorage` id and rides in the body. `country` is read from the `CF-IPCountry` header
(no raw IP stored); requests from known bot user-agents are stored with `is_bot=true`.

**Request body**
```json
{
  "path": "/leagues",
  "referrer": "https://google.com",
  "visitor_id": "uuid"
}
```
`path` and `visitor_id` are required; `referrer` is optional.

**Response** `204 No Content`. `400` if `path` or `visitor_id` is missing.

Rows land in the append-only `visits` table; there is no read endpoint yet — query the
table directly (visitors/day, anon vs logged-in, top pages, anon→sign-up funnel).

---

## Admin

### `POST /admin/sync-players`
Pulls the full player list from Sleeper and dynasty rankings from the configured CSV URL, then upserts all players into the database. Runs two fetches in parallel.

No request body.

**Response** `200 OK`
```json
{ "upserted": 1234 }
```

### `POST /admin/grade`
Grades every past-week lineup (`week_number < current_week`) that lacks a `week_results`
row and backfills anything still ungraded. Idempotent — already-graded lineups are
skipped, and a lineup that can't be graded this run (transient Sleeper failure, roster not
yet in the week's matchups) is left for a later run. Safe to run on a schedule.

No request body.

**Response** `200 OK`
```json
{ "graded": 42 }
```

---

## Health

### `GET /healthz`
Pings the database. Used by the server's `waitForReady` check in tests and by load balancers.

**Response** `200 OK` if healthy, `503 Service Unavailable` if the DB ping fails.

---

## Shared Types

### LeagueBookmark
```json
{
  "user_id": "uuid",
  "league_id": "string",
  "label": "string",
  "created_at": "RFC3339"
}
```

### Player
```json
{
  "player_id": "string",
  "first_name": "string",
  "last_name": "string",
  "number": 12,
  "age": 28,
  "team": "SF",
  "active": true,
  "fantasy_positions": ["WR"],
  "image_url": "https://sleepercdn.com/content/nfl/players/thumb/{player_id}.jpg",
  "rarity": "orange"
}
```
`rarity` values (dynasty rank percentile): `mythic` · `orange` · `purple` · `blue` · `green` · `grey`

### Lineup
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "league_id": "string",
  "roster_id": 1,
  "week_number": 1,
  "season": "2025",
  "source": "string",
  "starters": ["player_id"],
  "locked": false,
  "locks_at": "RFC3339",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```
- `season` — derived from the league at create time (immutable per `league_id`)
- `locked` — true once the week's first kickoff has passed; populated on reads
- `locks_at` — kickoff of the week's first game (UTC); omitted when no lock is seeded
