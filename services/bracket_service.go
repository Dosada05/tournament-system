package services

import (
	"context"
	// "database/sql" // Больше не нужен здесь, если только для GetFullTournamentData, но там тоже можно убрать, если репо не требуют
	"fmt"
	"log"
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
	// GenerateAndSaveBracket теперь принимает SQLExecutor
	GenerateAndSaveBracket(ctx context.Context, exec repositories.SQLExecutor, tournament *models.Tournament) (interface{}, error)
	GetFullTournamentData(ctx context.Context, tournamentID int, formatID int) (*models.Tournament, error)
}

type bracketService struct {
	formatRepo      repositories.FormatRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
}

func NewBracketService(
	formatRepo repositories.FormatRepository,
	participantRepo repositories.ParticipantRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
) BracketService {
	return &bracketService{
		formatRepo:      formatRepo,
		participantRepo: participantRepo,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
	}
}

// GenerateAndSaveBracket теперь принимает SQLExecutor (которым будет *sql.Tx из TournamentService)
func (s *bracketService) GenerateAndSaveBracket(ctx context.Context, exec repositories.SQLExecutor, tournament *models.Tournament) (interface{}, error) {
	// Важно: Этот метод теперь НЕ должен начинать или коммитить/откатывать транзакцию.
	// Это делает вызывающая сторона (TournamentService.AutoUpdateTournamentStatusesByDates).
	// Все операции записи в БД внутри этого метода должны использовать переданный 'exec'.

	if tournament.Format == nil {
		// Чтение формата: если formatRepo.GetByID не принимает SQLExecutor, он будет использовать свое соединение.
		// Это нормально, так как формат читается до каких-либо записей в текущей транзакции 'exec'.
		format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if err != nil {
			return nil, fmt.Errorf("GenerateAndSaveBracket: failed to load format %d: %w", tournament.FormatID, err)
		}
		if format == nil {
			return nil, fmt.Errorf("GenerateAndSaveBracket: format %d not found", tournament.FormatID)
		}
		tournament.Format = format
	}

	log.Printf("GenerateAndSaveBracket: Starting for tournament ID: %d within a transaction", tournament.ID)

	statusConfirmed := models.StatusParticipant
	// Чтение участников: аналогично формату, participantRepo.ListByTournament использует свое соединение.
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournament.ID, &statusConfirmed, true)
	if err != nil {
		return nil, fmt.Errorf("GenerateAndSaveBracket: failed to list participants for tournament %d: %w", tournament.ID, err)
	}

	if len(dbParticipants) < 2 { // Эта проверка уже есть в TournamentService, но здесь как дополнительная защита
		return nil, fmt.Errorf("GenerateAndSaveBracket: not enough participants (found %d, min 2)", len(dbParticipants))
	}

	var bracketGenerator brackets.BracketGenerator
	switch tournament.Format.BracketType {
	case "SingleElimination":
		bracketGenerator = brackets.NewSingleEliminationGenerator()
	default:
		return nil, fmt.Errorf("unsupported bracket type '%s'", tournament.Format.BracketType)
	}

	params := brackets.GenerateBracketParams{
		Tournament:   tournament,
		Participants: dbParticipants,
	}
	generatedBracketMatches, genErr := bracketGenerator.GenerateBracket(ctx, params)
	if genErr != nil {
		return nil, fmt.Errorf("GenerateAndSaveBracket: failed to generate structure: %w", genErr)
	}
	if len(generatedBracketMatches) == 0 && len(dbParticipants) >= 2 {
		return nil, fmt.Errorf("GenerateAndSaveBracket: no matches generated for %d participants", len(dbParticipants))
	}

	mapBracketUIDToDBMatchID := make(map[string]int)
	mapBracketUIDToModel := make(map[string]*brackets.BracketMatch)
	createdDBMatchEntities := make([]interface{}, 0, len(generatedBracketMatches))
	defaultMatchTime := tournament.StartDate
	if time.Now().After(defaultMatchTime) {
		defaultMatchTime = time.Now().Add(15 * time.Minute) // Можно вынести в константу или конфигурацию
	}

	// ПЕРВЫЙ ПРОХОД: Создаем все матчи-заготовки в БД, используя переданный 'exec'
	for _, bm := range generatedBracketMatches {
		mapBracketUIDToModel[bm.UID] = bm

		if bm.IsBye {
			if bm.ByeParticipantID != nil {
				log.Printf("GenerateAndSaveBracket: Participant ID %d (UID: %s) has a bye in Round %d. No DB match created for this bye event.", *bm.ByeParticipantID, bm.UID, bm.Round)
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
			err = s.soloMatchRepo.Create(ctx, exec, newMatch) // Используем exec
			if err != nil {
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
			err = s.teamMatchRepo.Create(ctx, exec, newMatch) // Используем exec
			if err != nil {
				return nil, fmt.Errorf("GenerateAndSaveBracket: failed to save team match (BracketUID: %s): %w", bm.UID, err)
			}
			currentDBMatchID = newMatch.ID
			currentDBMatchEntity = newMatch
		default:
			return nil, fmt.Errorf("unknown participant type '%s' for tournament %d", tournament.Format.ParticipantType, tournament.ID)
		}
		mapBracketUIDToDBMatchID[bm.UID] = currentDBMatchID
		createdDBMatchEntities = append(createdDBMatchEntities, currentDBMatchEntity)
		log.Printf("GenerateAndSaveBracket: DB Match ID %d (BracketUID: %s) created for Round %d using exec.", currentDBMatchID, bm.UID, roundNum)
	}

	// ВТОРОЙ ПРОХОД: Устанавливаем связи next_match_db_id и winner_to_slot, используя 'exec'
	for currentBracketUID, currentDBMatchID := range mapBracketUIDToDBMatchID {
		bm := mapBracketUIDToModel[currentBracketUID]

		var nextMatchDBIDForUpdate *int
		var targetSlotInNextMatchForUpdate *int

		for _, potentialTargetBm := range generatedBracketMatches {
			if potentialTargetBm.IsBye {
				continue
			}
			targetDBID, isTargetMatchInDB := mapBracketUIDToDBMatchID[potentialTargetBm.UID]
			if !isTargetMatchInDB {
				continue
			}

			isSource := false
			slot := 0
			if potentialTargetBm.SourceMatch1UID != nil && *potentialTargetBm.SourceMatch1UID == bm.UID {
				isSource = true
				slot = 1
			} else if potentialTargetBm.SourceMatch2UID != nil && *potentialTargetBm.SourceMatch2UID == bm.UID {
				isSource = true
				slot = 2
			}

			if isSource {
				nextMatchDBIDForUpdate = &targetDBID
				targetSlotInNextMatchForUpdate = &slot
				break
			}
		}

		if nextMatchDBIDForUpdate != nil {
			log.Printf("GenerateAndSaveBracket: Linking DB Match ID %d (BracketUID: %s) to Next DB Match ID %d, Slot %d using exec.",
				currentDBMatchID, bm.UID, *nextMatchDBIDForUpdate, *targetSlotInNextMatchForUpdate)
			if tournament.Format.ParticipantType == models.FormatParticipantSolo {
				err = s.soloMatchRepo.UpdateNextMatchInfo(ctx, exec, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
			} else {
				err = s.teamMatchRepo.UpdateNextMatchInfo(ctx, exec, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
			}
			if err != nil {
				return nil, fmt.Errorf("GenerateAndSaveBracket: failed to update next match info for DB match %d (BracketUID: %s): %w", currentDBMatchID, bm.UID, err)
			}
		}
	}
	// Третий проход был удален, т.к. предполагается, что генератор корректно устанавливает участников с bye
	// в поля Participant1ID/Participant2ID матчей следующего раунда, и первый проход BracketService их сохраняет.

	log.Printf("GenerateAndSaveBracket: Bracket processing completed for tournament %d using exec.", tournament.ID)
	// Возвращаем созданные сущности. Вызывающая сторона (TournamentService) решит, что с ними делать.
	// GetFullTournamentData здесь не вызывается, так как мы находимся внутри транзакции 'exec'.
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
			log.Printf("Error fetching format %d for tournament %d in GetFullTournamentData: %v", formatID, tournamentID, err)
			return fmt.Errorf("failed to fetch tournament format %d: %w", formatID, err)
		}
		if format == nil {
			log.Printf("Format %d not found for tournament %d in GetFullTournamentData", formatID, tournamentID)
			return fmt.Errorf("tournament format %d not found", formatID)
		}
		tournament.Format = format
		return nil
	})

	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		participants, err := s.participantRepo.ListByTournament(gCtx, tournamentID, &confirmedStatus, true)
		if err != nil {
			log.Printf("Error fetching confirmed participants for tournament %d in GetFullTournamentData: %v", tournamentID, err)
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
		soloMatches, err := s.soloMatchRepo.ListByTournament(gCtx, tournamentID, nil, nil)
		if err != nil {
			log.Printf("Error fetching solo matches for tournament %d in GetFullTournamentData: %v", tournamentID, err)
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
		return nil
	})

	g.Go(func() error {
		teamMatches, err := s.teamMatchRepo.ListByTournament(gCtx, tournamentID, nil, nil)
		if err != nil {
			log.Printf("Error fetching team matches for tournament %d in GetFullTournamentData: %v", tournamentID, err)
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
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Printf("Error during parallel fetching in GetFullTournamentData for tournament %d: %v", tournamentID, err)
		if tournament.Format == nil {
			return nil, fmt.Errorf("critical data (format) failed to load for tournament %d: %w", tournamentID, err)
		}
	}
	return tournament, nil
}
