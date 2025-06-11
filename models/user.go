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

	EmailConfirmed         bool   `json:"email_confirmed" db:"email_confirmed"`
	EmailConfirmationToken string `json:"-" db:"email_confirmation_token"`

	PasswordResetToken     *string    `json:"-" db:"password_reset_token"`
	PasswordResetExpiresAt *time.Time `json:"-" db:"password_reset_expires_at"`

	Team *Team `json:"team,omitempty" db:"-"`
}

type Credentials struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UserFilter struct {
	Search string
	Role   *string
	Status *string
	Page   int
	Limit  int
}

type UserListResponse struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"totalCount"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
}
