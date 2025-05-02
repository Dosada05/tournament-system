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

-- Таблица пользователей
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

-- Добавляем внешний ключ к teams после создания таблицы teams
ALTER TABLE users ADD CONSTRAINT fk_users_team FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE SET NULL; -- При удалении команды у пользователя team_id станет NULL
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
                              p1_participant_id INT NOT NULL,
                              p2_participant_id INT CHECK (p1_participant_id <> p2_participant_id),
                              score VARCHAR(50),
                              match_time TIMESTAMPTZ NOT NULL,
                              status match_status NOT NULL,
                              winner_participant_id INT CHECK (winner_participant_id IS NULL OR winner_participant_id = p1_participant_id OR winner_participant_id = p2_participant_id),
                              round INT,
                              created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              FOREIGN KEY (p1_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (p2_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (winner_participant_id) REFERENCES participants (id) ON DELETE SET NULL
);
CREATE INDEX idx_solo_matches_tournament_id ON solo_matches (tournament_id);
CREATE INDEX idx_solo_matches_p1_participant_id ON solo_matches (p1_participant_id);
CREATE INDEX idx_solo_matches_p2_participant_id ON solo_matches (p2_participant_id);
CREATE INDEX idx_solo_matches_winner_participant_id ON solo_matches (winner_participant_id);
CREATE INDEX idx_solo_matches_status ON solo_matches (status);
CREATE INDEX idx_solo_matches_match_time ON solo_matches (match_time);

CREATE TABLE team_matches (
                              id SERIAL PRIMARY KEY,
                              tournament_id INT NOT NULL,
                              t1_participant_id INT NOT NULL,
                              t2_participant_id INT CHECK (t1_participant_id <> t2_participant_id),
                              score VARCHAR(50),
                              match_time TIMESTAMPTZ NOT NULL,
                              status match_status NOT NULL,
                              winner_participant_id INT CHECK (winner_participant_id IS NULL OR winner_participant_id = t1_participant_id OR winner_participant_id = t2_participant_id), -- Ссылка на победившего участника
                              round INT,
                              created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              FOREIGN KEY (t1_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (t2_participant_id) REFERENCES participants (id) ON DELETE CASCADE,
                              FOREIGN KEY (winner_participant_id) REFERENCES participants (id) ON DELETE SET NULL
);
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