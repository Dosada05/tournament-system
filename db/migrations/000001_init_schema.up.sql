-- +migrate Up

-- Типы ENUM для статусов и ролей для повышения типобезопасности
CREATE TYPE user_role AS ENUM ('admin', 'organizer', 'player');
CREATE TYPE tournament_status AS ENUM ('soon', 'registration', 'active', 'completed', 'canceled');
CREATE TYPE participant_status AS ENUM ('application_submitted', 'application_rejected', 'participant');
CREATE TYPE match_status AS ENUM ('scheduled', 'in_progress', 'completed', 'canceled');

-- Таблица спорта
CREATE TABLE sports (
                        id SERIAL PRIMARY KEY,
                        name VARCHAR(50) NOT NULL UNIQUE
);
CREATE INDEX idx_sports_name ON sports (name); -- Индекс для поиска по имени

-- Таблица форматов
CREATE TABLE formats (
                         id SERIAL PRIMARY KEY,
                         name VARCHAR(50) UNIQUE NOT NULL
);
CREATE INDEX idx_formats_name ON formats (name); -- Индекс для поиска по имени

-- Таблица команд (создаем до users, т.к. users ссылается на teams)
CREATE TABLE teams (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(100) NOT NULL UNIQUE,
                       sport_id INT NOT NULL,
                       captain_id INT NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_teams_name ON teams (name);
CREATE INDEX idx_teams_sport_id ON teams (sport_id);
CREATE INDEX idx_teams_captain_id ON teams (captain_id);

CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       first_name VARCHAR(50) NOT NULL,
                       last_name VARCHAR(50) NOT NULL,
                       nickname VARCHAR(50),
                       team_id INT,
                       role user_role NOT NULL,
                       email VARCHAR(100) UNIQUE NOT NULL,
                       password_hash VARCHAR(255) NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE users ADD CONSTRAINT fk_users_team FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE SET NULL;
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_team_id ON users (team_id);
CREATE INDEX idx_users_role ON users (role);

-- Теперь добавляем внешние ключи для teams, т.к. users и sports созданы
ALTER TABLE teams ADD CONSTRAINT fk_teams_sport FOREIGN KEY (sport_id) REFERENCES sports (id) ON DELETE RESTRICT; -- Нельзя удалить спорт, если есть команды
ALTER TABLE teams ADD CONSTRAINT fk_teams_captain FOREIGN KEY (captain_id) REFERENCES users (id) ON DELETE RESTRICT; -- Нельзя удалить капитана, если он капитан команды

-- Таблица турниров
CREATE TABLE tournaments (
                             id SERIAL PRIMARY KEY,
                             name VARCHAR(100) NOT NULL,
                             description TEXT,
                             sport_id INT NOT NULL,
                             format_id INT NOT NULL,
                             organizer_id INT NOT NULL,
                             reg_date TIMESTAMPTZ NOT NULL,
                             start_date TIMESTAMPTZ NOT NULL,
                             end_date TIMESTAMPTZ NOT NULL,
                             location VARCHAR(100),
                             status tournament_status NOT NULL,
                             max_participants INT NOT NULL,
                             created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                             FOREIGN KEY (format_id) REFERENCES formats (id) ON DELETE RESTRICT,
                             FOREIGN KEY (organizer_id) REFERENCES users (id) ON DELETE RESTRICT,
                             FOREIGN KEY (sport_id) REFERENCES sports (id) ON DELETE RESTRICT
);

CREATE INDEX idx_tournaments_sport_id ON tournaments (sport_id);
CREATE INDEX idx_tournaments_format_id ON tournaments (format_id);
CREATE INDEX idx_tournaments_organizer_id ON tournaments (organizer_id);
CREATE INDEX idx_tournaments_status ON tournaments (status);
CREATE INDEX idx_tournaments_start_date ON tournaments (start_date);

-- Таблица участников
CREATE TABLE participants (
                              id SERIAL PRIMARY KEY,
                              user_id INT,
                              team_id INT,
                              tournament_id INT NOT NULL,
                              status participant_status NOT NULL,
                              created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
                              FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE CASCADE,
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              CONSTRAINT chk_participant_type CHECK ((user_id IS NOT NULL AND team_id IS NULL) OR (user_id IS NULL AND team_id IS NOT NULL)), -- Строго: или юзер, или команда
                              UNIQUE (user_id, tournament_id),
                              UNIQUE (team_id, tournament_id)
);

CREATE INDEX idx_participants_user_id ON participants (user_id);
CREATE INDEX idx_participants_team_id ON participants (team_id);
CREATE INDEX idx_participants_tournament_id ON participants (tournament_id);
CREATE INDEX idx_participants_status ON participants (status);

-- Таблицы матчей
CREATE TABLE solo_matches (
                              id SERIAL PRIMARY KEY,
                              tournament_id INT NOT NULL,
                              p1_participant_id INT,
                              p2_participant_id INT,
                              score VARCHAR(50),
                              match_time TIMESTAMPTZ NOT NULL,
                              status match_status NOT NULL,
                              winner_participant_id INT,
                              round INT,
                              created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              bracket_match_uid VARCHAR(255) UNIQUE,
                              next_match_db_id INT,
                              winner_to_slot SMALLINT,

                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              FOREIGN KEY (p1_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (p2_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (winner_participant_id) REFERENCES participants (id) ON DELETE SET NULL,
                              FOREIGN KEY (next_match_db_id) REFERENCES solo_matches (id) ON DELETE SET NULL, -- Ссылка на самого себя
                              CONSTRAINT chk_solo_distinct_participants CHECK (p1_participant_id IS NULL OR p2_participant_id IS NULL OR p1_participant_id <> p2_participant_id),
                              CONSTRAINT chk_solo_winner CHECK (winner_participant_id IS NULL OR winner_participant_id = p1_participant_id OR winner_participant_id = p2_participant_id),
                              CONSTRAINT chk_winner_to_slot CHECK (winner_to_slot IS NULL OR winner_to_slot IN (1, 2))
);
CREATE INDEX idx_solo_matches_bracket_match_uid ON solo_matches (bracket_match_uid);
CREATE INDEX idx_solo_matches_tournament_id ON solo_matches (tournament_id);
CREATE INDEX idx_solo_matches_p1_participant_id ON solo_matches (p1_participant_id);
CREATE INDEX idx_solo_matches_p2_participant_id ON solo_matches (p2_participant_id);
CREATE INDEX idx_solo_matches_winner_participant_id ON solo_matches (winner_participant_id);
CREATE INDEX idx_solo_matches_status ON solo_matches (status);
CREATE INDEX idx_solo_matches_match_time ON solo_matches (match_time);

CREATE TABLE team_matches (
                              id SERIAL PRIMARY KEY,
                              tournament_id INT NOT NULL,
                              t1_participant_id INT,
                              t2_participant_id INT,
                              score VARCHAR(50),
                              match_time TIMESTAMPTZ NOT NULL,
                              status match_status NOT NULL,
                              winner_participant_id INT,
                              round INT,
                              created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

                              bracket_match_uid VARCHAR(255) UNIQUE,
                              next_match_db_id INT,
                              winner_to_slot SMALLINT,

                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              FOREIGN KEY (t1_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (t2_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (winner_participant_id) REFERENCES participants (id) ON DELETE SET NULL,
                              FOREIGN KEY (next_match_db_id) REFERENCES team_matches (id) ON DELETE SET NULL, -- Ссылка на самого себя
                              CONSTRAINT chk_team_distinct_participants CHECK (t1_participant_id IS NULL OR t2_participant_id IS NULL OR t1_participant_id <> t2_participant_id),
                              CONSTRAINT chk_team_winner CHECK (winner_participant_id IS NULL OR winner_participant_id = t1_participant_id OR winner_participant_id = t2_participant_id),
                              CONSTRAINT chk_team_winner_to_slot CHECK (winner_to_slot IS NULL OR winner_to_slot IN (1, 2))
);
CREATE INDEX idx_team_matches_bracket_match_uid ON team_matches (bracket_match_uid);
CREATE INDEX idx_team_matches_tournament_id ON team_matches (tournament_id);
CREATE INDEX idx_team_matches_t1_participant_id ON team_matches (t1_participant_id);
CREATE INDEX idx_team_matches_t2_participant_id ON team_matches (t2_participant_id);
CREATE INDEX idx_team_matches_winner_participant_id ON team_matches (winner_participant_id);
CREATE INDEX idx_team_matches_status ON team_matches (status);
CREATE INDEX idx_team_matches_match_time ON team_matches (match_time);

-- Таблица приглашений
CREATE TABLE invites (
                         id SERIAL PRIMARY KEY,
                         team_id INTEGER NOT NULL,
                         token VARCHAR(64) NOT NULL UNIQUE,
                         expires_at TIMESTAMPTZ NOT NULL,
                         created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                         FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
);
CREATE INDEX idx_invites_token ON invites (token);
CREATE INDEX idx_invites_expires_at ON invites (expires_at);
CREATE INDEX idx_invites_team_id ON invites (team_id);

-- +migrate StatementEnd


-- Добавляем поле для ключа логотипа в таблицу users
ALTER TABLE users
    ADD COLUMN logo_key VARCHAR(255);

-- Добавляем поле для ключа логотипа в таблицу teams
ALTER TABLE teams
    ADD COLUMN logo_key VARCHAR(255);

-- Добавляем поле для ключа логотипа в таблицу sports
ALTER TABLE sports
    ADD COLUMN logo_key VARCHAR(255);

-- Добавляем поле для ключа логотипа в таблицу tournaments
ALTER TABLE tournaments
    ADD COLUMN logo_key VARCHAR(255);

-- Новая миграция для изменения таблицы formats
ALTER TABLE formats
    ADD COLUMN bracket_type VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN participant_type VARCHAR(10) NOT NULL DEFAULT 'solo',
    ADD COLUMN settings_json TEXT;

-- Можно добавить CHECK constraint для participant_type
ALTER TABLE formats ADD CONSTRAINT chk_format_participant_type CHECK (participant_type IN ('solo', 'team'));


-- Таблица для хранения турнирной таблицы (положения участников) для круговых турниров
CREATE TABLE tournament_standings (
                                      id SERIAL PRIMARY KEY,
                                      tournament_id INT NOT NULL,
                                      participant_id INT NOT NULL,
                                      points INT NOT NULL DEFAULT 0,
                                      games_played INT NOT NULL DEFAULT 0,
                                      wins INT NOT NULL DEFAULT 0,
                                      draws INT NOT NULL DEFAULT 0,
                                      losses INT NOT NULL DEFAULT 0,
                                      score_for INT NOT NULL DEFAULT 0, -- Забитые голы/очки
                                      score_against INT NOT NULL DEFAULT 0, -- Пропущенные голы/очки
                                      score_difference INT NOT NULL DEFAULT 0, -- Разница (score_for - score_against)
                                      rank INT, -- Опционально, может вычисляться при запросе
                                      updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                      FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                                      FOREIGN KEY (participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                                      UNIQUE (tournament_id, participant_id)
);

CREATE INDEX idx_tournament_standings_tournament_id ON tournament_standings (tournament_id);
CREATE INDEX idx_tournament_standings_participant_id ON tournament_standings (participant_id);
CREATE INDEX idx_tournament_standings_ranking ON tournament_standings (tournament_id, points DESC, score_difference DESC, score_for DESC);

CREATE OR REPLACE FUNCTION update_standings_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tournament_standings_updated_at
    BEFORE UPDATE ON tournament_standings
    FOR EACH ROW
EXECUTE FUNCTION update_standings_updated_at_column();

DO $$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='formats' AND column_name='bracket_type') THEN
            ALTER TABLE formats ADD COLUMN bracket_type VARCHAR(50) NOT NULL DEFAULT 'SingleElimination';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='formats' AND column_name='participant_type') THEN
            ALTER TABLE formats ADD COLUMN participant_type VARCHAR(10) NOT NULL DEFAULT 'solo';
            -- Добавляем CHECK constraint после добавления колонки, если его еще нет
            IF NOT EXISTS (SELECT 1 FROM information_schema.constraint_column_usage WHERE table_name='formats' AND constraint_name='chk_format_participant_type') THEN
                ALTER TABLE formats ADD CONSTRAINT chk_format_participant_type CHECK (participant_type IN ('solo', 'team'));
            END IF;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='formats' AND column_name='settings_json') THEN
            ALTER TABLE formats ADD COLUMN settings_json TEXT;
        END IF;
    END $$;


DO $$
    BEGIN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'chk_format_participant_type' AND conrelid = 'formats'::regclass
        ) THEN
            ALTER TABLE formats ADD CONSTRAINT chk_format_participant_type CHECK (participant_type IN ('solo', 'team'));
        END IF;
    END $$;

ALTER TABLE tournaments
    ADD CONSTRAINT tournaments_organizer_id_name_key UNIQUE (organizer_id, name);

ALTER TABLE tournaments
    ADD COLUMN overall_winner_participant_id INT,
    ADD CONSTRAINT fk_tournaments_overall_winner FOREIGN KEY (overall_winner_participant_id) REFERENCES participants(id) ON DELETE SET NULL;

CREATE INDEX idx_tournaments_overall_winner_participant_id ON tournaments (overall_winner_participant_id);


