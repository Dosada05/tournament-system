package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

// ErrMatchesListFailed - общая ошибка для листинга матчей
var ErrMatchesListFailed = errors.New("failed to list matches")

type MatchService interface {
	ListSoloMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.SoloMatch, error)
	ListTeamMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.TeamMatch, error)
	// В будущем здесь могут быть методы для создания/обновления матчей
}

type matchService struct {
	soloMatchRepo repositories.SoloMatchRepository
	teamMatchRepo repositories.TeamMatchRepository
	// participantRepo repositories.ParticipantRepository // Может понадобиться для обогащения данных матчей
}

func NewMatchService(
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	// participantRepo repositories.ParticipantRepository,
) MatchService {
	return &matchService{
		soloMatchRepo: soloMatchRepo,
		teamMatchRepo: teamMatchRepo,
		// participantRepo: participantRepo,
	}
}

func (s *matchService) ListSoloMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.SoloMatch, error) {
	// null для round и status означает получение всех матчей без фильтрации по этим полям
	matches, err := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: solo matches for tournament %d: %w", ErrMatchesListFailed, tournamentID, err)
	}
	if matches == nil {
		return []*models.SoloMatch{}, nil
	}
	// Здесь можно добавить логику обогащения матчей данными участников, если это не делает репозиторий
	// Например, загрузить P1, P2, Winner из participantRepo по их ID
	return matches, nil
}

func (s *matchService) ListTeamMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.TeamMatch, error) {
	matches, err := s.teamMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: team matches for tournament %d: %w", ErrMatchesListFailed, tournamentID, err)
	}
	if matches == nil {
		return []*models.TeamMatch{}, nil
	}
	// Аналогично, обогащение данными участников (команд)
	return matches, nil
}
