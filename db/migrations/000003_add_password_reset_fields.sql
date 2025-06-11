-- Добавление полей для сброса пароля
ALTER TABLE users
ADD COLUMN password_reset_token TEXT,
ADD COLUMN password_reset_expires_at TIMESTAMPTZ;

