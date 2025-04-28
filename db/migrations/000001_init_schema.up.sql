-- Таблица спорта
CREATE TABLE sports (
                        id SERIAL PRIMARY KEY,
                        name VARCHAR(50) NOT NULL UNIQUE
);

-- Таблица форматов
CREATE TABLE formats (
                         id SERIAL PRIMARY KEY,
                         name VARCHAR(50) UNIQUE NOT NULL
);

-- Таблица пользователей (без ограничения для team_id)
CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       first_name VARCHAR(50) NOT NULL,
                       last_name VARCHAR(50) NOT NULL,
                       nickname VARCHAR(50),
                       team_id INT,
                       role VARCHAR(20) NOT NULL,
                       email VARCHAR(100) UNIQUE NOT NULL,
                       password_hash VARCHAR(255) NOT NULL,
                       created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Таблица команд
CREATE TABLE teams (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(100) NOT NULL UNIQUE,
                       sport_id INT NOT NULL,
                       captain_id INT NOT NULL,
                       FOREIGN KEY (sport_id) REFERENCES sports (id),
                       FOREIGN KEY (captain_id) REFERENCES users (id)
);

-- Добавление ограничения для team_id в таблице users
ALTER TABLE users
    ADD CONSTRAINT fk_users_team FOREIGN KEY (team_id) REFERENCES teams (id);

-- Тип статуса турнира и таблица турниров
CREATE TYPE tournament_status AS ENUM ('soon', 'registration', 'active', 'completed', 'canceled');

CREATE TABLE tournaments (
                             id SERIAL PRIMARY KEY,
                             name VARCHAR(100) NOT NULL,
                             description TEXT,
                             sport_id INT NOT NULL,
                             format_id INT NOT NULL,
                             organizer_id INT NOT NULL,
                             reg_date TIMESTAMP NOT NULL,
                             start_date TIMESTAMP NOT NULL,
                             end_date TIMESTAMP NOT NULL,
                             location VARCHAR(100),
                             status tournament_status NOT NULL,
                             max_participants INT NOT NULL,
                             FOREIGN KEY (format_id) REFERENCES formats (id),
                             FOREIGN KEY (organizer_id) REFERENCES users (id),
                             FOREIGN KEY (sport_id) REFERENCES sports (id)
);

-- Тип статуса участника и таблица участников
CREATE TYPE participant_status AS ENUM ('application_submitted', 'application_rejected', 'participant');

CREATE TABLE participants (
                              id SERIAL PRIMARY KEY,
                              user_id INT,
                              team_id INT CHECK (user_id IS NULL OR team_id IS NULL),
                              tournament_id INT NOT NULL,
                              status participant_status NOT NULL,
                              created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              FOREIGN KEY (user_id) REFERENCES users (id),
                              FOREIGN KEY (team_id) REFERENCES teams (id),
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id) ON DELETE CASCADE,
                              CONSTRAINT chk_participant_user_or_team CHECK (user_id IS NULL OR team_id IS NULL),
                              UNIQUE (user_id, tournament_id),
                              UNIQUE (team_id, tournament_id)
);

-- Тип статуса матча и таблицы матчей
CREATE TYPE match_status AS ENUM ('scheduled', 'in_progress', 'completed', 'canceled');

CREATE TABLE solo_matches (
                              id SERIAL PRIMARY KEY,
                              tournament_id INT NOT NULL,
                              p1_id INT NOT NULL,
                              p2_id INT CHECK (p1_id <> p2_id),
                              score VARCHAR(50),
                              date TIMESTAMP NOT NULL,
                              status match_status NOT NULL,
                              winner_id INT CHECK (winner_id IS NULL OR winner_id = p1_id OR winner_id = p2_id),
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id),
                              FOREIGN KEY (p1_id) REFERENCES users (id),
                              FOREIGN KEY (p2_id) REFERENCES users (id),
                              FOREIGN KEY (winner_id) REFERENCES users (id)
);

CREATE TABLE team_matches (
                              id SERIAL PRIMARY KEY,
                              tournament_id INT NOT NULL,
                              t1_id INT NOT NULL,
                              t2_id INT CHECK (t1_id <> t2_id),
                              score VARCHAR(50),
                              date TIMESTAMP NOT NULL,
                              status match_status NOT NULL,
                              winner_id INT CHECK (winner_id IS NULL OR winner_id = t1_id OR winner_id = t2_id),
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id),
                              FOREIGN KEY (t1_id) REFERENCES teams (id),
                              FOREIGN KEY (t2_id) REFERENCES teams (id),
                              FOREIGN KEY (winner_id) REFERENCES teams (id)
);