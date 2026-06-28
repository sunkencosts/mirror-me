# Weekly Results Browser — Design

A per-week view on the **League Stats** page: for a chosen week and team, see how you did
vs the official lineup and vs **everyone else who mirrored that same team** that week, with
the ability to expand any setter to their full per-player lineup.

## Why (week × team), not (week)

The comparison only makes sense among people who mirrored **the same roster** — same player
pool, same ceiling ("same roster, different brain"). A flat "all lineups this week" list
mixes incomparable teams and doesn't scale (a popular team can have 100+ setters). So the
navigation is always **Week → Team → ranked setters**.

## Scale strategy (100+ setters)

1. **List = scores only; detail = on-demand.** The ranked list reads the cached
   `week_results` rows (cheap, no Sleeper call). The expensive per-player breakdown is fetched
   lazily, per row, only when expanded — never 100 at once.
2. **Top-N + you + search.** Default ~15 rows, the signed-in user's row always pinned with
   its true rank ("you beat 87 of 99 setters"); username search + pagination for the tail.

## UX

League Stats becomes two tabs:
- **Season** — the per-league leaderboard (already built).
- **By week** — week selector (graded weeks) + team selector (league rosters). For the chosen
  (week, team): a baseline header (official total, optimal total, official efficiency, setter
  count) then the ranked setter table (efficiency w/ scorebar, edge vs manager, points, W/L),
  you pinned. Expanding a row shows that setter's starters vs the official lineup, per player.

Reuse: week-selector pattern from `LineupsPage`; team names from `/league/{id}/rosters`;
`LeaderboardTable` scorebar/edge styling; `ScoredPlayer`/`PlayerAvatar` for the detail.

---

## Backend (TDD — tests first, per `CLAUDE.md` new-endpoint checklist)

No migration needed (`week_results` exists). Add `week_results` to the `TestMain` truncate
list if not already present.

### Endpoint A — weekly setter list (public, DB-only)
`GET /league/{leagueId}/week/{week}/results?season=&roster_id=&q=&limit=&offset=`

- Reads `week_results ⨝ users` for (season, league, week, roster). Per setter: efficiency =
  `clamp01(user_total/optimal_total)`, edge = `clamp01(user/opt) − clamp01(official/opt)`,
  rank within the roster by efficiency desc. Baseline (official_total, optimal_total,
  official_efficiency) from the roster's rows. `setter_count` = full count (pre-pagination).
- Public; the frontend marks "you" from its own `userId`. Graded weeks only (empty + 200 for
  ungraded/live or a team nobody mirrored).

Provider models: `WeeklyRosterResults { roster_id, official_total, optimal_total,
official_efficiency, setter_count, setters []WeeklySetterResult }`,
`WeeklySetterResult { user_id, username, user_total, efficiency, edge, result, rank }`.

DB method: `WeeklyResults(ctx, season, leagueID string, rosterID int, q string, limit, offset int)`.

**Tests** (`server_test.go`):
- `TestWeeklyResults_RanksByEfficiency` — multiple setters → sorted desc, ranks 1..n.
- `TestWeeklyResults_Baseline` — official/optimal/official_efficiency correct.
- `TestWeeklyResults_FilterByRoster` — only the requested roster's setters.
- `TestWeeklyResults_SearchUsername` — `q` filters by username (ILIKE).
- `TestWeeklyResults_Pagination` — limit/offset slice; `setter_count` is the full total.
- `TestWeeklyResults_EmptyRoster` — nobody mirrored → `200` with empty setters.
- `TestWeeklyResults_SeasonScoped` — only the requested season's rows.

### Endpoint B — one setter's scored lineup (public)
`GET /league/{leagueId}/week/{week}/roster/{rosterId}/score?user_id=`
→ `{ user: ScoredLineup, official: ScoredLineup, optimal_total, user_efficiency,
official_efficiency, edge, result, final }` (the `CompareResponse` shape).

- Reuses the compare scoring core. **Refactor**: extract the existing auth'd
  `HandleGetCompare` body into a shared `scoreUserWeek(...)` so the auth'd self-compare and
  this public per-user endpoint share one implementation (no logic drift). Uses `sleeperClient`
  for the week's `player_points` + official lineup (one cached Sleeper call per expand).
- `404` if that user has no lineup for (roster, week). Departed starter → 0 pts (D12).

**Tests**:
- `TestSetterLineup_ScoredUserAndOfficial` — starters carry per-player points; totals match.
- `TestSetterLineup_404NoLineup` — no lineup → 404 (render "didn't set one").
- `TestSetterLineup_DepartedStarterZero` — off-roster starter scores 0.
- `TestSetterLineup_PublicNoAuth` — succeeds without an auth cookie.
- `TestSetterLineup_EfficiencyEdgeFinal` — fields equal `GradeWeek` for the same inputs.

Routes (`cmd/server/routes.go`): both under the existing `/league/{leagueId}/week/{week}/…`
group; A and B are public (no `requireAuth`).

---

## Frontend
- `web/src/types.ts`: `WeeklyRosterResults`, `WeeklySetterResult` (reuse `CompareResponse`,
  `ScoredLineup`, `ScoredPlayer` for the detail).
- `LeagueStatsPage`: tab switch (Season / By week). New `WeeklyResults` component tree:
  `WeekTeamPicker`, `SetterTable` (ranked, you-pinned, search, paginate), `SetterDetail`
  (lazy `useQuery` on expand → endpoint B, per-player rows).
- Data: `useQuery` + `fetchJson`, `async/await`, full variable names; query keys include
  league/week/roster/season.

## Open / deferred
- Per-team **setter counts** in the team picker would need a small summary endpoint or an
  all-rosters variant of A — deferred; v1 shows counts after a team is selected.
- Live (ungraded) week: shows official/optimal only (no setter rows) until graded.
- Privacy: lineups are treated as public within the app (per product decision).
