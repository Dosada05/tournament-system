package models

import "time"

type Team struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	SportID   int       `json:"sport_id" db:"sport_id"`
	CaptainID int       `json:"captain_id" db:"captain_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	Sport        *Sport        `json:"sport,omitempty" db:"-"`
	Captain      *User         `json:"captain,omitempty" db:"-"`
	Members      []User        `json:"members,omitempty" db:"-"`
	Participants []Participant `json:"participants,omitempty" db:"-"`

	LogoKey *string `json:"-" db:"logo_key"`
	LogoURL *string `json:"logo_url,omitempty" db:"-"`
}
