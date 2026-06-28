-- num_teams: the league's team/roster count, cached alongside the rest of the league shape
-- so the My Leagues card (and anything else reading a cache-served league) reports the real
-- count instead of 0. Sleeper returns this in settings.num_teams; defaults to 0 for rows
-- written before this column existed (they refresh on the next live fetch / reseed).
ALTER TABLE leagues ADD COLUMN num_teams int NOT NULL DEFAULT 0;
