-- Create users table
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

-- Create teams table
CREATE TABLE teams (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(100) NOT NULL
);

-- Create tournaments table
CREATE TABLE tournaments (
                             id SERIAL PRIMARY KEY,
                             name VARCHAR(100) NOT NULL,
                             description TEXT,
                             sport_type VARCHAR(50) NOT NULL,
                             format VARCHAR(50) NOT NULL,
                             organizer_id INT NOT NULL,
                             reg_date TIMESTAMP NOT NULL,
                             start_date TIMESTAMP NOT NULL,
                             end_date TIMESTAMP NOT NULL,
                             location VARCHAR(100),
                             status VARCHAR(50) NOT NULL,
                             max_participants INT NOT NULL,
                             FOREIGN KEY (organizer_id) REFERENCES users (id)
);

-- Create participants table
CREATE TABLE participants (
                              id SERIAL PRIMARY KEY,
                              user_id INT NOT NULL,
                              team_id INT,
                              tournament_id INT NOT NULL,
                              status VARCHAR(50) NOT NULL,
                              created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                              FOREIGN KEY (user_id) REFERENCES users (id),
                              FOREIGN KEY (team_id) REFERENCES teams (id),
                              FOREIGN KEY (tournament_id) REFERENCES tournaments (id)
);

-- Create matches table
CREATE TABLE matches (
                         id SERIAL PRIMARY KEY,
                         tournament_id INT NOT NULL,
                         p1_id INT NOT NULL,
                         p2_id INT NOT NULL,
                         score VARCHAR(50),
                         date TIMESTAMP NOT NULL,
                         winner_id INT,
                         FOREIGN KEY (tournament_id) REFERENCES tournaments (id),
                         FOREIGN KEY (p1_id) REFERENCES users (id),
                         FOREIGN KEY (p2_id) REFERENCES users (id),
                         FOREIGN KEY (winner_id) REFERENCES users (id)
);