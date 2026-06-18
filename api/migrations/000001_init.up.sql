CREATE TABLE players(
    player_id text PRIMARY KEY,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    team text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT FALSE,
    fantasy_positions text[] NOT NULL DEFAULT '{}',
    number int NOT NULL DEFAULT 0,
    age int NOT NULL DEFAULT 0,
    rarity text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE lineups(
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    league_id text NOT NULL,
    roster_id int NOT NULL,
    week_number int NOT NULL,
    season text NOT NULL DEFAULT '',
    source text NOT NULL,
    starters text[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT lineups_key UNIQUE (user_id, league_id, roster_id, week_number, source)
);

-- week_locks: NFL-global lineup lock times, keyed (season, week).
-- locks_at is the kickoff of the first game of that NFL week, stored UTC.
-- Lineup writes are rejected once now() >= locks_at. A missing row = not locked
-- (fail open). Seed one row per NFL week per season; tests seed their own rows.
CREATE TABLE week_locks(
    season text NOT NULL,
    week int NOT NULL,
    locks_at timestamptz NOT NULL,
    PRIMARY KEY (season, week)
);

-- TODO(seed 2025): insert the 18 first-kickoff times for the 2025 regular season,
-- converting each week's first-game ET kickoff to UTC (watch the Nov DST fall-back).
-- Example shape (replace with real kickoff times):
--   INSERT INTO week_locks (season, week, locks_at) VALUES
--     ('2025', 1, '2025-09-05 00:20:00+00'),
--     ('2025', 2, '2025-09-12 00:15:00+00');

CREATE TABLE league_bookmarks(
    user_id text NOT NULL,
    league_id text NOT NULL,
    source text NOT NULL,
    label text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, league_id, source)
);

CREATE TABLE users(
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    oauth_provider text NOT NULL,
    oauth_id text NOT NULL,
    email text NOT NULL,
    username text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (oauth_provider, oauth_id)
);

