package models

import "time"

type ParticipantStatus string

const (
	StatusApplicationSubmitted ParticipantStatus = "application_submitted"
	StatusApplicationRejected  ParticipantStatus = "application_rejected"
	StatusParticipant          ParticipantStatus = "participant"
)

type Participant struct {
	ID           int               `json:"id"`
	UserID       *int              `json:"user_id,omitempty"`
	TeamID       *int              `json:"team_id,omitempty"`
	TournamentID int               `json:"tournament_id"`
	Status       ParticipantStatus `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
}
