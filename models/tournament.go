package models

import "time"

type TournamentStatus string

const (
	StatusSoon         TournamentStatus = "soon"
	StatusRegistration TournamentStatus = "registration"
	StatusActive       TournamentStatus = "active"
	StatusCompleted    TournamentStatus = "completed"
	StatusCanceled     TournamentStatus = "canceled"
)

type Tournament struct {
	ID              int              `json:"id"`
	Name            string           `json:"name"`
	Description     *string          `json:"description,omitempty"`
	SportID         int              `json:"sport_id"`
	FormatID        int              `json:"format_id"`
	OrganizerID     int              `json:"organizer_id"`
	RegDate         time.Time        `json:"reg_date"`
	StartDate       time.Time        `json:"start_date"`
	EndDate         time.Time        `json:"end_date"`
	Location        *string          `json:"location,omitempty"`
	Status          TournamentStatus `json:"status"`
	MaxParticipants int              `json:"max_participants"`
}
