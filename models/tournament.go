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
	ID              int              `json:"id" db:"id"`
	Name            string           `json:"name" db:"name"`
	Description     *string          `json:"description,omitempty" db:"description"`
	SportID         int              `json:"sport_id" db:"sport_id"`
	FormatID        int              `json:"format_id" db:"format_id"`
	OrganizerID     int              `json:"organizer_id" db:"organizer_id"`
	RegDate         time.Time        `json:"reg_date" db:"reg_date"`
	StartDate       time.Time        `json:"start_date" db:"start_date"`
	EndDate         time.Time        `json:"end_date" db:"end_date"`
	Location        *string          `json:"location,omitempty" db:"location"`
	Status          TournamentStatus `json:"status" db:"status"`
	MaxParticipants int              `json:"max_participants" db:"max_participants"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	LogoKey         *string          `json:"-" db:"logo_key"`
	LogoURL         *string          `json:"logo_url,omitempty" db:"-"`

	OverallWinnerParticipantID *int `json:"overall_winner_participant_id,omitempty" db:"overall_winner_participant_id"` // Added

	Sport        *Sport        `json:"sport,omitempty" db:"-"`
	Format       *Format       `json:"format,omitempty" db:"-"`
	Organizer    *User         `json:"organizer,omitempty" db:"-"`
	Participants []Participant `json:"participants,omitempty" db:"-"`
	SoloMatches  []SoloMatch   `json:"solo_matches,omitempty" db:"-"`
	TeamMatches  []TeamMatch   `json:"team_matches,omitempty" db:"-"`
}
