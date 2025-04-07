package models

import "time"

type Match struct {
	ID           int       `json:"id"`
	TournamentID int       `json:"tournament_id"`
	Player1ID    int       `json:"p1_id"`
	Player2ID    int       `json:"p2_id"`
	Score        string    `json:"score"`
	Date         time.Time `json:"date"`
	WinnerID     int       `json:"winner_id"`
}
