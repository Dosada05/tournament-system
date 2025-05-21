// tournament-system/brackets/single_elimination.go
package brackets

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dosada05/tournament-system/models"
	"math"
	"sort"
)

type BracketMatch struct {
	UID          string
	Round        int
	OrderInRound int

	Participant1ID *int
	Participant2ID *int

	SourceMatch1UID *string
	SourceMatch2UID *string

	IsPlaceholder bool

	IsBye            bool
	ByeParticipantID *int
}

type node struct {
	participantID    *int
	sourceMatchUID   *string
	isByePlaceholder bool
}

type SingleEliminationGenerator struct {
}

func NewSingleEliminationGenerator() BracketGenerator {
	return &SingleEliminationGenerator{}
}

func (g *SingleEliminationGenerator) GetName() string {
	return "SingleElimination"
}

func (g *SingleEliminationGenerator) GenerateBracket(ctx context.Context, params GenerateBracketParams) ([]*BracketMatch, error) {
	participants := params.Participants
	n := len(participants)

	if n == 0 {
		return nil, errors.New("cannot generate bracket with zero participants")
	}
	if n < 2 {
		return nil, errors.New("not enough participants to generate a single elimination bracket (minimum 2)")
	}

	shuffledParticipants := make([]*models.Participant, n)
	copy(shuffledParticipants, participants)

	numRounds := 0
	if n > 0 {
		numRounds = int(math.Ceil(math.Log2(float64(n))))
	}

	sizeOfFullBracket := 1
	if numRounds > 0 {
		sizeOfFullBracket = 1 << uint(numRounds)
	}

	numByes := sizeOfFullBracket - n

	fmt.Printf("Generating Full Bracket: Participants=%d, CalculatedRounds=%d, BracketSizeForPairing=%d, Byes=%d\n",
		n, numRounds, sizeOfFullBracket, numByes)

	allGeneratedMatches := make([]*BracketMatch, 0, sizeOfFullBracket-1)

	currentRoundNodes := make([]*node, sizeOfFullBracket)
	participantIdx := 0

	if numByes > 0 {
		for i := 0; i < n; i++ {
			pid := shuffledParticipants[participantIdx].ID
			currentRoundNodes[i] = &node{participantID: &pid}
			participantIdx++
		}
		for i := n; i < sizeOfFullBracket; i++ {
			currentRoundNodes[i] = &node{isByePlaceholder: true}
		}
	} else {
		for i := 0; i < n; i++ {
			pid := shuffledParticipants[participantIdx].ID
			currentRoundNodes[i] = &node{participantID: &pid}
			participantIdx++
		}
	}

	matchUIDCounter := 0

	for r := 1; r <= numRounds; r++ {
		nextRoundNodes := make([]*node, 0, len(currentRoundNodes)/2)
		matchesInThisRound := 0

		for i := 0; i < len(currentRoundNodes); i += 2 {
			node1 := currentRoundNodes[i]
			node2 := currentRoundNodes[i+1]

			matchUIDCounter++
			currentMatchUID := fmt.Sprintf("R%dM%d", r, matchesInThisRound+1)

			bm := &BracketMatch{
				UID:           currentMatchUID,
				Round:         r,
				OrderInRound:  matchesInThisRound + 1,
				IsPlaceholder: false,
			}

			if node1.participantID != nil {
				bm.Participant1ID = node1.participantID
			} else if node1.sourceMatchUID != nil {
				bm.SourceMatch1UID = node1.sourceMatchUID
				bm.IsPlaceholder = true
			}

			if node2.participantID != nil {
				bm.Participant2ID = node2.participantID
			} else if node2.sourceMatchUID != nil {
				bm.SourceMatch2UID = node2.sourceMatchUID
				bm.IsPlaceholder = true
			}

			if node1.participantID != nil && node2.isByePlaceholder {
				bm.IsBye = true
				bm.ByeParticipantID = node1.participantID
				bm.Participant1ID = node1.participantID
				bm.Participant2ID = nil
				bm.IsPlaceholder = false
				nextRoundNodes = append(nextRoundNodes, &node{participantID: node1.participantID})

			} else if node2.participantID != nil && node1.isByePlaceholder {
				bm.IsBye = true
				bm.ByeParticipantID = node2.participantID
				bm.Participant1ID = node2.participantID
				bm.Participant2ID = nil
				bm.IsPlaceholder = false
				nextRoundNodes = append(nextRoundNodes, &node{participantID: node2.participantID})

			} else if node1.participantID != nil && node2.participantID != nil {
				bm.IsPlaceholder = false
				nextRoundNodes = append(nextRoundNodes, &node{sourceMatchUID: &currentMatchUID})
			} else if (node1.sourceMatchUID != nil || node2.sourceMatchUID != nil) || (node1.isByePlaceholder && node2.isByePlaceholder) {
				if node1.isByePlaceholder && node2.isByePlaceholder {
					fmt.Printf("Warning: Two bye placeholders met for match %s. Skipping node for next round.\n", currentMatchUID)
					matchUIDCounter--
					continue
				}
				nextRoundNodes = append(nextRoundNodes, &node{sourceMatchUID: &currentMatchUID})
			} else {
				return nil, fmt.Errorf("unexpected node combination for round %d, match %d: node1=%+v, node2=%+v", r, matchesInThisRound+1, node1, node2)
			}

			allGeneratedMatches = append(allGeneratedMatches, bm)
			matchesInThisRound++
		}
		currentRoundNodes = nextRoundNodes

		if len(currentRoundNodes) == 1 && r == numRounds {
			fmt.Printf("Reached final round structure. Final match UID (placeholder for winner): %s\n", *currentRoundNodes[0].sourceMatchUID)
		} else if len(currentRoundNodes) == 0 && r < numRounds {
			return nil, fmt.Errorf("internal error: no nodes left for round %d, but expected %d total rounds", r+1, numRounds)
		}
	}

	sort.Slice(allGeneratedMatches, func(i, j int) bool {
		if allGeneratedMatches[i].Round != allGeneratedMatches[j].Round {
			return allGeneratedMatches[i].Round < allGeneratedMatches[j].Round
		}
		return allGeneratedMatches[i].OrderInRound < allGeneratedMatches[j].OrderInRound
	})

	return allGeneratedMatches, nil
}
