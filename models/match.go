package models

import "time"

type MatchStatus string

const (
	StatusScheduled      MatchStatus = "scheduled"
	StatusInProgress     MatchStatus = "in_progress"
	MatchStatusCompleted MatchStatus = "completed"
	MatchStatusCanceled  MatchStatus = "canceled"
)

type SoloMatch struct {
	ID           int         `json:"id"`
	TournamentID int         `json:"tournament_id"`
	P1ID         int         `json:"p1_id"`
	P2ID         int         `json:"p2_id"`
	Score        *string     `json:"score,omitempty"`
	Date         time.Time   `json:"date"`
	Status       MatchStatus `json:"status"`
	WinnerID     *int        `json:"winner_id,omitempty"`
}

type TeamMatch struct {
	ID           int         `json:"id"`
	TournamentID int         `json:"tournament_id"`
	T1ID         int         `json:"t1_id"`
	T2ID         int         `json:"t2_id"`
	Score        *string     `json:"score,omitempty"`
	Date         time.Time   `json:"date"`
	Status       MatchStatus `json:"status"`
	WinnerID     *int        `json:"winner_id,omitempty"`
}
