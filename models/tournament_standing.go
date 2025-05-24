package models

import "time"

type TournamentStanding struct {
	ID              int       `json:"id" db:"id"`
	TournamentID    int       `json:"tournament_id" db:"tournament_id"`
	ParticipantID   int       `json:"participant_id" db:"participant_id"`
	Points          int       `json:"points" db:"points"`
	GamesPlayed     int       `json:"games_played" db:"games_played"`
	Wins            int       `json:"wins" db:"wins"`
	Draws           int       `json:"draws" db:"draws"`
	Losses          int       `json:"losses" db:"losses"`
	ScoreFor        int       `json:"score_for" db:"score_for"`
	ScoreAgainst    int       `json:"score_against" db:"score_against"`
	ScoreDifference int       `json:"score_difference" db:"score_difference"`
	Rank            *int      `json:"rank,omitempty" db:"rank"` // Nullable, can be calculated
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	// Optional linked data, not directly in DB table, populated by service
	Participant *Participant `json:"participant,omitempty" db:"-"`
	Tournament  *Tournament  `json:"-" db:"-"` // Avoid circular ref in JSON if not needed
}
