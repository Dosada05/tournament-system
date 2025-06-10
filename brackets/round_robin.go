package brackets

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Dosada05/tournament-system/models"
)

type RoundRobinGenerator struct{}

func NewRoundRobinGenerator() BracketGenerator {
	return &RoundRobinGenerator{}
}

func (g *RoundRobinGenerator) GetName() string {
	return "RoundRobin"
}

// GenerateBracket creates matches for a round-robin tournament.
// For a single round-robin, each participant plays every other participant once.
// For a double round-robin, they play each other twice.
func (g *RoundRobinGenerator) GenerateBracket(ctx context.Context, params GenerateBracketParams) ([]*BracketMatch, error) {
	participants := params.Participants
	tournament := params.Tournament

	if len(participants) < 2 {
		return nil, fmt.Errorf("RoundRobinGenerator: not enough participants (found %d, min 2 required)", len(participants))
	}

	rrSettings := &models.RoundRobinSettings{NumberOfRounds: 1} // Default to 1 round
	if tournament.Format != nil && tournament.Format.SettingsJSON != nil && *tournament.Format.SettingsJSON != "" {
		var settings models.RoundRobinSettings
		if err := json.Unmarshal([]byte(*tournament.Format.SettingsJSON), &settings); err == nil {
			if settings.NumberOfRounds == 1 || settings.NumberOfRounds == 2 {
				rrSettings.NumberOfRounds = settings.NumberOfRounds
			}
		} else {
			// Log or handle error if settings are invalid, but proceed with default
			fmt.Printf("Warning: Could not parse RoundRobin settings for tournament %d: %v. Defaulting to 1 round.\n", tournament.ID, err)
		}
	}

	matches := make([]*BracketMatch, 0)

	pairIndex := 0 // Порядковый номер пары

	// Generate pairings
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			pairIndex++
			p1ID := participants[i].ID
			p2ID := participants[j].ID

			// Первый круг (Leg 1)
			matches = append(matches, &BracketMatch{
				UID:            fmt.Sprintf("T%d_RRP%d_L1_%dvs%d", tournament.ID, pairIndex, p1ID, p2ID), // RRP для RoundRobin Pair
				Round:          1,                                                                        // Круг 1
				OrderInRound:   pairIndex,                                                                // Порядок матча в этом круге (основан на индексе пары)
				Participant1ID: &p1ID,
				Participant2ID: &p2ID,
			})

			if rrSettings.NumberOfRounds == 2 {
				// Второй круг (Leg 2)
				matches = append(matches, &BracketMatch{
					UID:            fmt.Sprintf("T%d_RRP%d_L2_%dvs%d", tournament.ID, pairIndex, p2ID, p1ID), // Участники меняются местами в UID для наглядности
					Round:          2,                                                                        // Круг 2
					OrderInRound:   pairIndex,                                                                // Порядок матча в этом круге (соответствует порядку в первом круге для той же пары)
					Participant1ID: &p2ID,                                                                    // Участники меняются местами для самого матча
					Participant2ID: &p1ID,
				})
			}
		}
	}

	// Сортировка матчей: сначала по номеру круга, затем по порядку в круге
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Round != matches[j].Round {
			return matches[i].Round < matches[j].Round
		}
		return matches[i].OrderInRound < matches[i].OrderInRound // Исправлено на matches[j].OrderInRound
	})

	return matches, nil
}
