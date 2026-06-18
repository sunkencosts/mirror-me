# Lineup Lock Plan (Step toward efficiency grading)

## Goal
Lock a user's lineup edits at the **kickoff of the first game of the week**, so we can
later grade a user's roster-management efficiency against the original manager on a
fair, frozen-at-kickoff basis. Deliberately **simple**: the whole week locks atomically
at one timestamp — no per-player kickoff logic.

## Resolved decisions
1. **Lock-time source:** a seeded, NFL-global `week_locks(season, week, locks_at)` table.
   No live schedule API, no heuristic. 2025 seeded by hand.
2. **Enforcement:** backend write-gate is authoritative (reject post-lock writes);
   frontend reflects lock state for UX. Read-only flags are never the gate.
3. **Season:** stored on each `lineups` row (derived server-side from the league at
   create). `week_locks` stays keyed `(season, week)` — global, ~18 rows/season.
   `league_id` → `season` is immutable in Sleeper, so the derived value never drifts.
4. **Missing lock row:** fail **open** + `logger.Warn`. Absence of seed data ≠ locked.
5. **Rejected write:** `409 Conflict`, plain-text body (`"lineup locked"`). Frontend
   keys off the status code. No structured error body.
6. **Read surfacing:** envelope the week-matchups response
   `{ locked, locks_at, matchups: [...] }` (array→object change) as the editor's source
   of truth, **and** echo `locked` onto `Lineup` reads.
7. **Missing lineup at grade time:** no grade, no penalty. Keep current 404. Out of
   scope of this work; a grading-feature decision.
8. **Seed + tests:** 2025 seed lives in `000001_init.up.sql`. `week_locks` is truncated
   in `TestMain` (both truncate statements) so each test seeds its own `locks_at`.
9. **Write-path shape:** create does one cached `GetLeague` to derive+store season
   (it already hits Sleeper for `validateStarters`); update is fetch-free (season is on
   the row). `locks_at` stored as `timestamptz` UTC; compare against `time.Now()`.
   ET→UTC conversion happens once at seed-entry time — watch the Nov DST fall-back.

---

## Test plan (TDD — written first)

New helper in `server_test.go`:
- `seedWeekLock(t, season string, week int, locksAt time.Time)` — direct INSERT into
  `week_locks`.
- Fake Sleeper handler must serve `GET /league/{id}` with `"season":"2025"` so the
  create path can derive season.

Add `week_locks` to **both** `TRUNCATE` statements (lines ~63 and ~81).

### Write-gate tests
- `TestCreateLineup_RejectedAfterLock` — seed `locks_at` in the past → `POST /lineups`
  returns **409**, body `"lineup locked"`; assert no row persisted.
- `TestCreateLineup_AllowedBeforeLock` — seed `locks_at` in the future → **201**.
- `TestCreateLineup_AllowedWhenNoLockRow` — no row for (season, week) → **201**
  (fail-open).
- `TestUpdateLineup_RejectedAfterLock` — create with future lock, re-seed lock into the
  past, `PATCH /lineups/{id}` → **409**; assert starters unchanged.
- `TestUpdateLineup_AllowedBeforeLock` — future lock → **200**, starters updated.

### Season-storage tests
- `TestCreateLineup_StoresDerivedSeason` — created lineup persists `season` resolved
  from the league object (assert via DB read or exposed field).

### Read-surfacing tests
- `TestWeekMatchups_EnvelopeLockedTrue` — past lock → `GET /league/{id}/week/{w}`
  returns `{ "locked": true, "locks_at": <ts>, "matchups": [...] }`.
- `TestWeekMatchups_EnvelopeLockedFalse` — future lock → `locked:false`.
- `TestWeekMatchups_EnvelopeNoRow` — no row → `locked:false`, `locks_at:null`.
- `TestLineupRead_EchoesLocked` — `GET /lineups/{id}` and `GET /lineups` include
  `locked` reflecting the seeded lock state.

### Regression
- `TestGetCompare_StillWorks` — compare unaffected; missing-lineup path still 404.

---

## Implementation steps (new-endpoint checklist order)

1. **Migration — `api/migrations/000001_init.up.sql`**
   - `CREATE TABLE week_locks(season text NOT NULL, week int NOT NULL,
     locks_at timestamptz NOT NULL, PRIMARY KEY (season, week));`
   - Add `season text NOT NULL DEFAULT ''` to `lineups`.
   - Append 2025 `INSERT`s into `week_locks` (18 rows, first-kickoff ET→UTC).
   - `000001_init.down.sql`: `DROP TABLE week_locks;` (+ revert lineups column).

2. **Provider models — `internal/provider/provider.go`**
   - `Lineup`: add `Season string` + `Locked bool` (echoed on reads; `locks_at`
     optional `*time.Time`).
   - New `WeekMatchupsResponse { Locked bool; LocksAt *time.Time; Matchups []WeekMatchup }`.

3. **DB methods — `internal/db/db.go`**
   - `GetWeekLock(ctx, season string, week int) (time.Time, bool, error)` — `bool=false`
     when no row (drives fail-open).
   - `CreateLineup` signature gains `season`; INSERT + `scanLineup` + all lineup
     `SELECT`s updated to include `season`.

4. **Handlers**
   - `lineup.go`: shared `lockCheck(ctx, season, week) (locked bool, err error)`:
     look up `GetWeekLock`; if found and `time.Now().After/Equal(locksAt)` → locked;
     if not found → `logger.Warn` + not locked.
     - Create: `GetLeague` → season; lock check → 409 if locked; then existing
       `validateStarters`; store season.
     - Update: read existing lineup (has season) → lock check → 409 if locked.
     - List/GetByID: populate `Locked` per row via `GetWeekLock`.
   - `week_matchups.go`: resolve season via cached `GetLeague`, look up lock, return the
     `WeekMatchupsResponse` envelope.

5. **Routes — `cmd/server/routes.go`**
   - No new routes. (Envelope changes existing `GET /league/{leagueId}/week/{week}`
     response shape — update `api/api.md`.)

6. **Frontend**
   - Adapt week-matchups fetch to the `{ locked, locks_at, matchups }` envelope.
   - When `locked`, render read-only lineup + "Locked at kickoff" (and/or a countdown
     to `locks_at` when not yet locked); disable save controls.
   - Treat a `409` from create/update as the locked state (defensive; UI should already
     prevent it).

---

## Out of scope (future)
- Grading/penalizing non-submitters (decided: none for now).
- Per-player kickoff locking.
- Live schedule ingestion (Tank01/ESPN) to auto-populate `week_locks`.
- Snapshotting the manager's official lineup (Sleeper is authoritative post-kickoff;
  manager locks at the same time).
