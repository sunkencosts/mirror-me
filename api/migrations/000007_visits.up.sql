-- First-party page-view analytics. One append-only row per SPA navigation, written by
-- POST /collect via a fetch keepalive beacon. Closes the "anonymous hole": Cloudflare
-- edge stats and the users table can't tell whether real (incl. anonymous) humans browse
-- the site. visitor_id reuses the existing first-party localStorage anon id (no new cookie);
-- user_id is stamped only when the beacon carries a valid auth JWT, so it is NULL while a
-- visitor is anonymous. Sessions are computed at read time (30-min inactivity gap), so no
-- session column is stored. country comes from Cloudflare's CF-IPCountry header — no raw IP.
CREATE TABLE visits (
    id         bigserial   PRIMARY KEY,
    visitor_id text        NOT NULL,
    user_id    text,
    path       text        NOT NULL,
    referrer   text,
    country    text,
    is_bot     boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_visits_created_at ON visits (created_at);
CREATE INDEX idx_visits_visitor_created ON visits (visitor_id, created_at);
