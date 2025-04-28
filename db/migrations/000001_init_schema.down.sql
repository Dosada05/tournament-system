-- Drop users table
DROP TABLE IF EXISTS users CASCADE;

DROP TABLE IF EXISTS sports CASCADE ;

-- Drop matches table
DROP TABLE IF EXISTS solo_matches CASCADE;
DROP TABLE IF EXISTS team_matches CASCADE;

-- Drop participants table
DROP TABLE IF EXISTS participants CASCADE;

-- Drop tournaments table
DROP TABLE IF EXISTS tournaments CASCADE;

-- Drop teams table
DROP TABLE IF EXISTS teams CASCADE;

DROP TABLE IF EXISTS formats CASCADE;

-- Удаляем типы, если они уже существуют
DROP TYPE IF EXISTS tournament_status CASCADE;
DROP TYPE IF EXISTS participant_status CASCADE;
DROP TYPE IF EXISTS match_status CASCADE;