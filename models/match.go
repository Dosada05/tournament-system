package models

import "time"

// MatchStatus представляет статусы матча, соответствующие ENUM в БД.
type MatchStatus string

const (
	StatusScheduled      MatchStatus = "scheduled"
	StatusInProgress     MatchStatus = "in_progress"
	MatchStatusCompleted MatchStatus = "completed"
	MatchStatusCanceled  MatchStatus = "canceled"
)

type SoloMatch struct {
	ID                  int         `json:"id" db:"id"`
	TournamentID        int         `json:"tournament_id" db:"tournament_id"`
	P1ParticipantID     int         `json:"p1_participant_id" db:"p1_participant_id"`                   // Ссылка на participants.id
	P2ParticipantID     int         `json:"p2_participant_id" db:"p2_participant_id"`                   // Ссылка на participants.id
	Score               *string     `json:"score,omitempty" db:"score"`                                 // Указатель для NULL VARCHAR
	MatchTime           time.Time   `json:"match_time" db:"match_time"`                                 // Соответствует TIMESTAMPTZ, имя изменено
	Status              MatchStatus `json:"status" db:"status"`                                         // Используем кастомный тип
	WinnerParticipantID *int        `json:"winner_participant_id,omitempty" db:"winner_participant_id"` // Указатель для NULL, ссылка на participants.id
	Round               *int        `json:"round,omitempty" db:"round"`                                 // Добавлено поле Round (nullable INT)
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`                                 // Добавлено поле CreatedAt (TIMESTAMPTZ)

	// Опциональные связанные сущности
	Tournament *Tournament  `json:"tournament,omitempty" db:"-"`
	P1         *Participant `json:"p1,omitempty" db:"-"`
	P2         *Participant `json:"p2,omitempty" db:"-"`
	Winner     *Participant `json:"winner,omitempty" db:"-"`
}

type TeamMatch struct {
	ID                  int         `json:"id" db:"id"`
	TournamentID        int         `json:"tournament_id" db:"tournament_id"`
	T1ParticipantID     int         `json:"t1_participant_id" db:"t1_participant_id"`                   // Ссылка на participants.id
	T2ParticipantID     int         `json:"t2_participant_id" db:"t2_participant_id"`                   // Ссылка на participants.id
	Score               *string     `json:"score,omitempty" db:"score"`                                 // Указатель для NULL VARCHAR
	MatchTime           time.Time   `json:"match_time" db:"match_time"`                                 // Соответствует TIMESTAMPTZ, имя изменено
	Status              MatchStatus `json:"status" db:"status"`                                         // Используем кастомный тип
	WinnerParticipantID *int        `json:"winner_participant_id,omitempty" db:"winner_participant_id"` // Указатель для NULL, ссылка на participants.id
	Round               *int        `json:"round,omitempty" db:"round"`                                 // Добавлено поле Round (nullable INT)
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`                                 // Добавлено поле CreatedAt (TIMESTAMPTZ)

	// Опциональные связанные сущности
	Tournament *Tournament  `json:"tournament,omitempty" db:"-"`
	T1         *Participant `json:"t1,omitempty" db:"-"`
	T2         *Participant `json:"t2,omitempty" db:"-"`
	Winner     *Participant `json:"winner,omitempty" db:"-"`
}
