package brackets

import (
	"context"
	"github.com/Dosada05/tournament-system/models"
)

type GenerateBracketParams struct {
	Tournament   *models.Tournament
	Participants []*models.Participant
}

type BracketGenerator interface {
	GenerateBracket(ctx context.Context, params GenerateBracketParams) ([]*BracketMatch, error)

	GetName() string
}
