package models

import "encoding/json"

type FormatParticipantType string

const (
	FormatParticipantSolo FormatParticipantType = "solo"
	FormatParticipantTeam FormatParticipantType = "team"
)

type RoundRobinSettings struct {
	NumberOfRounds int `json:"number_of_rounds"`
}

type Format struct {
	ID              int                   `json:"id" db:"id"`
	Name            string                `json:"name" db:"name"`
	BracketType     string                `json:"bracket_type" db:"bracket_type"`
	ParticipantType FormatParticipantType `json:"participant_type" db:"participant_type"`
	SettingsJSON    *string               `json:"-" db:"settings_json"`

	ParsedRoundRobinSettings *RoundRobinSettings `json:"parsed_round_robin_settings,omitempty" db:"-"`
}

func (f *Format) GetRoundRobinSettings() (*RoundRobinSettings, error) {
	if f.BracketType != "RoundRobin" || f.SettingsJSON == nil || *f.SettingsJSON == "" {
		return nil, nil
	}
	var settings RoundRobinSettings
	err := json.Unmarshal([]byte(*f.SettingsJSON), &settings)
	if err != nil {
		return nil, err
	}
	if settings.NumberOfRounds < 1 || settings.NumberOfRounds > 2 {
		settings.NumberOfRounds = 1
	}
	return &settings, nil
}
