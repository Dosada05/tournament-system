package models

import "time"

type User struct {
	ID           int       `json:"id"`
	FirstName    string    `json:"first_name" validate:"required"`
	LastName     string    `json:"last_name" validate:"required"`
	Nickname     *string   `json:"nickname,omitempty"`
	TeamID       *int      `json:"team_id,omitempty"`
	Role         string    `json:"role"`
	Email        string    `json:"email" validate:"required,email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
