-- Cache the full league response as a single jsonb blob instead of hand-picked typed columns.
-- New fields from Sleeper (avatar, scoring settings, status, …) are then available to cache
-- consumers with no schema change and no per-field backfill: the whole provider.League round-
-- trips through `data`. Rows written before this migration get NULL data and are treated as a
-- cache miss, so they refetch live on next read — no manual backfill needed.
ALTER TABLE leagues ADD COLUMN data jsonb;

ALTER TABLE leagues
  DROP COLUMN name,
  DROP COLUMN season,
  DROP COLUMN leg,
  DROP COLUMN num_teams,
  DROP COLUMN roster_positions;
