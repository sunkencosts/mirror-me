DROP INDEX users_username_lower_key;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
ALTER TABLE users DROP COLUMN display_name;
