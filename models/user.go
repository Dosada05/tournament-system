package models

import "time"

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleOrganizer UserRole = "organizer"
	RolePlayer    UserRole = "player"
)

type User struct {
	ID           int       `json:"id" db:"id"`
	FirstName    string    `json:"first_name" db:"first_name"`
	LastName     string    `json:"last_name" db:"last_name"`
	Nickname     *string   `json:"nickname,omitempty" db:"nickname"`
	TeamID       *int      `json:"team_id,omitempty" db:"team_id"`
	Role         UserRole  `json:"role" db:"role"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	LogoKey      *string   `json:"-" db:"logo_key"`
	LogoURL      *string   `json:"logo_url,omitempty" db:"-"`

	Team *Team `json:"team,omitempty" db:"-"`
}

// Credentials используется для передачи данных аутентификации (логин/пароль).
type Credentials struct {
	Email    string `json:"email" validate:"required,email"` // Валидация уместна в DTO
	Password string `json:"password" validate:"required"`    // Валидация уместна в DTO
}
