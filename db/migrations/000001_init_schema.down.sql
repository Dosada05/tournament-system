-- +migrate Down

-- 1. Удаляем таблицы, на которые никто не ссылается
DROP TABLE IF EXISTS invites CASCADE;
DROP TABLE IF EXISTS team_matches CASCADE;
DROP TABLE IF EXISTS solo_matches CASCADE;
DROP TABLE IF EXISTS team_matches CASCADE;
DROP TABLE IF EXISTS participants CASCADE;
DROP TABLE IF EXISTS tournaments CASCADE;

-- 2. Удаляем внешние ключи ПЕРЕД удалением таблиц, на которые они ссылаются
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_captain_id_fkey;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_team;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_sport;

-- 3. Удаляем таблицы в правильном порядке зависимостей
DROP TABLE IF EXISTS users CASCADE; -- Теперь можно удалить users
DROP TABLE IF EXISTS teams CASCADE; -- Теперь можно удалить teams
DROP TABLE IF EXISTS formats CASCADE;
DROP TABLE IF EXISTS sports CASCADE;

-- 4. Удаляем ENUM типы (они больше не используются таблицами)
DROP TYPE IF EXISTS match_status;
DROP TYPE IF EXISTS participant_status;
DROP TYPE IF EXISTS tournament_status;
DROP TYPE IF EXISTS user_role;

ALTER TABLE formats
    DROP COLUMN settings_json,
    DROP COLUMN participant_type,
    DROP COLUMN bracket_type;

DROP TRIGGER IF EXISTS update_tournament_standings_updated_at ON tournament_standings;
DROP FUNCTION IF EXISTS update_standings_updated_at_column();
DROP TABLE IF EXISTS tournament_standings CASCADE;

ALTER TABLE tournaments DROP CONSTRAINT IF EXISTS tournaments_organizer_id_name_key;

ALTER TABLE tournaments DROP CONSTRAINT IF EXISTS fk_tournaments_overall_winner;
ALTER TABLE tournaments DROP COLUMN IF EXISTS overall_winner_participant_id;


-- +migrate StatementEnd
