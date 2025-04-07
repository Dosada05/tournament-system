package models

import "time"

type User struct {
	ID           int       `json:"id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Nickname     string    `json:"nickname"`
	TeamID       int       `json:"team_id,omitempty"`
	Role         string    `json:"role"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
