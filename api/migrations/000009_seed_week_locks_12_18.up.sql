-- GH #14: weeks 12 and 18 of the 2026 season were left unseeded in 000001_init.up.sql
-- pending confirmation of the real slate. A missing week_locks row now fails CLOSED for
-- any past week (see weekLocked in internal/handlers/lineup.go), so leaving these two
-- gaps in place would make weeks 12 and 18 permanently unplayable once they become
-- past weeks. Seed them now with a conservative EARLY kickoff estimate — early enough
-- that it is very unlikely to be later than the real first kickoff — and tighten once
-- the official slate is confirmed. Both dates fall after the Nov 1 2026 DST change, so
-- both are EST (UTC-5).
INSERT INTO week_locks (season, week, locks_at) VALUES
    ('2026', 12, '2026-11-26 17:30:00+00'),  -- Thu Nov 26, 12:30pm EST — traditional
                                              -- Thanksgiving early-game kickoff (Lions
                                              -- home game), the earliest NFL window of
                                              -- the week regardless of who plays in it.
    ('2026', 18, '2027-01-09 18:00:00+00');  -- Sat Jan 9 2027, 1:00pm EST — earliest
                                              -- standard NFL kickoff window; Week 18
                                              -- start times are flexed/TBD this far out,
                                              -- so this errs early on purpose.
