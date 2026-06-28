-- No-op. lineups.season is conceptually owned by 000001_init. Rolling back this
-- reconciliation must NOT drop a column the schema still needs. Intentionally empty.
SELECT 1;
