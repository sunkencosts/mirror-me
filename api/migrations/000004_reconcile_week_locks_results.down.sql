-- No-op. This is a reconciliation migration: week_locks and week_results are
-- conceptually owned by 000001_init, which already drops them in its own .down.
-- Rolling back this reconciliation must NOT drop tables the schema still needs,
-- otherwise a `migrate down 1` on a healthy DB would leave it missing tables that
-- version 3 should still have. Intentionally empty.
SELECT 1;
