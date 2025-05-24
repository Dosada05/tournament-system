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
	matchOrder := 0

	// Generate pairings
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			p1ID := participants[i].ID
			p2ID := participants[j].ID

			// First leg
			matchOrder++
			matches = append(matches, &BracketMatch{
				UID:            fmt.Sprintf("T%d_RRM%d_L1_P%dvsP%d", tournament.ID, matchOrder, p1ID, p2ID),
				Round:          1, // Conceptual round or game day. For simplicity, using 1 for all.
				OrderInRound:   matchOrder,
				Participant1ID: &p1ID,
				Participant2ID: &p2ID,
			})

			if rrSettings.NumberOfRounds == 2 {
				// Second leg (participants swapped for home/away implication if needed, or just a second match)
				// UID needs to be unique for the second leg match
				matches = append(matches, &BracketMatch{
					UID:            fmt.Sprintf("T%d_RRM%d_L2_P%dvP%ds", tournament.ID, matchOrder, p2ID, p1ID), // Swapped for UID
					Round:          1,                                                                           // Still part of the overall "league phase"
					OrderInRound:   matchOrder + len(participants)*(len(participants)-1)/2,                      // Ensure unique order
					Participant1ID: &p2ID,                                                                       // For the match, P1 is P2 from the first leg
					Participant2ID: &p1ID,                                                                       // For the match, P2 is P1 from the first leg
				})
			}
		}
	}

	// Sort matches for consistent order if needed, e.g., by OrderInRound
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].OrderInRound < matches[j].OrderInRound
	})

	return matches, nil
}
