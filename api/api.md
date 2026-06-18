# API Reference

> **Keep this file up to date.** Whenever you add, remove, or change a route in `cmd/server/routes.go`, update this file in the same commit.

Base path: `/`

---

## League Bookmarks

A user's saved references to Sleeper leagues, with optional labels. `user_id` is a client-generated UUID stored locally by the frontend (no auth yet — will move to session in Step 3).

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

## Admin

### `POST /admin/sync-players`
Pulls the full player list from Sleeper and dynasty rankings from the configured CSV URL, then upserts all players into the database. Runs two fetches in parallel.

No request body.

**Response** `200 OK`
```json
{ "upserted": 1234 }
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
