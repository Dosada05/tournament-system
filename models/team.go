package models

type Team struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	SportID   int    `json:"sport_id"`
	CaptainID int    `json:"captain_id"`
}
