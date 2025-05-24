package models

import "encoding/json"

type FormatParticipantType string

const (
	FormatParticipantSolo FormatParticipantType = "solo"
	FormatParticipantTeam FormatParticipantType = "team"
)

// RoundRobinSettings defines specific settings for a RoundRobin tournament format.
type RoundRobinSettings struct {
	NumberOfRounds int `json:"number_of_rounds"` // 1 for single round-robin, 2 for double
	// Potentially add other settings like points_for_win, points_for_draw, points_for_loss if they can vary
}

type Format struct {
	ID              int                   `json:"id" db:"id"`
	Name            string                `json:"name" db:"name"`
	BracketType     string                `json:"bracket_type" db:"bracket_type"` // e.g., "SingleElimination", "RoundRobin"
	ParticipantType FormatParticipantType `json:"participant_type" db:"participant_type"`
	SettingsJSON    *string               `json:"-" db:"settings_json"` // Raw JSON string from DB

	// Parsed settings, not stored in DB, populated by service if needed
	ParsedRoundRobinSettings *RoundRobinSettings `json:"parsed_round_robin_settings,omitempty" db:"-"`
}

// Helper to unmarshal settings if it's a RoundRobin format
func (f *Format) GetRoundRobinSettings() (*RoundRobinSettings, error) {
	if f.BracketType != "RoundRobin" || f.SettingsJSON == nil || *f.SettingsJSON == "" {
		return nil, nil // Not a RoundRobin format or no settings
	}
	var settings RoundRobinSettings
	err := json.Unmarshal([]byte(*f.SettingsJSON), &settings)
	if err != nil {
		return nil, err
	}
	// Default to 1 round if not specified or invalid
	if settings.NumberOfRounds < 1 || settings.NumberOfRounds > 2 { // Assuming max 2 rounds for now
		settings.NumberOfRounds = 1
	}
	return &settings, nil
}
