package models

type Team struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CaptainID int    `json:"captain_id"`
}
