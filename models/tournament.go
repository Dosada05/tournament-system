package models

import "time"

type Tournament struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	SportType        string    `json:"sport_type"`
	Format           string    `json:"format"`
	OrganizerID      int       `json:"organizer_id"`
	RegistrationDate time.Time `json:"reg_date"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	Location         string    `json:"location"`
	Status           string    `json:"status"`
	MaxParticipants  int       `json:"max_participants"`
}
