package services

import "errors"

// Общие ошибки, используемые в разных сервисах и маппинге HTTP.
var (
	// Ресурс не найден (универсальная)
	ErrNotFound = errors.New("requested resource not found")

	// Ошибки валидации и бизнес-правил
	ErrValidationFailed       = errors.New("validation failed") // Общая ошибка валидации
	ErrPasswordTooShort       = errors.New("password is too short")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrTeamNameRequired       = errors.New("team name is required")
	ErrUserAlreadyInTeam      = errors.New("user is already in a team")
	ErrCannotRemoveCaptain    = errors.New("cannot remove the team captain")
	ErrUserCannotRegisterSolo = errors.New("user cannot register solo (already in a team)")
	ErrInviteExpired          = errors.New("invite has expired")
	ErrRegistrationNotOpen    = errors.New("tournament registration is not open")
	ErrTournamentFull         = errors.New("tournament registration is full")

	// Ошибки конфликтов
	ErrUserEmailConflict      = errors.New("email address is already in use")
	ErrUserNicknameConflict   = errors.New("nickname is already in use")
	ErrTeamNameConflict       = errors.New("team name is already in use")
	ErrRegistrationConflict   = errors.New("user or team is already registered for this tournament")
	ErrTournamentNameConflict = errors.New("tournament name already exists") // Добавлено для полноты

	// Ошибки аутентификации и авторизации
	ErrAuthenticationFailed   = errors.New("authentication failed") // Общая ошибка аутентификации
	ErrForbiddenOperation     = errors.New("operation not allowed for the current user")
	ErrCaptainActionForbidden = errors.New("only the team captain can perform this action")
	ErrSelfLeaveForbidden     = errors.New("only the team captain or the member themselves can perform this action")
	ErrUserMustBeCaptain      = errors.New("only the team captain can register the team")

	// Ошибки, специфичные для сущностей (могут дублировать ErrNotFound, но дают больше контекста)
	ErrUserNotFound        = errors.New("user not found")
	ErrTeamNotFound        = errors.New("team not found")
	ErrSportNotFound       = errors.New("sport not found")
	ErrFormatNotFound      = errors.New("format not found")
	ErrTournamentNotFound  = errors.New("tournament not found")
	ErrParticipantNotFound = errors.New("participant registration not found")
	ErrInviteNotFound      = errors.New("invite not found")

	// Ошибки турниров (примеры)
	ErrTournamentInvalidRegDate          = errors.New("tournament registration end date must be after start date")
	ErrTournamentInvalidDateRange        = errors.New("tournament end date must be after start date")
	ErrTournamentInvalidCapacity         = errors.New("tournament max participants must be positive")
	ErrTournamentInvalidStatus           = errors.New("invalid tournament status provided")
	ErrTournamentInvalidStatusTransition = errors.New("invalid tournament status transition")
)
