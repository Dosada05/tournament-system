-- +migrate Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_confirmed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_confirmation_token VARCHAR(255);

-- +migrate Down
ALTER TABLE users DROP COLUMN IF EXISTS email_confirmed;
ALTER TABLE users DROP COLUMN IF EXISTS email_confirmation_token;

