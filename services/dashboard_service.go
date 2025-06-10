package services

import (
	"context"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

type DashboardService interface {
	GetStats(ctx context.Context) (models.DashboardStats, error)
}

type dashboardService struct {
	userRepo       repositories.UserRepository
	tournamentRepo repositories.TournamentRepository
	soloMatchRepo  repositories.SoloMatchRepository
	teamMatchRepo  repositories.TeamMatchRepository
}

func NewDashboardService(
	userRepo repositories.UserRepository,
	tournamentRepo repositories.TournamentRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,

) DashboardService {
	return &dashboardService{
		userRepo:       userRepo,
		tournamentRepo: tournamentRepo,
		soloMatchRepo:  soloMatchRepo,
		teamMatchRepo:  teamMatchRepo,
	}
}

func (s *dashboardService) GetStats(ctx context.Context) (models.DashboardStats, error) {
	usersTotal, _ := s.userRepo.Count(ctx, nil)
	bannedUsers, _ := s.userRepo.Count(ctx, map[string]interface{}{"status": "banned"})
	tournamentsTotal, _ := s.tournamentRepo.CountTournaments(ctx, nil)
	activeTournaments, _ := s.tournamentRepo.CountTournaments(ctx, map[string]interface{}{"status": "active"})
	SoloMatchesTotal, _ := s.soloMatchRepo.CountSoloMatches(ctx, nil)
	TeamMatchesTotal, _ := s.teamMatchRepo.CountTeamMatches(ctx, nil)

	return models.DashboardStats{
		UsersTotal:        usersTotal,
		BannedUsers:       bannedUsers,
		TournamentsTotal:  tournamentsTotal,
		ActiveTournaments: activeTournaments,
		SoloMatchesTotal:  SoloMatchesTotal,
		TeamMatchesTotal:  TeamMatchesTotal,
	}, nil
}
