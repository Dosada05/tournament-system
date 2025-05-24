package services

import (
	"context"
	"fmt"
	"log/slog" // Switched to slog
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"golang.org/x/sync/errgroup"
)

// ErrTournamentRequiresTransaction был определен ранее, можно оставить или удалить, если не используется вне этого контекста
// var ErrTournamentRequiresTransaction = fmt.Errorf("database transaction is required for this operation")

type TournamentWinnerPayload struct {
	TournamentID   int                 `json:"tournament_id"`
	Winner         *models.Participant `json:"winner"`
	Message        string              `json:"message"`
	IsAutoWin      bool                `json:"is_auto_win"`
	ParticipantIds []int               `json:"participant_ids"`
}

type BracketService interface {
	GenerateAndSaveBracket(ctx context.Context, exec repositories.SQLExecutor, tournament *models.Tournament) (interface{}, error)
	GetFullTournamentData(ctx context.Context, tournamentID int, formatID int) (*models.Tournament, error) // Kept for now
}

type bracketService struct {
	formatRepo      repositories.FormatRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
	standingRepo    repositories.TournamentStandingRepository // Added
	logger          *slog.Logger                              // Added
}

func NewBracketService(
	formatRepo repositories.FormatRepository,
	participantRepo repositories.ParticipantRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	standingRepo repositories.TournamentStandingRepository, // Added
	logger *slog.Logger, // Added
) BracketService {
	return &bracketService{
		formatRepo:      formatRepo,
		participantRepo: participantRepo,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
		standingRepo:    standingRepo, // Added
		logger:          logger,       // Added
	}
}

func (s *bracketService) GenerateAndSaveBracket(ctx context.Context, exec repositories.SQLExecutor, tournament *models.Tournament) (interface{}, error) {
	if tournament.Format == nil {
		format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if err != nil {
			s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to load format", slog.Int("format_id", tournament.FormatID), slog.Any("error", err))
			return nil, fmt.Errorf("GenerateAndSaveBracket: failed to load format %d: %w", tournament.FormatID, err)
		}
		if format == nil {
			s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: format not found", slog.Int("format_id", tournament.FormatID))
			return nil, fmt.Errorf("GenerateAndSaveBracket: format %d not found", tournament.FormatID)
		}
		tournament.Format = format
	}

	s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Starting", slog.Int("tournament_id", tournament.ID), slog.String("format_type", tournament.Format.BracketType))

	statusConfirmed := models.StatusParticipant
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournament.ID, &statusConfirmed, true) // includeNested true to get user/team details if needed by generator
	if err != nil {
		s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to list participants", slog.Int("tournament_id", tournament.ID), slog.Any("error", err))
		return nil, fmt.Errorf("GenerateAndSaveBracket: failed to list participants for tournament %d: %w", tournament.ID, err)
	}

	if len(dbParticipants) < 2 {
		s.logger.WarnContext(ctx, "GenerateAndSaveBracket: not enough participants", slog.Int("tournament_id", tournament.ID), slog.Int("participant_count", len(dbParticipants)))
		return nil, fmt.Errorf("GenerateAndSaveBracket: not enough participants (found %d, min 2)", len(dbParticipants))
	}

	var bracketGenerator brackets.BracketGenerator
	switch tournament.Format.BracketType {
	case "SingleElimination":
		bracketGenerator = brackets.NewSingleEliminationGenerator()
	case "RoundRobin":
		bracketGenerator = brackets.NewRoundRobinGenerator()
	default:
		s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: unsupported bracket type", slog.String("bracket_type", tournament.Format.BracketType))
		return nil, fmt.Errorf("unsupported bracket type '%s'", tournament.Format.BracketType)
	}
	s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Using generator", slog.String("generator_name", bracketGenerator.GetName()))

	params := brackets.GenerateBracketParams{
		Tournament:   tournament,
		Participants: dbParticipants,
	}
	generatedBracketMatches, genErr := bracketGenerator.GenerateBracket(ctx, params)
	if genErr != nil {
		s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to generate bracket structure", slog.Any("error", genErr))
		return nil, fmt.Errorf("GenerateAndSaveBracket: failed to generate structure: %w", genErr)
	}
	if len(generatedBracketMatches) == 0 && len(dbParticipants) >= 2 && tournament.Format.BracketType != "RoundRobin" { // RoundRobin might have 0 matches if only 1 participant (though we check for <2)
		s.logger.WarnContext(ctx, "GenerateAndSaveBracket: no matches generated", slog.Int("participant_count", len(dbParticipants)))
		// For RoundRobin with 2 participants, 1 match is expected. If 0, it's an issue.
		if tournament.Format.BracketType == "RoundRobin" && len(dbParticipants) >= 2 {
			return nil, fmt.Errorf("GenerateAndSaveBracket: no matches generated for RoundRobin with %d participants", len(dbParticipants))
		}
		// For SingleElimination, this is definitely an issue.
		if tournament.Format.BracketType == "SingleElimination" {
			return nil, fmt.Errorf("GenerateAndSaveBracket: no matches generated for SingleElimination with %d participants", len(dbParticipants))
		}
	}
	s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Generated bracket matches", slog.Int("count", len(generatedBracketMatches)))

	mapBracketUIDToDBMatchID := make(map[string]int)
	mapBracketUIDToModel := make(map[string]*brackets.BracketMatch)
	createdDBMatchEntities := make([]interface{}, 0, len(generatedBracketMatches))

	defaultMatchTime := tournament.StartDate
	if time.Now().After(defaultMatchTime) {
		defaultMatchTime = time.Now().Add(15 * time.Minute)
	}
	// Ensure defaultMatchTime is not before tournament start_date, adjust if it is.
	if defaultMatchTime.Before(tournament.StartDate) {
		defaultMatchTime = tournament.StartDate.Add(15 * time.Minute) // Schedule slightly after official start
	}

	// ПЕРВЫЙ ПРОХОД: Создаем все матчи-заготовки в БД, используя переданный 'exec'
	for _, bm := range generatedBracketMatches {
		mapBracketUIDToModel[bm.UID] = bm

		if bm.IsBye && tournament.Format.BracketType == "SingleElimination" { // Byes primarily for SE
			if bm.ByeParticipantID != nil {
				// Corrected slog call for *int
				s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Participant has a bye in Single Elimination.", slog.Any("participant_id", bm.ByeParticipantID), slog.String("uid", bm.UID), slog.Int("round", bm.Round))
			}
			continue
		}

		roundNum := bm.Round
		var currentDBMatchID int
		var currentDBMatchEntity interface{}
		bmUIDStr := bm.UID

		switch tournament.Format.ParticipantType {
		case models.FormatParticipantSolo:
			newMatch := &models.SoloMatch{
				TournamentID:    tournament.ID,
				P1ParticipantID: bm.Participant1ID,
				P2ParticipantID: bm.Participant2ID,
				MatchTime:       defaultMatchTime,
				Status:          models.StatusScheduled,
				Round:           &roundNum,
				BracketMatchUID: &bmUIDStr,
			}
			err = s.soloMatchRepo.Create(ctx, exec, newMatch)
			if err != nil {
				s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to save solo match", slog.String("bracket_uid", bm.UID), slog.Any("error", err))
				return nil, fmt.Errorf("GenerateAndSaveBracket: failed to save solo match (BracketUID: %s): %w", bm.UID, err)
			}
			currentDBMatchID = newMatch.ID
			currentDBMatchEntity = newMatch
		case models.FormatParticipantTeam:
			newMatch := &models.TeamMatch{
				TournamentID:    tournament.ID,
				T1ParticipantID: bm.Participant1ID,
				T2ParticipantID: bm.Participant2ID,
				MatchTime:       defaultMatchTime,
				Status:          models.StatusScheduled,
				Round:           &roundNum,
				BracketMatchUID: &bmUIDStr,
			}
			err = s.teamMatchRepo.Create(ctx, exec, newMatch)
			if err != nil {
				s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to save team match", slog.String("bracket_uid", bm.UID), slog.Any("error", err))
				return nil, fmt.Errorf("GenerateAndSaveBracket: failed to save team match (BracketUID: %s): %w", bm.UID, err)
			}
			currentDBMatchID = newMatch.ID
			currentDBMatchEntity = newMatch
		default:
			s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: unknown participant type", slog.String("participant_type", string(tournament.Format.ParticipantType)))
			return nil, fmt.Errorf("unknown participant type '%s' for tournament %d", tournament.Format.ParticipantType, tournament.ID)
		}
		mapBracketUIDToDBMatchID[bm.UID] = currentDBMatchID
		createdDBMatchEntities = append(createdDBMatchEntities, currentDBMatchEntity)
		s.logger.InfoContext(ctx, "GenerateAndSaveBracket: DB Match created", slog.Int("db_match_id", currentDBMatchID), slog.String("bracket_uid", bm.UID), slog.Int("round", roundNum))
	}

	// SECOND PASS (Only for SingleElimination): Set up next_match_db_id and winner_to_slot
	if tournament.Format.BracketType == "SingleElimination" {
		for currentBracketUID, currentDBMatchID := range mapBracketUIDToDBMatchID {
			bm := mapBracketUIDToModel[currentBracketUID] // The source match from the generator

			var nextMatchDBIDForUpdate *int
			var targetSlotInNextMatchForUpdate *int

			// Find which generated match (bmTarget) has currentBracketUID as one of its sources
			for _, bmTarget := range generatedBracketMatches {
				if bmTarget.IsBye { // Skip byes as target matches
					continue
				}
				targetDBID, isTargetMatchInDB := mapBracketUIDToDBMatchID[bmTarget.UID]
				if !isTargetMatchInDB { // Target not a real match
					continue
				}

				slot := 0
				if bmTarget.SourceMatch1UID != nil && *bmTarget.SourceMatch1UID == bm.UID {
					slot = 1
				} else if bmTarget.SourceMatch2UID != nil && *bmTarget.SourceMatch2UID == bm.UID {
					slot = 2
				}

				if slot > 0 { // currentBracketUID is a source for bmTarget
					nextMatchDBIDForUpdate = &targetDBID
					targetSlotInNextMatchForUpdate = &slot
					break // Found the next match
				}
			}

			if nextMatchDBIDForUpdate != nil && targetSlotInNextMatchForUpdate != nil {
				s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Linking SE DB Match",
					slog.Int("source_match_id", currentDBMatchID), slog.String("source_bracket_uid", bm.UID),
					slog.Any("next_match_db_id", nextMatchDBIDForUpdate), slog.Any("winner_to_slot", targetSlotInNextMatchForUpdate))

				if tournament.Format.ParticipantType == models.FormatParticipantSolo {
					err = s.soloMatchRepo.UpdateNextMatchInfo(ctx, exec, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
				} else {
					err = s.teamMatchRepo.UpdateNextMatchInfo(ctx, exec, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
				}
				if err != nil {
					s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to update next match info", slog.Int("db_match_id", currentDBMatchID), slog.Any("error", err))
					return nil, fmt.Errorf("GenerateAndSaveBracket: failed to update next match info for DB match %d (BracketUID: %s): %w", currentDBMatchID, bm.UID, err)
				}
			}
		}
	}

	// Initialize standings for RoundRobin
	if tournament.Format.BracketType == "RoundRobin" {
		s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Initializing standings for RoundRobin tournament", slog.Int("tournament_id", tournament.ID))
		standingsToCreate := make([]*models.TournamentStanding, 0, len(dbParticipants))
		for _, p := range dbParticipants {
			standingsToCreate = append(standingsToCreate, &models.TournamentStanding{
				TournamentID:  tournament.ID,
				ParticipantID: p.ID, // Participant's DB ID
				Points:        0,
				GamesPlayed:   0,
				Wins:          0,
				Draws:         0,
				Losses:        0,
				ScoreFor:      0,
				ScoreAgainst:  0,
				UpdatedAt:     time.Now(),
			})
		}
		if err := s.standingRepo.BatchCreate(ctx, exec, standingsToCreate); err != nil {
			s.logger.ErrorContext(ctx, "GenerateAndSaveBracket: failed to batch create standings", slog.Int("tournament_id", tournament.ID), slog.Any("error", err))
			return nil, fmt.Errorf("GenerateAndSaveBracket: failed to initialize standings for tournament %d: %w", tournament.ID, err)
		}
		s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Standings initialized", slog.Int("tournament_id", tournament.ID), slog.Int("standings_count", len(standingsToCreate)))
	}

	s.logger.InfoContext(ctx, "GenerateAndSaveBracket: Bracket processing completed successfully.", slog.Int("tournament_id", tournament.ID))
	return createdDBMatchEntities, nil
}

// GetFullTournamentData остается без изменений, он не использует транзакции этого сервиса
// и предназначен для чтения полного состояния сетки для отображения.
func (s *bracketService) GetFullTournamentData(ctx context.Context, tournamentID int, formatID int) (*models.Tournament, error) {
	tournament := &models.Tournament{ID: tournamentID, FormatID: formatID}
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		format, err := s.formatRepo.GetByID(gCtx, formatID)
		if err != nil {
			s.logger.ErrorContext(gCtx, "Error fetching format in GetFullTournamentData", slog.Int("format_id", formatID), slog.Any("error", err))
			return fmt.Errorf("failed to fetch tournament format %d: %w", formatID, err)
		}
		if format == nil {
			s.logger.WarnContext(gCtx, "Format not found in GetFullTournamentData", slog.Int("format_id", formatID))
			return fmt.Errorf("tournament format %d not found", formatID)
		}
		tournament.Format = format
		return nil
	})

	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		participants, err := s.participantRepo.ListByTournament(gCtx, tournamentID, &confirmedStatus, true) // includeNested=true
		if err != nil {
			s.logger.WarnContext(gCtx, "Error fetching confirmed participants in GetFullTournamentData", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
			// Non-critical, proceed with empty slice
		}
		if participants == nil {
			tournament.Participants = []models.Participant{}
		} else {
			tournament.Participants = make([]models.Participant, len(participants))
			for i, p := range participants {
				if p != nil {
					tournament.Participants[i] = *p
				}
			}
		}
		return nil
	})

	g.Go(func() error {
		// Wait for format to be loaded to determine participant type
		// This is a simplification; proper synchronization or sequential loading might be better.
		// Or, the calling service (TournamentService) should ensure format is loaded.
		// For now, we'll proceed hoping format is loaded by the time this goroutine runs effectively.
		// If format loading fails, these might try to fetch wrong match types or fail.

		// It's safer to check if tournament.Format is populated after the errgroup.Wait()
		// and then fetch matches. Or, make match fetching dependent on format being loaded.
		// Let's assume for now that TournamentService's GetTournamentByID loads the format.
		// If called directly, GetFullTournamentData needs to handle format loading carefully.

		// This function is usually called by TournamentService.GetTournamentByID or GetTournamentBracketData,
		// which should have already loaded the format into the tournament object.
		// If tournament.Format is not nil here, we use its type.

		if tournament.Format != nil { // Check if format is available
			if tournament.Format.ParticipantType == models.FormatParticipantSolo {
				soloMatches, err := s.soloMatchRepo.ListByTournament(gCtx, tournamentID, nil, nil)
				if err != nil {
					s.logger.WarnContext(gCtx, "Error fetching solo matches in GetFullTournamentData", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
				}
				if soloMatches == nil {
					tournament.SoloMatches = []models.SoloMatch{}
				} else {
					tournament.SoloMatches = make([]models.SoloMatch, len(soloMatches))
					for i, m := range soloMatches {
						if m != nil {
							tournament.SoloMatches[i] = *m
						}
					}
				}
			}
		}
		return nil
	})

	g.Go(func() error {
		if tournament.Format != nil { // Check if format is available
			if tournament.Format.ParticipantType == models.FormatParticipantTeam {
				teamMatches, err := s.teamMatchRepo.ListByTournament(gCtx, tournamentID, nil, nil)
				if err != nil {
					s.logger.WarnContext(gCtx, "Error fetching team matches in GetFullTournamentData", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
				}
				if teamMatches == nil {
					tournament.TeamMatches = []models.TeamMatch{}
				} else {
					tournament.TeamMatches = make([]models.TeamMatch, len(teamMatches))
					for i, m := range teamMatches {
						if m != nil {
							tournament.TeamMatches[i] = *m
						}
					}
				}
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// This error is from one of the goroutines. If format loading failed, it's critical.
		s.logger.ErrorContext(ctx, "Error during parallel fetching in GetFullTournamentData", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
		if tournament.Format == nil { // Critical data (format) failed to load
			return nil, fmt.Errorf("critical data (format) failed to load for tournament %d: %w", tournamentID, err)
		}
		// If format loaded but other things failed, we might return a partially filled tournament.
	}

	// If after g.Wait(), tournament.Format is still nil, it's an issue from the format loading goroutine.
	if tournament.Format == nil {
		s.logger.ErrorContext(ctx, "Format was not loaded after parallel fetch for GetFullTournamentData", slog.Int("tournament_id", tournamentID))
		// This indicates a fundamental issue, perhaps return an error or a partially filled object.
		// For now, we rely on the errgroup error handling.
	}

	s.logger.DebugContext(ctx, "GetFullTournamentData: Data fetching complete.", slog.Int("tournament_id", tournamentID))
	return tournament, nil
}
