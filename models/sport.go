package models

// Sport представляет вид спорта.
type Sport struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}
