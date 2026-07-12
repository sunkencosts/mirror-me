-- Split identity into a unique handle (username) and a free-form, non-unique display name.
-- Until now `username` doubled as both the unique key and the shown name; `display_name` is
-- the new user-facing label. Existing rows are backfilled from their current username so the
-- NOT NULL column has a sensible value. Username uniqueness also becomes case-insensitive:
-- the old case-sensitive UNIQUE constraint is replaced by a unique index on lower(username),
-- so "Bold_Hawk" and "bold_hawk" can no longer both exist.
ALTER TABLE users ADD COLUMN display_name text NOT NULL DEFAULT '';
UPDATE users SET display_name = username WHERE display_name = '';

ALTER TABLE users DROP CONSTRAINT users_username_key;
CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));
