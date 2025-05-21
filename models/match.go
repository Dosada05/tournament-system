package models

import "time"

// MatchStatus представляет статусы матча, соответствующие ENUM в БД.
type MatchStatus string

const (
	StatusScheduled      MatchStatus = "scheduled"
	StatusInProgress     MatchStatus = "in_progress"
	MatchStatusCompleted MatchStatus = "completed"
	MatchStatusCanceled  MatchStatus = "canceled"
)

type SoloMatch struct {
	ID                  int         `json:"id" db:"id"`
	TournamentID        int         `json:"tournament_id" db:"tournament_id"`
	P1ParticipantID     *int        `json:"p1_participant_id,omitempty" db:"p1_participant_id"`
	P2ParticipantID     *int        `json:"p2_participant_id,omitempty" db:"p2_participant_id"`
	Score               *string     `json:"score,omitempty" db:"score"`
	MatchTime           time.Time   `json:"match_time" db:"match_time"`
	Status              MatchStatus `json:"status" db:"status"`
	WinnerParticipantID *int        `json:"winner_participant_id,omitempty" db:"winner_participant_id"`
	Round               *int        `json:"round,omitempty" db:"round"`
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`

	// Новые поля
	BracketMatchUID *string `json:"bracket_match_uid,omitempty" db:"bracket_match_uid"`
	NextMatchDBID   *int    `json:"next_match_db_id,omitempty" db:"next_match_db_id"`
	WinnerToSlot    *int    `json:"winner_to_slot,omitempty" db:"winner_to_slot"`

	Tournament *Tournament  `json:"tournament,omitempty" db:"-"`
	P1         *Participant `json:"p1,omitempty" db:"-"`
	P2         *Participant `json:"p2,omitempty" db:"-"`
	Winner     *Participant `json:"winner,omitempty" db:"-"`
}

type TeamMatch struct {
	ID                  int         `json:"id" db:"id"`
	TournamentID        int         `json:"tournament_id" db:"tournament_id"`
	T1ParticipantID     *int        `json:"t1_participant_id,omitempty" db:"t1_participant_id"`
	T2ParticipantID     *int        `json:"t2_participant_id,omitempty" db:"t2_participant_id"`
	Score               *string     `json:"score,omitempty" db:"score"`
	MatchTime           time.Time   `json:"match_time" db:"match_time"`
	Status              MatchStatus `json:"status" db:"status"`
	WinnerParticipantID *int        `json:"winner_participant_id,omitempty" db:"winner_participant_id"`
	Round               *int        `json:"round,omitempty" db:"round"`
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`

	// Новые поля
	BracketMatchUID *string `json:"bracket_match_uid,omitempty" db:"bracket_match_uid"`
	NextMatchDBID   *int    `json:"next_match_db_id,omitempty" db:"next_match_db_id"`
	WinnerToSlot    *int    `json:"winner_to_slot,omitempty" db:"winner_to_slot"`

	Tournament *Tournament  `json:"tournament,omitempty" db:"-"`
	T1         *Participant `json:"t1,omitempty" db:"-"`
	T2         *Participant `json:"t2,omitempty" db:"-"`
	Winner     *Participant `json:"winner,omitempty" db:"-"`
}
