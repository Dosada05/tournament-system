package models

import "time"

type Participant struct {
	ID           int    `json:"id"`
	TeamID       *int   `json:"team_id,omitempty"`
	UserID       *int   `json:"user_id,omitempty"`
	TournamentID int    `json:"tournament_id"`
	Status       string `json:"status"`
	CreatedAt    time.Time
}
