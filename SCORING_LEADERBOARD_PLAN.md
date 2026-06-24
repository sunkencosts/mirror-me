# Scoring & Leaderboard — v1.0 Spec + Test Plan

> Shared-understanding document for the competitive layer. Decisions below were made
> deliberately (see "Decisions" table). The bulk of this file is **English test
> scenarios** written Given/When/Then so we can turn them into Go e2e tests (TDD first,
> per `CLAUDE.md`). Where a rule is **net-new work** vs. what the code does today, it's
> flagged ⚠️.

---

## 1. The competition in one paragraph

A user mirrors a real team in a public Sleeper league and sets their **own** legal
starting lineup from that team's roster for a week. Lineups lock at first kickoff
(already built). Once the week is in the past, the app scores the user's chosen starters
against the **real manager's** official starters using Sleeper's per-player points for
that week. Two things come out of that: a per-week **head-to-head result** (did you beat
the manager — the visceral, shareable brag) and a **lineup-efficiency** score (how close
your starters came to that roster's best-possible lineup). **Mean lineup efficiency** ranks
two leaderboards — a **global** one (one row per user, current season, 3-week minimum) and a
**per-league** one (instant, no minimum) — with head-to-head win rate shown alongside.

---

## 2. Decisions (locked)

| # | Question | Decision |
|---|---|---|
| D1 | What do you beat? | **The real manager only** — head-to-head, same roster (today's `compare.go`). |
| D2 | Lineup legality | **Enforce `roster_positions`** — exact slot counts + FLEX eligibility. ⚠️ net-new |
| D3 | Primary rank metric | **Average lineup efficiency** (user points ÷ that roster's optimal-lineup points, averaged over graded weeks). Win-rate-vs-manager is shown as a **secondary column**, not the sort. *(Revised — was win rate; efficiency is fair across users who mirror different-quality managers, and is an established no-learning fantasy stat. See D23/D24.)* |
| D4 | Board scope | **Both** a per-league board and a global board. |
| D5 | Tie in a week | **Doesn't count toward win rate** — excluded from numerator *and* denominator (`winrate = wins / (wins + losses)`). Applies only to the *secondary* win-rate column; a tie week still has an efficiency value that counts normally (D23). Near-moot in practice — true ties almost never occur. |
| D6 | Global board identity | **One row per user** — aggregate across all their mirrors. |
| D7 | Minimum games to be ranked | **Global board: 3 graded weeks** before a user enters the ranked standings (below = "provisional / N weeks to qualify"). **Per-league board: no minimum — ranked from week 1.** Rationale: the global board mixes strangers where a 1-week 100% would look broken/gameable; a per-league board is a known friend group where seeing results *immediately* (and watching leaguemates set each other's lineups) is the whole fun, so instant ranking wins over small-sample purity. |
| D8 | No lineup submitted for a week | **Skipped** — not a win, loss, or counted week. |
| D9 | Week finality | **Past weeks only** — a week is gradeable iff `week < CURRENT_WEEK`. No live scoring. |
| D10 | FLEX / eligibility | **Sleeper-style** — `FLEX`=RB/WR/TE, `SUPER_FLEX` adds QB; any rostered player startable (bye/injured allowed, they score 0). |
| D11 | Equal-efficiency tiebreaker | **More graded weeks ranks higher** — rewards both a sustained sample and the act of setting more lineups (each set lineup becomes a graded week once past). Then D20. |
| D12 | Started player left roster at grading | **That slot scores 0** — they weren't on the roster at kickoff, so they were never a legal starter and can't earn points. ⚠️ today it 500s. **UI must show the reason** ("off this roster at kickoff — 0 pts") so the zero isn't mysterious. |
| D13 | Board timespan | **Current season only** (2026); resets per season. |
| D14 | Multi-team aggregation | **Pool all weeks** across every mirror, each week weighted equally. Primary metric (efficiency) pools per D24; the secondary win-rate column pools as `Σwins / Σ(wins+losses)`. |
| D15 | Auth to rank | **Required** — only signed-in users (with a username) appear on either board. Anonymous play works but doesn't rank. |
| D16 | Slot scope | **Standard offense + FLEX/SUPER_FLEX** (QB/RB/WR/TE/K/DEF/FLEX/SF). Exotic slots (IDP, REC_FLEX, WRRB_FLEX) **fail-open with a logged warning** in v1.0. |
| D17 | No published matchup data at save time | **Keep skipping** position validation (fail-open) — don't block early lineup-setting. |
| D18 | Same-league multi-mirror | **One row, pooled** within that league (consistent with D6). |
| D19 | Tie-only user (0 decided games) | **Dissolved by D3.** Since ranking is by efficiency (not win rate), a tie-only user still has efficiency values and ranks normally; their *win-rate column* shows "—". No special handling needed. |
| D20 | Final deterministic tiebreak | Sort order is **mean efficiency desc → graded weeks desc → user id asc** (the last is stable/arbitrary-but-deterministic). |
| D21 | Compare endpoint fate | **Keep as a route, but harden it.** It powers a per-week "how did your lineup do" results screen AND becomes the shared grading function the leaderboard calls. ⚠️ Must move behind `requireAuth` and derive `user_id` from the JWT (drop the `?user_id=` query param). |
| D22 | Compare for the live/current week | **Display live, grade only when final.** The results screen shows live Sleeper scores for an in-progress week with a `final: false` flag; the leaderboard ignores any non-final result (D9). Display and grading are separate concerns. |
| D23 | Optimal lineup & efficiency | **Optimal lineup** = the highest-scoring **legal** lineup obtainable from that roster for the week, using Sleeper `player_points` + `roster_positions` + slot eligibility (a max-weight assignment over the same legality engine as D2). **Efficiency** (per week) = `user_total / optimal_total`, clamped to [0,1]; `optimal_total == 0` ⇒ efficiency undefined, week excluded. Hindsight-optimal, exactly like the "optimal points" stat Sleeper/ESPN already show. |
| D24 | Efficiency aggregation | **Mean of weekly efficiency** across all of a user's graded weeks, pooled across every mirror (each graded week weighted equally — consistent with D14). *Not* `Σuser / Σoptimal`. |
| D25 | Per-week framing | The results screen labels both truths together: **"Result: Win/Loss · Efficiency: NN%"** — you can win a week at low efficiency (both left points on the bench) or lose at high efficiency; both are sensible and shown side by side. |
| D26 | CURRENT_WEEK advancement | **Automate it** (cron/scheduled) so finality (D9) actually fires on time; a stale config makes the board feel stuck ("why hasn't my Sunday win counted?"). |
| D27 | Vs-manager efficiency edge | **Surface it prominently** — `edge = user_efficiency − official_efficiency` (e.g. "You 96% · Manager 88% · **+8%**") on the per-week screen *and* as a stat on profiles/boards. This is the app's core brag ("I out-managed them"); both halves are already in `GradeWeek`, edge is derived. |
| D28 | Shareable brag artifact | **Deferred from v1.0, but designed for.** No result card / share image yet, BUT `GradeWeek` + leaderboard responses must return everything a future card needs — edge (D27), head-to-head record, rank, weeks played — so adding the artifact later is pure frontend. |
| D29 | Weeks played always shown | **Every efficiency figure is displayed with its sample** ("94% over 12 wks"). Display-only — does **not** gate the league board (preserves D7 immediacy), but it contextualizes brags and blunts cherry-picking (a 97%-over-3 visibly outranked by 94%-over-14 in the reader's eye, and by D11 on ties). |
| D30 | `week_results` writer & backfill | A **grading step runs when `CURRENT_WEEK` advances** (rides D26): it computes + caches `week_results` for the newly-final week and **retries/backfills any week still ungraded** (e.g. a transient Sleeper outage at grade time) rather than skipping it permanently. Boards read the cache → league board is genuinely instant; no earned win is silently lost. |

---

## 3. Net-new work vs. today (gap analysis)

| Area | Today | v1.0 target |
|---|---|---|
| Lineup validation (`validateStarters`, `lineup.go:250`) | Membership only — every starter must be on the roster. **No** count check, **no** position/FLEX check, **no** dedup. The **frontend** prevents illegal lineups by construction (fixed slot template from `roster_positions`; per-slot `canFillSlot` filtered picker in `useRosterCard.ts:118`; dedup via bench exclusion), so honest users never hit it — but a direct API call bypasses everything. | ⚠️ Add backend: exact starter count = number of non-bench slots; position-slot legality with FLEX/SUPER_FLEX; reject duplicate player IDs. Mirror the FE's `SLOT_ELIGIBILITY` table (note: FE already includes IDP slots; reconcile with D16). |
| Compare grading (`compare.go`) | Per-week, 2 lineups, winner. **500s** if a user starter isn't in the roster's current player set. | ⚠️ Missing/departed starter → score **0** for that slot, no error. Optionally also assert the user lineup is still legal at grade time. |
| Week finality | Compare scores whatever Sleeper currently returns (could be live). | Leaderboard counts a week only when `week < CURRENT_WEEK`. Compare still shows live but returns `final: false` for non-past weeks; the *leaderboard* is the gated consumer. |
| Compare auth | `GET …/compare?user_id=<uuid>` — **unauthenticated**, trusts the query param; not wired to any UI. | ⚠️ Move behind `requireAuth`, derive `user_id` from JWT claims, drop the query param. Add a `final` flag to the response. Becomes a shared `scoring.GradeWeek`-style function the leaderboard grader reuses (D21/D22). |
| Optimal lineup / efficiency | Does not exist. | ⚠️ New: an `OptimalLineup(roster, player_points, roster_positions)` solver (max-weight slot assignment, reusing the D2 legality engine) → per-week efficiency `user_total/optimal_total`. The single biggest new computation. |
| Leaderboard | No backend at all (stub pages). | ⚠️ New: aggregate graded weeks → **mean lineup efficiency** per user (sort), win-rate-vs-manager per user (secondary column); global + per-league endpoints; **global: min 3 weeks to rank; per-league: no minimum (instant)**; tiebreak by graded weeks. |

---

## 4. Glossary / definitions used in scenarios

- **Official lineup** — the real manager's started players (`WeekMatchup.Starters`).
- **Official total** — `custom_points` if Sleeper provides it, else `points`.
- **User lineup** — the starters the user submitted (`Lineup.Starters`).
- **User total** — `Σ player_points[id]` over the user's starters (missing id ⇒ 0, per D12).
- **Graded week** — a `(user, league, roster, week)` where the user submitted a lineup
  **and** `week < CURRENT_WEEK`. Only graded weeks feed the leaderboard (efficiency + record).
- **Result** — `user` (user total > official), `official` (official total > user), or `tie`.
  This is the per-week head-to-head shown to the user; it is **not** the leaderboard sort.
- **Legal lineup** — exactly fills the league's non-bench slots, each player eligible for
  its slot, no duplicates.
- **Optimal lineup / optimal total** — the highest-scoring *legal* lineup obtainable from
  the roster that week (max-weight slot assignment over `player_points`), and its point sum.
- **Efficiency (weekly)** — `user_total / optimal_total`, in [0,1]; undefined if optimal is 0.
- **Mean efficiency** — average of a user's weekly efficiencies over all graded weeks,
  pooled across mirrors. **This is the leaderboard ranking metric (D3/D24).**
- **Win rate (secondary)** — `wins / (wins + losses)` over graded weeks; ties not in either
  count. Displayed as a secondary column, not the sort.
- **Ranked vs provisional** — *global board only*: a user with ≥3 graded weeks (D7) is
  *ranked*; fewer is *provisional* (shown, flagged, not in the ordered standings). The
  *per-league* board has no such gate — every user with ≥1 graded week is ranked immediately.

---

## 5. Scenario set A — Per-week scoring (winner determination)

> Building on `compare.go`. Fake Sleeper via `httptest` as usual.

- **A1 — User beats manager.** *Given* a roster whose bench WR outscored a started WR,
  *and* the user benched the dud and started the bench WR, *when* compare runs, *then*
  user total > official total and `winner = "user"`.
- **A2 — Manager beats user.** *Given* the user left points on their bench, *when*
  compare runs, *then* `winner = "official"`.
- **A3 — Exact tie.** *Given* user total == official total to the decimal, *then*
  `winner = "tie"`.
- **A4 — Identical lineup ⇒ tie.** *Given* the user started the exact same players as the
  manager, *then* totals are equal and `winner = "tie"`.
- **A5 — `custom_points` overrides `points` for the official side.** *Given* the matchup
  has `custom_points = 101.0` and `points = 99.0`, *then* official total = 101.0 (not 99).
- **A6 — Bench players carry real points.** *Given* the user starts a player who was on
  the manager's *bench*, *then* that player's `player_points` value is counted (Sleeper
  publishes points for bench players too).
- **A7 — Zero-point starter.** *Given* the user starts a player with `player_points` of 0
  (bye / didn't play), *then* that slot contributes 0 and the rest score normally.
- **A8 — No lineup submitted ⇒ 404.** *Given* no user lineup for `(user, roster, week)`,
  *when* compare runs, *then* HTTP 404 `no lineup submitted` (unchanged).
- **A9 — Roster not in week ⇒ 404.** *Given* `rosterId` absent from the week's matchups,
  *then* 404 `roster not found`.
- **A10 — Unauthenticated request ⇒ 401.** ⚠️ *Given* no/invalid auth cookie, *when*
  compare is called, *then* 401 (route now behind `requireAuth`; the old `?user_id=` query
  param is gone — `user_id` comes from JWT claims).
- **A13 — `user_id` derived from JWT, not query.** *Given* user A is signed in but passes
  `?user_id=<userB>`, *then* the param is ignored and A's own lineup is graded.
- **A14 — Past week ⇒ `final: true`.** *Given* `week < CURRENT_WEEK`, *then* the response
  carries `final: true` and is leaderboard-eligible.
- **A15 — Live week ⇒ `final: false`.** *Given* `week >= CURRENT_WEEK`, *then* the
  response still returns live scores/winner but with `final: false`; the leaderboard
  grader must ignore it (cross-check C2).

### Scenario set A′ — Roster drift at grade time (⚠️ D12, changes today's 500)

- **A11 — Started player traded away.** *Given* the user started player `X`, *and* by
  grading `X` is no longer on the roster (`X` absent from `WeekMatchup.Players` and
  `player_points`), *when* compare runs, *then* `X`'s slot scores **0**, the lineup still
  totals the rest, and there is **no 500**.
- **A12 — All started players still present.** Control: no drift ⇒ behaves exactly as A1.

---

## 6. Scenario set B — Lineup legality (⚠️ net-new validation, D2/D10)

> These exercise `validateStarters` on `POST /lineups` and `PATCH /lineups/{id}`.
> League `roster_positions` come from the fake Sleeper league object.

Reference league shape for these tests: `["QB","RB","RB","WR","WR","TE","FLEX","K","DEF","BN","BN","BN"]`
→ **9 starting slots**, FLEX eligible = RB/WR/TE.

- **B1 — Legal lineup accepted.** *Given* 9 starters filling exactly QB×1, RB×2, WR×2,
  TE×1, FLEX×1 (an RB/WR/TE), K×1, DEF×1, *then* `POST /lineups` → 201.
- **B2 — Too few starters rejected.** *Given* only 8 starters submitted, *then* 400
  (`expected 9 starters, got 8`).
- **B3 — Too many starters rejected.** *Given* 10 starters, *then* 400.
- **B4 — Wrong position mix rejected.** *Given* 9 valid-count starters but **3 RB and 1
  WR** (can't fill WR×2), *then* 400 (`no legal assignment for position WR`).
- **B5 — FLEX absorbs the extra RB.** *Given* QB,RB,RB,WR,WR,TE,**RB**(flex),K,DEF, *then*
  201 — the third RB legally fills FLEX.
- **B6 — QB cannot fill standard FLEX.** *Given* a second QB placed where FLEX must go in a
  non-superflex league, *then* 400.
- **B7 — SUPER_FLEX allows a second QB.** *Given* a league with `SUPER_FLEX` and the user
  starts 2 QBs (one in SUPER_FLEX), *then* 201.
- **B8 — Duplicate player rejected.** *Given* the same `player_id` listed twice, *then*
  400 (`duplicate starter`). ⚠️ today this passes.
- **B9 — Non-rostered player rejected.** *Given* a starter not on the roster, *then* 400
  (`player … not available`) — existing behavior, keep.
- **B10 — Bye/injured player allowed.** *Given* a rostered player on bye started in a legal
  slot, *then* 201 (legality is about slots, not availability — D10). They'll score 0 at
  grading (A7).
- **B11 — K and DEF required.** *Given* a lineup missing the DEF, *then* 400.
- **B12 — Validation skipped when no matchup data.** *Given* the week has no published
  matchups yet, *then* validation is skipped and 201 (fail-open, kept per D17 — don't block
  early lineup-setting).
- **B13 — Legality enforced on PATCH too.** *Given* an existing legal lineup, *when*
  PATCH submits an illegal one, *then* 400 and the stored lineup is unchanged.

---

## 7. Scenario set C — Week finality / gradeability (D9)

- **C1 — Past week is graded.** *Given* `CURRENT_WEEK = 5` and a submitted lineup for
  week 3, *then* week 3 counts toward the leaderboard (efficiency + record).
- **C2 — Current week not graded.** *Given* `CURRENT_WEEK = 5` and a lineup for week 5,
  *then* week 5 is **excluded** from the leaderboard (even if compare displays a live result).
- **C3 — Future week not graded.** Lineup for week 7 with `CURRENT_WEEK = 5` ⇒ excluded.
- **C4 — Boundary.** Week == `CURRENT_WEEK - 1` is graded; week == `CURRENT_WEEK` is not.
- **C5 — Advancing the week promotes results.** *Given* a week-5 lineup excluded at
  `CURRENT_WEEK = 5`, *when* `CURRENT_WEEK` becomes 6, *then* week 5 now counts.

---

## 8. Scenario set H — Optimal lineup & efficiency (⚠️ net-new, D23/D24)

> The leaderboard's ranking input. `OptimalLineup` finds the max-points legal lineup from
> the roster's `player_points`. Same fake-Sleeper setup.

- **H1 — Optimal picks the best legal set.** *Given* a roster where the two highest WR
  scorers are benched, *then* the optimal lineup starts them (in WR/FLEX slots as legal)
  and `optimal_total` reflects them.
- **H2 — FLEX goes to the best remaining eligible.** *Given* fixed slots are filled, *then*
  FLEX is assigned the highest-scoring leftover RB/WR/TE (a greedy that's actually optimal
  here only because FLEX-eligibility is a superset — **test a case where naive greedy
  fails** to force a correct assignment algorithm).
- **H3 — User efficiency = user_total / optimal_total.** *Given* user 110, optimal 125,
  *then* efficiency = 0.88.
- **H4 — Perfect lineup ⇒ 1.0.** *Given* the user started exactly the optimal set, *then*
  efficiency = 1.00.
- **H5 — Efficiency clamps / undefined at 0 optimal.** *Given* `optimal_total == 0` (bye-
  heavy / all 0), *then* the week is excluded from mean efficiency (no divide-by-zero).
- **H6 — Departed starter hurts efficiency too (D12).** *Given* a user starter scored 0 for
  being off-roster, *then* their `user_total` is lower while `optimal_total` is unaffected,
  so efficiency drops — consistent with the head-to-head.
- **H7 — Manager efficiency.** *Given* the official lineup, *then* `official_total /
  optimal_total` is computed (same optimal denominator as the user's).
- **H8 — Vs-manager edge (D27).** *Given* user efficiency 0.96 and manager 0.88, *then*
  `edge = +0.08`; returned on the per-week response and aggregated (mean edge) on the boards.
  *Given* the user is less efficient than the manager, *then* edge is **negative** (you can
  brag *or* get humbled — both must render).

## 9. Scenario set D — Per-week result + record (head-to-head, now secondary)

- **D1 — Result recorded per graded week.** Win, loss, win across 3 graded weeks ⇒ record
  **2–1** and the per-week screen shows W/L/W (D25 labels each with its efficiency).
- **D2 — Win rate is secondary.** Record 2–1 ⇒ win rate 0.667 shown as a **column**, not
  the sort key (E-set ranks by efficiency).
- **D3 — Tie (rare) excluded from win rate.** Win, tie, loss ⇒ win rate `1/(1+1)=0.500`;
  the tie still has an efficiency value that **does** count toward mean efficiency.
- **D4 — Unsubmitted weeks ignored (D8).** Weeks 1–3 past, user submitted only 1 and 3 ⇒
  only 2 graded weeks contribute to both efficiency and record.
- **D5 — Current/future submissions excluded (C2/C3).** A not-yet-past week changes neither
  efficiency nor record.
- **D6 — Win at low efficiency / loss at high efficiency (D25).** *Given* the user wins a
  week but only hit 0.80 efficiency, *then* the result is "Win · 80%" — both are stored and
  shown; they don't contradict.

---

## 10. Scenario set E — Global leaderboard (ranked by mean efficiency: D3, D6, D7, D13, D24)

- **E1 — One row per user.** *Given* user U mirrored 2 teams, *then* U appears **once**.
- **E2 — Ranking by mean efficiency desc.** Higher mean weekly efficiency ranks above lower.
- **E3 — Pooled, each week weighted equally (D24).** U has weekly efficiencies
  [0.90, 1.00, 0.80] on team A and [0.95, 0.85] on team B ⇒ mean of all 5 =
  `(0.90+1.00+0.80+0.95+0.85)/5 = 0.90`. **Not** `Σuser/Σoptimal`, and **not** a per-team
  average of averages.
- **E4 — Secondary win-rate column.** Each row also shows the user's pooled
  win-rate-vs-manager, but it does **not** affect ordering (E2).
- **E5 — Minimum weeks to rank (D7).** *Given* a user with 2 graded weeks at 1.00 mean
  efficiency, *then* they are **provisional**, listed/badged "1 week to qualify", and do
  **not** sit atop ranked users; a user with 6 weeks at 0.94 ranks above them.
- **E6 — Tiebreak by graded weeks (D11).** Two ranked users at equal mean efficiency ⇒ the
  one with more graded weeks ranks higher; final tiebreak user id asc (D20).
- **E7 — Current season only (D13).** Mixed 2025/2026 graded weeks ⇒ the 2026 board uses
  only 2026 weeks.
- **E8 — Auth required (D15).** Only authenticated users appear; purely anonymous users are
  excluded.
- **E9 — Live week never moves the board (C2/D9).** In-progress scores don't change any
  efficiency or ranking until the week is past.
- **E10 — Ties are normal here.** A rare tie week still has an efficiency value that counts
  toward mean efficiency, so a tie never makes a user "disappear" from the board (this is
  why moving the sort to efficiency dissolves the old all-ties "—" edge case).
- **E11 — Brag fields on every row (D27/D28/D29).** Each row returns `win_rate`, mean `edge`
  vs managers, `weeks_played`, and `rank` — the exact payload a future share card needs;
  `edge` and `weeks_played` are present even though only efficiency sorts.
- **E12 — Weeks-played context (D29).** *Given* user P at 0.97 over 3 weeks and user Q at
  0.94 over 14, *then* both rows expose `weeks_played` so the UI can show "97% / 3 wks" vs
  "94% / 14 wks"; P still sorts above Q on raw efficiency, but the sample is never hidden.

---

## 11. Scenario set F — Per-league leaderboard (D4)

- **F1 — Scoped membership.** Board for league L lists only users who mirrored a team in
  L; a user who never touched L is absent.
- **F2 — Same metric, scoped weeks.** Mean efficiency computed only from that user's graded
  weeks **in L** (not pooled across other leagues); win rate secondary, also L-scoped.
- **F3 — Same-league multi-mirror pooled (D18).** A user mirroring two teams in L appears
  **once**, efficiency pooled across both within L.
- **F4 — No minimum: ranked from week 1 (D7).** *Given* a league user with a **single**
  graded week at 0.95 efficiency, *then* they appear in the **ranked** standings immediately
  — no provisional state. *(Contrast E5: the same user is provisional on the *global* board.)*
- **F5 — Tiebreak parity.** Equal mean efficiency ⇒ more graded weeks ranks higher, then
  user id asc (same as E6); only the min-weeks gate differs from global.
- **F6 — Empty league board.** A league nobody mirrored returns an empty board, 200 (not
  404).
- **F7 — Week-1 immediacy (the point of no-gate).** *Given* leaguemates set lineups for a
  now-past week 1, *then* the per-league board shows a full ranking that week — no "come back
  in 3 weeks". This is the feature, tested explicitly.

---

## 12. Scenario set G — Robustness / data edges

- **G1 — Sleeper down at grade time → backfilled, not lost (D30).** *Given* the grading step
  runs but Sleeper's fetch fails for a week, *then* no `week_results` row is written and the
  board is computed from available rows; *when* the grading step next runs, *then* it
  **re-attempts** that still-ungraded past week and writes it. An earned win is never
  permanently dropped.
- **G2 — Player map stale.** A started player id missing from the player reference still
  scores via `player_points` if present (scoring keys off points, not the name lookup);
  display falls back gracefully.
- **G3 — Roster with no FLEX.** A league whose `roster_positions` has no FLEX validates
  with strict position counts only (B-set still holds).
- **G4 — IDP / unusual slots (D16).** Leagues with IDP (`DL`,`LB`,`DB`) or `REC_FLEX`:
  v1.0 supports standard offense + `FLEX`/`SUPER_FLEX`/`K`/`DEF`; exotic slots **fail-open
  with a logged warning** (both validation and optimal-lineup skip them gracefully).
- **G5 — Week with no first-kickoff lock (wks 12/18, fail-open).** Lineups can still be
  set late, but grading still keys off `week < CURRENT_WEEK`, so finality is unaffected by
  the missing lock row.

---

## 13. Proposed surface (for the build phase, not yet decided in detail)

- **Scoring core (internal):**
  - `scoring.OptimalLineup(starterSlots, players, playerPoints)` → optimal legal lineup +
    `optimal_total` (max-weight slot assignment; reuse the D2 legality engine). The one
    genuinely new algorithm — must be **correct under FLEX**, not naive greedy (see H2).
  - `scoring.GradeWeek(...)` → `{user_total, official_total, optimal_total, result,
    user_efficiency, official_efficiency, edge, final}` (`edge = user_efficiency −
    official_efficiency`, D27). Shared by the compare handler **and** the grading step.
- **Grading step (the `week_results` writer, D30):** runs when `CURRENT_WEEK` advances
  (rides D26's automation). For each `(user, league, roster)` with a submitted lineup in the
  newly-final week, call `GradeWeek` and upsert a `week_results` row. **Backfills**: also
  re-attempts any past week with a submitted lineup but no `week_results` row (covers
  transient Sleeper failures — G1). Idempotent.
- **Endpoint(s):**
  - `GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare` → **kept**, behind
    `requireAuth`; `user_id` from JWT; response carries `final` + efficiency fields +
    `edge` (D21/D22/D25/D27). Powers the per-week screen ("You 96% · Mgr 88% · +8%").
  - `GET /leaderboard?season=2026` → global, one row per user, **sorted by mean efficiency**;
    each row also returns `win_rate`, `edge` (mean vs-manager edge), `weeks_played` (D29),
    `rank`, and a `provisional` flag (D7). These are exactly the fields a future share card
    needs (D28).
  - `GET /league/{leagueId}/leaderboard` → per-league, same shape (no provisional gate).
- **Data:** a `week_results` table caching each graded week per `(user, league, roster,
  week)` — stores `user_total, official_total, optimal_total, result`. The leaderboard
  reads/aggregates this rather than recomputing from Sleeper each request (stable, cheap,
  rate-limit-safe). Mean efficiency, mean edge, win rate, and weeks-played are aggregations
  over these rows.
- If we add `week_results`, **add it to the truncate list** in `newTestServer`.

---

## 14. Open items — all resolved ✅

1. **B12** → **D17**: keep skipping validation when no matchup data (fail-open).
2. **D4 / E8** (tie-only user "—") → **dissolved by D3**: ranking by efficiency means a tie
   week still has an efficiency value, so tie-heavy users never vanish from the board.
3. **E5** → **D20**: final tiebreak is user id ascending (deterministic).
4. **E9** → **D15**: ranking requires an authenticated account.
5. **F3** → **D18**: same-league multi-mirror pools into one row per user.
6. **G4** → **D16**: standard offense + FLEX/SF only; exotic slots fail-open + warn.
7. **Small-sample board** (1–0 tops the board) → **D7 (revised)**: **global** ranks at **3**
   graded weeks (efficiency metric); **per-league** ranks instantly (week 1) for immediacy.
8. **Unfair-across-opponents win rate** → **D3 (revised)**: rank by lineup efficiency, win
   rate secondary.

No open design questions or un-pinned numbers remain for v1.0.

---

## 15. TDD test function list (write first, per CLAUDE.md)

> Go e2e in `cmd/server/server_test.go`, plus focused validation tests.

**Lineup legality (B-set):**
- `TestCreateLineup_LegalLineupAccepted`
- `TestCreateLineup_RejectsWrongStarterCount`
- `TestCreateLineup_RejectsBadPositionMix`
- `TestCreateLineup_FlexAbsorbsExtraRB`
- `TestCreateLineup_RejectsQBInStandardFlex`
- `TestCreateLineup_SuperFlexAllowsSecondQB`
- `TestCreateLineup_RejectsDuplicateStarter`
- `TestCreateLineup_AllowsByePlayerInLegalSlot`
- `TestUpdateLineup_RejectsIllegalAndKeepsExisting`

**Compare / drift (A-set):**
- `TestCompare_UserBeatsManager`
- `TestCompare_TieWhenEqual`
- `TestCompare_CustomPointsOverridesOfficial`
- `TestCompare_DepartedStarterScoresZeroNo500`
- `TestCompare_RequiresAuth`
- `TestCompare_UsesJWTUserIgnoringQueryParam`
- `TestCompare_PastWeekIsFinal`
- `TestCompare_LiveWeekIsNotFinal`

**Optimal lineup & efficiency (H-set):**
- `TestOptimalLineup_PicksBestLegalSet`
- `TestOptimalLineup_CorrectUnderFlexNotNaiveGreedy`
- `TestEfficiency_UserOverOptimal`
- `TestEfficiency_PerfectLineupIsOne`
- `TestEfficiency_ZeroOptimalExcludesWeek`
- `TestEdge_PositiveWhenUserMoreEfficient`
- `TestEdge_NegativeWhenManagerMoreEfficient`

**Grading step / backfill (G/D30):**
- `TestGradingStep_WritesWeekResultsOnAdvance`
- `TestGradingStep_BackfillsUngradedPastWeek`
- `TestGradingStep_Idempotent`

**Result/record + finality (C/D-set):**
- `TestResult_RecordAndWinRateSecondary`
- `TestResult_TieExcludedFromWinRateButCountsEfficiency`
- `TestResult_UnsubmittedWeeksIgnored`
- `TestGrading_OnlyPastWeeksCount`
- `TestGrading_AdvancingCurrentWeekPromotesResult`

**Leaderboards (E/F-set):**
- `TestGlobalLeaderboard_OneRowPerUser`
- `TestGlobalLeaderboard_RankedByMeanEfficiency`
- `TestGlobalLeaderboard_PooledEfficiencyEqualWeekWeight`
- `TestGlobalLeaderboard_ProvisionalBelowThreeWeeks`
- `TestGlobalLeaderboard_TiebreakByGradedWeeks`
- `TestGlobalLeaderboard_SecondaryWinRateDoesNotSort`
- `TestGlobalLeaderboard_RowReturnsEdgeAndWeeksPlayed`
- `TestGlobalLeaderboard_CurrentSeasonOnly`
- `TestLeagueLeaderboard_ScopedToLeagueMembers`
- `TestLeagueLeaderboard_RanksFromWeekOneNoMinimum`
- `TestLeagueLeaderboard_EmptyReturns200`
