package models

type FormatParticipantType string

const (
	FormatParticipantSolo FormatParticipantType = "solo"
	FormatParticipantTeam FormatParticipantType = "team"
)

type Format struct {
	ID              int                   `json:"id" db:"id"`
	Name            string                `json:"name" db:"name"`
	BracketType     string                `json:"bracket_type" db:"bracket_type"`
	ParticipantType FormatParticipantType `json:"participant_type" db:"participant_type"`
	SettingsJSON    *string               `json:"-" db:"settings_json"`
}
