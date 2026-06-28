-- Reconciliation migration. week_locks and week_results were added to 000001_init
-- AFTER it had already been applied on the production Pi, so golang-migrate never
-- re-ran version 1 there and these two tables were never created (every current-week
-- inference 500'd with "relation \"week_locks\" does not exist"). This migration
-- creates them on any DB that's missing them. On local/dev/fresh DBs they already
-- exist from 000001, so IF NOT EXISTS / ON CONFLICT make this a harmless no-op.
-- Definitions are copied verbatim from 000001_init.up.sql — keep them in sync.

CREATE TABLE IF NOT EXISTS week_locks(
    season text NOT NULL,
    week int NOT NULL,
    locks_at timestamptz NOT NULL,
    PRIMARY KEY (season, week)
);

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
    ('2026', 17, '2027-01-01 01:15:00+00')   -- Thu Dec 31, 8:15pm EST
ON CONFLICT (season, week) DO NOTHING;

CREATE TABLE IF NOT EXISTS week_results(
    user_id text NOT NULL,
    league_id text NOT NULL,
    roster_id int NOT NULL,
    week int NOT NULL,
    season text NOT NULL,
    user_total double precision NOT NULL,
    official_total double precision NOT NULL,
    optimal_total double precision NOT NULL,
    result text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, league_id, roster_id, week, season)
);
