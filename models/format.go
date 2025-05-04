package models

// Format представляет формат проведения турнира.
type Format struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"` // UNIQUE в БД
}
