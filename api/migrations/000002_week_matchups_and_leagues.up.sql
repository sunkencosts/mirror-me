-- week_matchups: a persistent cache of Sleeper's per-roster weekly matchup data (the
-- roster's players, the manager's official starters, and crucially each player's points
-- for the week — `player_points`). Sleeper is the source; we cache FINAL weeks (week <
-- current week) here permanently so repeated reads (compare, the per-setter lineup view,
-- grading, the weekly browser) don't re-hit Sleeper and risk its rate limit. The live week
-- is not cached here (its points still change). Keyed (league, week, roster) like the older
-- in-memory cache — season-aware keying is tracked as a follow-up. Seeding these rows lets
-- dev render real per-player scores without any live Sleeper data.
CREATE TABLE week_matchups(
    league_id text NOT NULL,
    week int NOT NULL,
    roster_id int NOT NULL,
    season text NOT NULL DEFAULT '',
    matchup_id int NOT NULL DEFAULT 0,
    owner_id text NOT NULL DEFAULT '',
    team_name text NOT NULL DEFAULT '',
    points double precision NOT NULL DEFAULT 0,
    custom_points double precision,
    players text[] NOT NULL DEFAULT '{}',
    starters text[] NOT NULL DEFAULT '{}',
    player_points jsonb NOT NULL DEFAULT '{}',
    fetched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (league_id, week, roster_id)
);

-- leagues: a persistent cache of a league's static-per-season shape (its roster_positions,
-- name, season, current leg). Grading and the per-setter compare drill-down need
-- roster_positions to compute the optimal legal lineup; caching it here means those reads
-- don't re-hit Sleeper's /league endpoint (rate limit) and, crucially, lets the dev seeder
-- populate league shape so the whole app — including offline synthetic leagues — grades and
-- renders without any live Sleeper data. Written through on a cache miss. roster_positions is
-- treated as stable for the season (a mid-season settings change would serve stale until the
-- row is refreshed — acceptable; the older in-memory cache already accepted 24h staleness).
CREATE TABLE leagues(
    league_id text PRIMARY KEY,
    name text NOT NULL DEFAULT '',
    season text NOT NULL DEFAULT '',
    leg int NOT NULL DEFAULT 0,
    roster_positions text[] NOT NULL DEFAULT '{}',
    fetched_at timestamptz NOT NULL DEFAULT now()
);
