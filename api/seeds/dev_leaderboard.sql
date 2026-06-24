-- Dev-only seed for eyeballing the leaderboard endpoints without live NFL data.
-- Inserts a handful of accounts and graded week_results for season 2026 in league
-- 'dev-std'. Safe to re-run (idempotent upserts). Apply with:
--   PGPASSWORD=mirrorleague psql -h localhost -p 5433 -U mirrorleague -d mirrorleague \
--     -f api/seeds/dev_leaderboard.sql
-- Then GET http://localhost:8080/leaderboard?season=2026
--  and  GET http://localhost:8080/league/dev-std/leaderboard?season=2026

INSERT INTO users (id, oauth_provider, oauth_id, email, username) VALUES
  ('00000000-0000-0000-0000-0000000000d1', 'dev', 'dev-d1', 'ana@dev.local',   'ana_sharp'),
  ('00000000-0000-0000-0000-0000000000d2', 'dev', 'dev-d2', 'ben@dev.local',   'ben_steady'),
  ('00000000-0000-0000-0000-0000000000d3', 'dev', 'dev-d3', 'cora@dev.local',  'cora_clutch'),
  ('00000000-0000-0000-0000-0000000000d4', 'dev', 'dev-d4', 'dan@dev.local',   'dan_dart'),
  ('00000000-0000-0000-0000-0000000000d5', 'dev', 'dev-d5', 'evan@dev.local',  'evan_eh'),
  ('00000000-0000-0000-0000-0000000000d6', 'dev', 'dev-d6', 'fiona@dev.local', 'fiona_fresh')
ON CONFLICT (oauth_provider, oauth_id) DO NOTHING;

-- (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result)
-- optimal_total is 150 across the board; official is 140; user varies so efficiency/edge/
-- win-rate differ. fiona has only 2 weeks → provisional on the GLOBAL board (3-week gate),
-- but ranked immediately on the per-league board.
INSERT INTO week_results (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result) VALUES
  ('00000000-0000-0000-0000-0000000000d1', 'dev-std', 1, 1, '2026', 148, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d1', 'dev-std', 1, 2, '2026', 150, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d1', 'dev-std', 1, 3, '2026', 146, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d1', 'dev-std', 1, 4, '2026', 149, 140, 150, 'user'),

  ('00000000-0000-0000-0000-0000000000d2', 'dev-std', 2, 1, '2026', 142, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d2', 'dev-std', 2, 2, '2026', 138, 140, 150, 'official'),
  ('00000000-0000-0000-0000-0000000000d2', 'dev-std', 2, 3, '2026', 145, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d2', 'dev-std', 2, 4, '2026', 140, 140, 150, 'tie'),

  ('00000000-0000-0000-0000-0000000000d3', 'dev-std', 3, 1, '2026', 150, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d3', 'dev-std', 3, 2, '2026', 150, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d3', 'dev-std', 3, 3, '2026', 135, 140, 150, 'official'),
  ('00000000-0000-0000-0000-0000000000d3', 'dev-std', 3, 4, '2026', 150, 140, 150, 'user'),

  ('00000000-0000-0000-0000-0000000000d4', 'dev-std', 4, 1, '2026', 130, 140, 150, 'official'),
  ('00000000-0000-0000-0000-0000000000d4', 'dev-std', 4, 2, '2026', 135, 140, 150, 'official'),
  ('00000000-0000-0000-0000-0000000000d4', 'dev-std', 4, 3, '2026', 140, 140, 150, 'tie'),
  ('00000000-0000-0000-0000-0000000000d4', 'dev-std', 4, 4, '2026', 138, 140, 150, 'official'),

  ('00000000-0000-0000-0000-0000000000d5', 'dev-std', 5, 1, '2026', 144, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d5', 'dev-std', 5, 2, '2026', 141, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d5', 'dev-std', 5, 3, '2026', 143, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d5', 'dev-std', 5, 4, '2026', 142, 140, 150, 'user'),

  ('00000000-0000-0000-0000-0000000000d6', 'dev-std', 6, 1, '2026', 149, 140, 150, 'user'),
  ('00000000-0000-0000-0000-0000000000d6', 'dev-std', 6, 2, '2026', 147, 140, 150, 'user')
ON CONFLICT (user_id, league_id, roster_id, week, season)
DO UPDATE SET user_total = EXCLUDED.user_total, official_total = EXCLUDED.official_total,
             optimal_total = EXCLUDED.optimal_total, result = EXCLUDED.result;
