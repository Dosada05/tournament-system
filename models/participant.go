package models

import "time"

// ParticipantStatus представляет статусы участника турнира, соответствующие ENUM в БД.
type ParticipantStatus string

const (
	StatusApplicationSubmitted ParticipantStatus = "application_submitted"
	StatusApplicationRejected  ParticipantStatus = "application_rejected"
	StatusParticipant          ParticipantStatus = "participant"
)

type Participant struct {
	ID           int               `json:"id" db:"id"`
	UserID       *int              `json:"user_id,omitempty" db:"user_id"`
	TeamID       *int              `json:"team_id,omitempty" db:"team_id"`
	TournamentID int               `json:"tournament_id" db:"tournament_id"`
	Status       ParticipantStatus `json:"status" db:"status"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`

	User            *User       `json:"user,omitempty" db:"-"`
	Team            *Team       `json:"team,omitempty" db:"-"`
	Tournament      *Tournament `json:"tournament,omitempty" db:"-"`
	SoloMatchesAsP1 []SoloMatch `json:"solo_matches_as_p1,omitempty" db:"-"` // Матчи, где этот участник P1
	SoloMatchesAsP2 []SoloMatch `json:"solo_matches_as_p2,omitempty" db:"-"` // Матчи, где этот участник P2
	TeamMatchesAsT1 []TeamMatch `json:"team_matches_as_t1,omitempty" db:"-"` // Матчи, где этот участник T1
	TeamMatchesAsT2 []TeamMatch `json:"team_matches_as_t2,omitempty" db:"-"` // Матчи, где этот участник T2
}
