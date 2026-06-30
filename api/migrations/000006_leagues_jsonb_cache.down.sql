ALTER TABLE leagues
  ADD COLUMN name text NOT NULL DEFAULT '',
  ADD COLUMN season text NOT NULL DEFAULT '',
  ADD COLUMN leg integer NOT NULL DEFAULT 0,
  ADD COLUMN num_teams integer NOT NULL DEFAULT 0,
  ADD COLUMN roster_positions text[] NOT NULL DEFAULT '{}';

ALTER TABLE leagues DROP COLUMN data;
