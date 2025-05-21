package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"golang.org/x/sync/errgroup"
)

var ErrTournamentRequiresTransaction = fmt.Errorf("database transaction is required for this operation")

type TournamentWinnerPayload struct {
	TournamentID   int                 `json:"tournament_id"`
	Winner         *models.Participant `json:"winner"`
	Message        string              `json:"message"`
	IsAutoWin      bool                `json:"is_auto_win"`
	ParticipantIds []int               `json:"participant_ids"`
}

type BracketService interface {
	GenerateAndSaveBracket(ctx context.Context, tournament *models.Tournament) (interface{}, error)
	GetFullTournamentData(ctx context.Context, tournamentID int, formatID int) (*models.Tournament, error)
}

type bracketService struct {
	db              *sql.DB
	formatRepo      repositories.FormatRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
}

func NewBracketService(
	db *sql.DB,
	formatRepo repositories.FormatRepository,
	participantRepo repositories.ParticipantRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
) BracketService {
	return &bracketService{
		db:              db,
		formatRepo:      formatRepo,
		participantRepo: participantRepo,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
	}
}

func (s *bracketService) GenerateAndSaveBracket(ctx context.Context, tournament *models.Tournament) (interface{}, error) {
	if tournament.Format == nil {
		format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if err != nil {
			return nil, fmt.Errorf("failed to load format %d for bracket generation: %w", tournament.FormatID, err)
		}
		if format == nil {
			return nil, fmt.Errorf("format %d not found, though no error was returned", tournament.FormatID)
		}
		tournament.Format = format
	}

	log.Printf("Starting bracket generation for tournament ID: %d, Format: %s, ParticipantType: %s, BracketType: %s",
		tournament.ID, tournament.Format.Name, tournament.Format.ParticipantType, tournament.Format.BracketType)

	statusConfirmed := models.StatusParticipant
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournament.ID, &statusConfirmed, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list confirmed participants for tournament %d: %w", tournament.ID, err)
	}

	if len(dbParticipants) == 0 {
		return nil, fmt.Errorf("no confirmed participants to generate bracket for tournament %d", tournament.ID)
	}
	if len(dbParticipants) < 2 {
		return nil, fmt.Errorf("not enough participants to generate bracket (minimum 2 required, found %d)", len(dbParticipants))
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
		return nil, fmt.Errorf("failed to generate bracket structure for tournament %d: %w", tournament.ID, genErr)
	}
	if len(generatedBracketMatches) == 0 && len(dbParticipants) >= 2 { // Уточнено условие
		return nil, fmt.Errorf("bracket generation resulted in no matches for %d participants", len(dbParticipants))
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	var txErr error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if txErr != nil {
			log.Printf("Rolling back transaction due to error: %v", txErr)
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("Error during rollback: %v. Original error: %v", rbErr, txErr)
				txErr = fmt.Errorf("transaction processing error: %w (rollback also failed: %v)", txErr, rbErr)
			}
		} else {
			if cErr := tx.Commit(); cErr != nil {
				log.Printf("Failed to commit transaction for tournament %d: %v", tournament.ID, cErr)
				txErr = fmt.Errorf("failed to commit transaction: %w", cErr)
			} else {
				log.Printf("Transaction committed for tournament %d bracket.", tournament.ID)
			}
		}
	}()

	mapBracketUIDToDBMatchID := make(map[string]int)
	mapBracketUIDToModel := make(map[string]*brackets.BracketMatch) // Для второго прохода
	createdDBMatchEntities := make([]interface{}, 0, len(generatedBracketMatches))

	defaultMatchTime := tournament.StartDate
	if time.Now().After(defaultMatchTime) {
		defaultMatchTime = time.Now().Add(15 * time.Minute)
	}

	// ПЕРВЫЙ ПРОХОД: Создаем все матчи-заготовки в БД
	for _, bm := range generatedBracketMatches {
		mapBracketUIDToModel[bm.UID] = bm // Сохраняем для второго прохода

		if bm.IsBye {
			// Участник bm.ByeParticipantID (который также должен быть в bm.Participant1ID)
			// просто проходит дальше. Запись в БД для этого "bye-матча" не создается.
			// Его продвижение учтено генератором при формировании узлов для следующего раунда.
			if bm.ByeParticipantID != nil {
				log.Printf("Participant ID %d (UID: %s) has a bye in Round %d. No DB match created for this bye event.", *bm.ByeParticipantID, bm.UID, bm.Round)
			}
			continue
		}

		roundNum := bm.Round
		var currentDBMatchID int
		var currentDBMatchEntity interface{}
		bmUIDStr := bm.UID

		// Участники bm.Participant1ID и bm.Participant2ID уже должны быть корректно
		// установлены генератором, если они известны (включая тех, кто прошел по bye
		// в предыдущем "слое" обработки узлов генератором).
		// Если они nil, значит, это плейсхолдеры для победителей предыдущих матчей.
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
			txErr = s.soloMatchRepo.Create(ctx, tx, newMatch)
			if txErr != nil {
				return nil, txErr
			}
			currentDBMatchID = newMatch.ID
			currentDBMatchEntity = newMatch
		case models.FormatParticipantTeam:
			newMatch := &models.TeamMatch{
				TournamentID:    tournament.ID,
				T1ParticipantID: bm.Participant1ID, // Используем Participant1ID/2ID для обоих типов
				T2ParticipantID: bm.Participant2ID,
				MatchTime:       defaultMatchTime,
				Status:          models.StatusScheduled,
				Round:           &roundNum,
				BracketMatchUID: &bmUIDStr,
			}
			txErr = s.teamMatchRepo.Create(ctx, tx, newMatch)
			if txErr != nil {
				return nil, txErr
			}
			currentDBMatchID = newMatch.ID
			currentDBMatchEntity = newMatch
		default:
			txErr = fmt.Errorf("unknown participant type '%s'", tournament.Format.ParticipantType)
			return nil, txErr
		}
		mapBracketUIDToDBMatchID[bm.UID] = currentDBMatchID
		createdDBMatchEntities = append(createdDBMatchEntities, currentDBMatchEntity)
		log.Printf("DB Match ID %d (BracketUID: %s) created for Round %d", currentDBMatchID, bm.UID, roundNum)
	}
	if txErr != nil {
		return nil, txErr
	}

	// ВТОРОЙ ПРОХОД: Устанавливаем связи next_match_db_id и winner_to_slot
	for currentBracketUID, currentDBMatchID := range mapBracketUIDToDBMatchID {
		bm := mapBracketUIDToModel[currentBracketUID] // Гарантированно существует, т.к. currentBracketUID из ключей карты

		var nextMatchDBIDForUpdate *int
		var targetSlotInNextMatchForUpdate *int

		for _, potentialTargetBm := range generatedBracketMatches {
			if potentialTargetBm.IsBye {
				continue
			} // Следующий матч не может быть "bye-слотом"

			targetDBID, isTargetMatchInDB := mapBracketUIDToDBMatchID[potentialTargetBm.UID]
			if !isTargetMatchInDB {
				continue
			} // Если по какой-то причине целевой матч не был создан в БД

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
			log.Printf("Linking DB Match ID %d (BracketUID: %s) to Next DB Match ID %d, Slot %d",
				currentDBMatchID, bm.UID, *nextMatchDBIDForUpdate, *targetSlotInNextMatchForUpdate)
			if tournament.Format.ParticipantType == models.FormatParticipantSolo {
				txErr = s.soloMatchRepo.UpdateNextMatchInfo(ctx, tx, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
			} else {
				txErr = s.teamMatchRepo.UpdateNextMatchInfo(ctx, tx, currentDBMatchID, nextMatchDBIDForUpdate, targetSlotInNextMatchForUpdate)
			}
			if txErr != nil {
				return nil, txErr
			}
		}
	}
	if txErr != nil {
		return nil, txErr
	}

	// Третий проход больше не нужен, так как генератор должен корректно устанавливать
	// участников, прошедших по bye, в поля Participant1ID/Participant2ID
	// матчей следующего раунда, и первый проход BracketService их уже сохранит.

	if txErr != nil { // Проверка после всех операций перед возвратом из defer
		return nil, txErr
	}

	log.Printf("Bracket processing completed for tournament %d. Fetching full data.", tournament.ID)
	fullBracketData, fetchErr := s.GetFullTournamentData(context.Background(), tournament.ID, tournament.FormatID)
	if fetchErr != nil {
		log.Printf("Bracket saved for tournament %d, but failed to fetch full data for response: %v. Returning created DB matches.", tournament.ID, fetchErr)
		return createdDBMatchEntities, nil
	}

	return fullBracketData, nil
}

func getParticipantName(p *models.Participant) string {
	if p == nil {
		return "Unknown Participant"
	}
	if p.User != nil {
		if p.User.Nickname != nil && *p.User.Nickname != "" {
			return *p.User.Nickname
		}
		if p.User.FirstName != "" {
			name := p.User.FirstName
			if p.User.LastName != "" {
				name += " " + p.User.LastName
			}
			return name
		}
	}
	if p.Team != nil && p.Team.Name != "" {
		return p.Team.Name
	}
	if p.ID != 0 { // Если есть хотя бы Participant.ID
		return fmt.Sprintf("Participant (ID: %d)", p.ID)
	}
	return "Unnamed Participant"
}

func (s *bracketService) GetFullTournamentData(ctx context.Context, tournamentID int, formatID int) (*models.Tournament, error) {
	// Инициализируем турнир только с ID и FormatID, остальное загрузим.
	// Основные детали турнира (имя, описание и т.д.) должны быть загружены
	// вызывающим TournamentService, если это необходимо для полного ответа.
	// Этот метод фокусируется на данных, связанных с сеткой.
	tournament := &models.Tournament{ID: tournamentID, FormatID: formatID}

	g, gCtx := errgroup.WithContext(ctx)

	// 1. Загрузка формата турнира
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

	// 2. Загрузка подтвержденных участников
	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		// participantRepo.ListByTournament должен уметь загружать User/Team детали
		participants, err := s.participantRepo.ListByTournament(gCtx, tournamentID, &confirmedStatus, true)
		if err != nil {
			log.Printf("Error fetching confirmed participants for tournament %d in GetFullTournamentData: %v", tournamentID, err)
			// Не возвращаем ошибку, чтобы остальные данные могли загрузиться, если это не критично
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

	// 3. Загрузка соло матчей
	g.Go(func() error {
		// soloMatchRepo.ListByTournament должен уметь загружать детали P1/P2, если это настроено
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

	// 4. Загрузка командных матчей
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
		// Если любая из горутин вернула ошибку, она будет здесь.
		// Однако, если горутины только логируют ошибку и возвращают nil, то этой ошибки не будет.
		log.Printf("Error during parallel fetching in GetFullTournamentData for tournament %d: %v", tournamentID, err)
		// В зависимости от критичности, можно вернуть nil, err или частично заполненный tournament.
		// Пока что, если была ошибка в одной из критичных загрузок (например, формат), вернем ошибку.
		if tournament.Format == nil { // Если формат не загрузился, это проблема
			return nil, fmt.Errorf("critical data (format) failed to load for tournament %d: %w", tournamentID, err)
		}
		// Иначе, возвращаем что есть, ошибки залогированы.
	}
	return tournament, nil
}
