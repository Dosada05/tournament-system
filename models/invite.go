package models

import "time"

type Invite struct {
	ID        int       `json:"id" db:"id"`
	TeamID    int       `json:"team_id" db:"team_id"`
	Token     string    `json:"-" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
