-- Reconciliation, continued. lineups.season was retro-added to 000001_init AFTER it
-- had already been applied on the production Pi, so prod's lineups table is missing
-- the column and every ListLineups/GetLineup SELECT 500'd with
-- "column \"season\" does not exist". (This is a separate migration from 000004 because
-- 000004 was itself already applied on prod — never edit an applied migration.)
-- IF NOT EXISTS makes this a harmless no-op on DBs that already have the column.
ALTER TABLE lineups ADD COLUMN IF NOT EXISTS season text NOT NULL DEFAULT '';
