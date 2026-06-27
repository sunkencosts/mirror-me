# Scoring / Leaderboard — Punch List

Known defects and loose ends in the scoring + leaderboard backend, surfaced during review.
These are **backend** items (grader + leaderboard SQL); the frontend leaderboard UI is
separate. Ordered by severity. See `SCORING_LEADERBOARD_PLAN.md` for the intended model.

---

## 1. Go-vs-SQL efficiency clamp divergence — MEDIUM

**Where:** `api/internal/db/db.go:261` (leaderboard `mean_eff`) vs
`api/internal/scoring/grade.go` (`GradeWeek` → `clamp01`).

**Problem:** The per-week compare path clamps efficiency to `[0,1]`
(`UserEfficiency = clamp01(userTotal/optimalTotal)`), but the leaderboard's `mean_eff` is
`AVG(wr.user_total / NULLIF(wr.optimal_total, 0))` — **unclamped**. Inconsistent even
*within the same query*: the `edge` term right below it (`db.go:262-265`) clamps both halves
with `LEAST(GREATEST(…,0),1)`, while `mean_eff` does not.

**Why it matters:** If any week has `user_total > optimal_total` (a legal lineup shouldn't
exceed optimal, but it can via data anomalies — a started player outside the optimal
denominator, an off-roster starter, or unusual Sleeper points), the season `mean_efficiency`
can exceed what the per-week compare screen showed for the same weeks, and can even render
`> 100%`. The board and the per-week card then disagree.

**Fix:** Clamp `mean_eff` in SQL to match Go:
`AVG(LEAST(GREATEST(wr.user_total / NULLIF(wr.optimal_total, 0), 0), 1))`.
Add a regression test asserting board efficiency == mean of per-week clamped efficiencies.

---

## 2. Cross-season grading scope — MEDIUM

**Where:** `api/internal/db/db.go:209-219` (`ListGradableLineups`),
`api/internal/grading/grading.go` (`GradeSeason`), `grade.go` (`Final = Week < CurrentWeek`).

**Problem:** Finality is decided by a single, season-agnostic `currentWeek` int:
`WHERE l.week_number < $1`. But week numbers repeat every season, and `currentWeek` is the
*current* season's week. A lineup from a **prior season** (whose weeks are all long final) is
gated against the current season's week — e.g. a 2025 week-15 lineup with `currentWeek = 4`
(2026) has `15 < 4 = false` and is **never graded**. The same season-blind comparison drives
`WeekGrade.Final`.

**Why it matters:** Past-season lineups silently never produce `week_results`, so historical
seasons can't be graded or shown on a season-scoped leaderboard. Today only 2026 exists so it
hasn't bitten, but it will the moment a second season is present.

**Fix:** Make finality season-aware: a week is final if its season is past, OR
(season == current season AND week < currentWeek). Pass the current season alongside
currentWeek (or compute per-row from `week_locks` for that lineup's season).

---

## 3. `weeks_played` vs `NULLIF` residual — LOW

**Where:** `api/internal/db/db.go:261, 270`.

**Problem (mostly already handled):** `weeks_played` is
`COUNT(*) FILTER (WHERE wr.optimal_total > 0)`, so weeks excluded from `mean_eff` (via
`NULLIF(optimal_total,0)`) don't pad the count — which means the **global 3-week
provisional gate is safe** (a user whose every graded week is uncomputable has
`weeks_played = 0` and can't qualify). This is the originally-flagged bug, now mitigated.

**Residual:** On the **per-league board (no gate)**, such an all-excluded user still appears
(via `COALESCE(mean_eff,0)`) as `0% · 0 wks`, sorted last. Cosmetic, rare (needs *all* their
weeks uncomputable), but technically a meaningless row.

**Fix (optional):** Exclude users with `weeks_played = 0` from both boards
(`HAVING COUNT(*) FILTER (WHERE optimal_total > 0) > 0`), so a zero-sample user never renders.
