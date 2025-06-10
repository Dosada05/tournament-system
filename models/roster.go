// File: models/roster.go
package models

import "time"

type TournamentTeamRoster struct {
	ID            int       `json:"id" db:"id"`
	ParticipantID int       `json:"participant_id" db:"participant_id"`
	UserID        int       `json:"user_id" db:"user_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`

	Participant *Participant `json:"participant,omitempty" db:"-"`
	User        *User        `json:"user,omitempty" db:"-"`
}
