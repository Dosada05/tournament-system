package models

import "time"

type Participant struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	TeamID       int       `json:"team_id"`
	TournamentID int       `json:"tournament_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
