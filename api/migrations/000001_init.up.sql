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

-- 2026 regular-season lock times: kickoff of each week's FIRST game, stored UTC.
-- Source: official slate via NFL.com / fbschedules.com (2026 TNF schedule). DST ends
-- Sun Nov 1 2026, so weeks before it are EDT (UTC-4) and weeks on/after are EST (UTC-5).
-- Week 1 is the WEDNESDAY Sep 9 opener (8:20pm ET), not the Thursday game.
-- Weeks 2-11 and 13-17 are the standard Thursday 8:15pm ET TNF game.
INSERT INTO week_locks (season, week, locks_at) VALUES
    ('2026', 1,  '2026-09-10 00:20:00+00'),  -- Wed Sep 9, 8:20pm EDT  (NE @ SEA opener)
    ('2026', 2,  '2026-09-18 00:15:00+00'),  -- Thu Sep 17, 8:15pm EDT
    ('2026', 3,  '2026-09-25 00:15:00+00'),  -- Thu Sep 24, 8:15pm EDT
    ('2026', 4,  '2026-10-02 00:15:00+00'),  -- Thu Oct 1,  8:15pm EDT
    ('2026', 5,  '2026-10-09 00:15:00+00'),  -- Thu Oct 8,  8:15pm EDT
    ('2026', 6,  '2026-10-16 00:15:00+00'),  -- Thu Oct 15, 8:15pm EDT
    ('2026', 7,  '2026-10-23 00:15:00+00'),  -- Thu Oct 22, 8:15pm EDT
    ('2026', 8,  '2026-10-30 00:15:00+00'),  -- Thu Oct 29, 8:15pm EDT
    ('2026', 9,  '2026-11-06 01:15:00+00'),  -- Thu Nov 5,  8:15pm EST
    ('2026', 10, '2026-11-13 01:15:00+00'),  -- Thu Nov 12, 8:15pm EST
    ('2026', 11, '2026-11-20 01:15:00+00'),  -- Thu Nov 19, 8:15pm EST
    ('2026', 13, '2026-12-04 01:15:00+00'),  -- Thu Dec 3,  8:15pm EST
    ('2026', 14, '2026-12-11 01:15:00+00'),  -- Thu Dec 10, 8:15pm EST
    ('2026', 15, '2026-12-18 01:15:00+00'),  -- Thu Dec 17, 8:15pm EST
    ('2026', 16, '2026-12-25 01:15:00+00'),  -- Thu Dec 24, 8:15pm EST
    ('2026', 17, '2027-01-01 01:15:00+00');  -- Thu Dec 31, 8:15pm EST
-- Weeks 12 and 18 intentionally NOT seeded (fail open = unlocked) until confirmed:
--   Week 12 = Thanksgiving week (Nov 26); first game may be a Wed game or the 1:00pm ET
--     Thanksgiving opener — sources conflict, do not guess.
--   Week 18 (Sat Jan 9 2027) kickoff times are flexed/TBD this far out.
-- Fill these two rows once the real first-game kickoffs are confirmed.

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

