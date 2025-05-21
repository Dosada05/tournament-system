// File: tournament-system/services/tournament_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
	"golang.org/x/sync/errgroup"
)

const (
	tournamentLogoPrefix = "logos/tournaments"
)

var (
	ErrTournamentNameRequired       = errors.New("tournament name is required")
	ErrTournamentDatesRequired      = errors.New("registration, start, and end dates are required")
	ErrTournamentSportNotFound      = errors.New("specified sport not found")
	ErrTournamentFormatNotFound     = errors.New("specified format not found")
	ErrTournamentOrganizerNotFound  = errors.New("specified organizer user not found")
	ErrTournamentCreationFailed     = errors.New("failed to create tournament")
	ErrTournamentUpdateFailed       = errors.New("failed to update tournament")
	ErrTournamentDeleteFailed       = errors.New("failed to delete tournament")
	ErrTournamentListFailed         = errors.New("failed to list tournaments")
	ErrTournamentUpdateNotAllowed   = errors.New("tournament update not allowed in current status")
	ErrTournamentDeletionNotAllowed = errors.New("tournament deletion not allowed")
	ErrTournamentInUse              = repositories.ErrTournamentInUse
)

type TournamentService interface {
	CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, status models.TournamentStatus) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int, currentUserID int) error
	UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error)
	FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantID int, currentUserID int) (*models.Tournament, error)
	GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error) // Используем новую структуру для ответа
}

type FullTournamentBracketView struct {
	TournamentID               int                     `json:"tournament_id"`
	Name                       string                  `json:"name"`
	Status                     models.TournamentStatus `json:"status"`
	Sport                      *models.Sport           `json:"sport,omitempty"`
	Format                     *models.Format          `json:"format,omitempty"`
	Rounds                     []RoundView             `json:"rounds"`
	ParticipantsMap            map[int]ParticipantView `json:"participants_map,omitempty"`
	OverallWinnerParticipantID *int                    `json:"overall_winner_participant_id,omitempty"`
}

type RoundView struct {
	RoundNumber int         `json:"round_number"`
	Matches     []MatchView `json:"matches"`
}

type MatchView struct {
	MatchID               int                `json:"match_id"` // DB ID матча
	BracketMatchUID       *string            `json:"bracket_match_uid,omitempty"`
	Status                models.MatchStatus `json:"status"`
	Round                 int                `json:"round"`
	OrderInRound          int                `json:"order_in_round"` // Полезно для сортировки и отображения
	Participant1          *ParticipantView   `json:"participant1,omitempty"`
	Participant2          *ParticipantView   `json:"participant2,omitempty"`
	ScoreP1               *int               `json:"score_p1,omitempty"` // Если счет ведется по участникам
	ScoreP2               *int               `json:"score_p2,omitempty"`
	ScoreString           *string            `json:"score_string,omitempty"` // Общий счет строкой
	WinnerParticipantDBID *int               `json:"winner_participant_db_id,omitempty"`
	NextMatchDBID         *int               `json:"next_match_db_id,omitempty"`
	WinnerToSlot          *int               `json:"winner_to_slot,omitempty"`
	MatchTime             time.Time          `json:"match_time"`
}

type ParticipantView struct {
	ParticipantDBID int     `json:"participant_db_id"` // ID из таблицы participants
	Type            string  `json:"type"`              // "team" или "user"
	Name            string  `json:"name"`
	LogoURL         *string `json:"logo_url,omitempty"`
	// Можно добавить оригинальные UserID/TeamID, если нужно клиенту
	OriginalUserID *int `json:"original_user_id,omitempty"`
	OriginalTeamID *int `json:"original_team_id,omitempty"`
}

type CreateTournamentInput struct {
	Name            string    `json:"name" validate:"required"`
	Description     *string   `json:"description"`
	SportID         int       `json:"sport_id" validate:"required,gt=0"`
	FormatID        int       `json:"format_id" validate:"required,gt=0"`
	RegDate         time.Time `json:"reg_date" validate:"required"`
	StartDate       time.Time `json:"start_date" validate:"required"`
	EndDate         time.Time `json:"end_date" validate:"required"`
	Location        *string   `json:"location"`
	MaxParticipants int       `json:"max_participants" validate:"required,gt=0"`
}

type UpdateTournamentDetailsInput struct {
	Name            *string    `json:"name"`
	Description     *string    `json:"description"`
	RegDate         *time.Time `json:"reg_date"`
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	Location        *string    `json:"location"`
	MaxParticipants *int       `json:"max_participants" validate:"omitempty,gt=0"`
}

type ListTournamentsFilter struct {
	SportID     *int
	FormatID    *int
	OrganizerID *int
	Status      *models.TournamentStatus
	Limit       int
	Offset      int
}

type tournamentService struct {
	tournamentRepo  repositories.TournamentRepository
	sportRepo       repositories.SportRepository
	formatRepo      repositories.FormatRepository
	userRepo        repositories.UserRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
	bracketService  BracketService
	matchService    MatchService
	uploader        storage.FileUploader
	hub             *brackets.Hub
}

func NewTournamentService(
	tournamentRepo repositories.TournamentRepository,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
	participantRepo repositories.ParticipantRepository,
	bracketService BracketService,
	matchService MatchService,
	uploader storage.FileUploader,
	hub *brackets.Hub,
) TournamentService {
	return &tournamentService{
		tournamentRepo:  tournamentRepo,
		sportRepo:       sportRepo,
		formatRepo:      formatRepo,
		userRepo:        userRepo,
		participantRepo: participantRepo,
		bracketService:  bracketService,
		matchService:    matchService,
		uploader:        uploader,
		hub:             hub,
	}
}

func (s *tournamentService) populateTournamentLogoURL(tournament *models.Tournament) {
	if tournament != nil && tournament.LogoKey != nil && *tournament.LogoKey != "" && s.uploader != nil {
		url := s.uploader.GetPublicURL(*tournament.LogoKey)
		if url != "" {
			tournament.LogoURL = &url
		}
	}
}

func (s *tournamentService) populateTournamentListLogoURLs(tournaments []models.Tournament) {
	for i := range tournaments {
		s.populateTournamentLogoURL(&tournaments[i])
	}
}

func (s *tournamentService) populateUserDetails(user *models.User) {
	if user == nil {
		return
	}
	user.PasswordHash = ""
	if user.LogoKey != nil && *user.LogoKey != "" && s.uploader != nil {
		url := s.uploader.GetPublicURL(*user.LogoKey)
		if url != "" {
			user.LogoURL = &url
		}
	}
}

func (s *tournamentService) CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTournamentNameRequired
	}
	if err := validateTournamentDates(input.RegDate, input.StartDate, input.EndDate); err != nil {
		return nil, err
	}
	if input.MaxParticipants <= 0 {
		return nil, ErrTournamentInvalidCapacity
	}

	_, err := s.sportRepo.GetByID(ctx, input.SportID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentSportNotFound, "failed to verify sport %d", input.SportID)
	}
	_, err = s.formatRepo.GetByID(ctx, input.FormatID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentFormatNotFound, "failed to verify format %d", input.FormatID)
	}
	_, err = s.userRepo.GetByID(ctx, organizerID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentOrganizerNotFound, "failed to verify organizer %d", organizerID)
	}

	tournament := &models.Tournament{
		Name:            name,
		Description:     input.Description,
		SportID:         input.SportID,
		FormatID:        input.FormatID,
		OrganizerID:     organizerID,
		RegDate:         input.RegDate,
		StartDate:       input.StartDate,
		EndDate:         input.EndDate,
		Location:        input.Location,
		MaxParticipants: input.MaxParticipants,
		Status:          models.StatusSoon,
	}

	err = s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentCreationFailed, err)
	}
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament by id %d", id)
	}
	s.populateTournamentLogoURL(tournament)

	g, gCtx := errgroup.WithContext(ctx)

	// Загрузка формата (всегда нужна)
	if tournament.FormatID > 0 {
		g.Go(func() error {
			format, formatErr := s.formatRepo.GetByID(gCtx, tournament.FormatID)
			if formatErr != nil {
				log.Printf("Warning: failed to fetch format %d for tournament %d: %v\n", tournament.FormatID, id, formatErr)
				return nil // Не делаем ошибку критической, если другие данные могут быть полезны
			}
			tournament.Format = format
			return nil
		})
	}

	// Загрузка участников (подтвержденных)
	// Вне зависимости от статуса, может быть полезно видеть подтвержденных участников
	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		dbParticipants, participantsErr := s.participantRepo.ListByTournament(gCtx, id, &confirmedStatus, true)
		if participantsErr != nil {
			log.Printf("Warning: failed to fetch confirmed participants for tournament %d: %v\n", id, participantsErr)
		} else {
			s.populateParticipantListDetails(dbParticipants)
			tournament.Participants = ParticipantsToInterface(dbParticipants)
		}
		return nil
	})

	// Загрузка матчей, если турнир активен или завершен (через MatchService)
	if tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted {
		g.Go(func() error {
			soloMatches, soloErr := s.matchService.ListSoloMatchesByTournament(gCtx, id)
			if soloErr != nil {
				log.Printf("Warning: failed to fetch solo matches for tournament %d: %v\n", id, soloErr)
			} else {
				tournament.SoloMatches = SoloMatchesToInterface(soloMatches) // Используем хелпер, если soloMatches это []*models.SoloMatch
			}
			return nil
		})

		g.Go(func() error {
			teamMatches, teamErr := s.matchService.ListTeamMatchesByTournament(gCtx, id)
			if teamErr != nil {
				log.Printf("Warning: failed to fetch team matches for tournament %d: %v\n", id, teamErr)
			} else {
				tournament.TeamMatches = TeamMatchesToInterface(teamMatches) // Используем хелпер, если teamMatches это []*models.TeamMatch
			}
			return nil
		})
	}

	// Загрузка спорта
	if tournament.SportID > 0 {
		g.Go(func() error {
			sport, sportErr := s.sportRepo.GetByID(gCtx, tournament.SportID)
			if sportErr != nil {
				log.Printf("Warning: failed to fetch sport %d for tournament %d: %v\n", tournament.SportID, id, sportErr)
				return nil
			}
			s.populateSportLogoURL(sport)
			tournament.Sport = sport
			return nil
		})
	}

	// Загрузка организатора
	if tournament.OrganizerID > 0 {
		g.Go(func() error {
			organizer, orgErr := s.userRepo.GetByID(gCtx, tournament.OrganizerID)
			if orgErr != nil {
				log.Printf("Warning: failed to fetch organizer %d for tournament %d: %v\n", tournament.OrganizerID, id, orgErr)
				return nil
			}
			s.populateUserDetails(organizer)
			tournament.Organizer = organizer
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// Эта ошибка будет возвращена, если какая-либо из горутин вернула ошибку (не nil)
		log.Printf("Error during parallel fetching of tournament details for tournament %d: %v\n", id, err)
		// Можно решить, возвращать ли частично заполненный турнир или ошибку.
		// Если горутины логируют ошибки и возвращают nil, то эта ветка не будет достигнута для таких случаев.
	}

	return tournament, nil
}

func (s *tournamentService) ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error) {
	repoFilter := repositories.ListTournamentsFilter{
		SportID:     filter.SportID,
		FormatID:    filter.FormatID,
		OrganizerID: filter.OrganizerID,
		Status:      filter.Status,
		Limit:       filter.Limit,
		Offset:      filter.Offset,
	}
	tournaments, err := s.tournamentRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTournamentListFailed, err)
	}
	if tournaments == nil {
		return []models.Tournament{}, nil
	}
	s.populateTournamentListLogoURLs(tournaments)
	return tournaments, nil
}

func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for update", id)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration {
		return nil, fmt.Errorf("%w: cannot update tournament in status '%s'", ErrTournamentUpdateNotAllowed, tournament.Status)
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrTournamentNameRequired
		}
		if trimmedName != tournament.Name {
			tournament.Name = trimmedName
			updated = true
		}
	}
	if input.Description != nil && derefString(input.Description) != derefString(tournament.Description) {
		tournament.Description = input.Description
		updated = true
	}
	if input.RegDate != nil && !input.RegDate.IsZero() && !input.RegDate.Equal(tournament.RegDate) {
		tournament.RegDate = *input.RegDate
		updated = true
	}
	if input.StartDate != nil && !input.StartDate.IsZero() && !input.StartDate.Equal(tournament.StartDate) {
		tournament.StartDate = *input.StartDate
		updated = true
	}
	if input.EndDate != nil && !input.EndDate.IsZero() && !input.EndDate.Equal(tournament.EndDate) {
		tournament.EndDate = *input.EndDate
		updated = true
	}
	if input.Location != nil && derefString(input.Location) != derefString(tournament.Location) {
		tournament.Location = input.Location
		updated = true
	}
	if input.MaxParticipants != nil && *input.MaxParticipants != tournament.MaxParticipants {
		if *input.MaxParticipants <= 0 {
			return nil, ErrTournamentInvalidCapacity
		}
		tournament.MaxParticipants = *input.MaxParticipants
		updated = true
	}

	if updated {
		if err := validateTournamentDates(tournament.RegDate, tournament.StartDate, tournament.EndDate); err != nil {
			return nil, err
		}
	}

	if !updated {
		s.populateTournamentLogoURL(tournament)
		return tournament, nil
	}

	err = s.tournamentRepo.Update(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: error from repository on update", ErrTournamentUpdateFailed)
	}
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus) (*models.Tournament, error) {
	if !isValidTournamentStatus(newStatus) {
		return nil, fmt.Errorf("%w: '%s'", ErrTournamentInvalidStatus, newStatus)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for status update", id)
	}

	if tournament.Format == nil && tournament.FormatID >= 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			log.Printf("Warning: could not load format %d for tournament %d during status update: %v", tournament.FormatID, id, formatErr)
			if newStatus == models.StatusActive {
				return nil, fmt.Errorf("cannot activate tournament %d: failed to load its format %d: %w", id, tournament.FormatID, formatErr)
			}
		} else {
			tournament.Format = format
		}
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	currentStatus := tournament.Status
	if !isValidStatusTransition(currentStatus, newStatus) {
		return nil, fmt.Errorf("%w: from '%s' to '%s'", ErrTournamentInvalidStatusTransition, currentStatus, newStatus)
	}

	if newStatus == models.StatusActive && currentStatus != models.StatusActive {
		if tournament.Format == nil {
			return nil, fmt.Errorf("cannot activate tournament %d: format information is missing or failed to load", id)
		}
		log.Printf("Tournament %d status changing to Active. Attempting to generate bracket via BracketService.", id)

		bracketDataInterface, genErr := s.bracketService.GenerateAndSaveBracket(ctx, tournament)
		if genErr != nil {
			log.Printf("Error generating bracket for tournament %d: %v", id, genErr)
			errorPayload := map[string]string{"error": fmt.Sprintf("Failed to generate bracket: %v", genErr)}
			if s.hub != nil {
				roomID := "tournament_" + strconv.Itoa(id)
				s.hub.BroadcastToRoom(
					roomID,
					brackets.WebSocketMessage{Type: "BRACKET_GENERATION_ERROR", Payload: errorPayload, RoomID: roomID},
				)
			}
			return nil, fmt.Errorf("failed to generate bracket upon activating tournament %d: %w", id, genErr)
		}

		// Обработка результата от BracketService
		if winnerPayload, ok := bracketDataInterface.(TournamentWinnerPayload); ok && winnerPayload.IsAutoWin {
			// Случай автоматической победы (1 участник)
			log.Printf("Tournament %d: Auto-winner detected (Participant: %s). Setting status to Completed.", id, getParticipantName(winnerPayload.Winner))

			// Обновляем статус турнира на Завершен
			// Важно: isValidStatusTransition должна разрешать переход из Registration (или Soon) в Completed в этом случае,
			// или мы должны это обработать как специальный случай.
			// Пока предполагаем, что такой переход разрешен или будет обработан логикой.
			// Для большей чистоты, возможно, стоит иметь отдельный метод для завершения турнира с победителем.
			if !isValidStatusTransition(currentStatus, models.StatusCompleted) && currentStatus != models.StatusCompleted {
				// Если прямой переход в Completed не разрешен из текущего статуса (кроме если уже Completed)
				// можно сначала перевести в Active (если это было намерение), а потом сразу в Completed.
				// Но для авто-победы логичнее сразу в Completed.
				// Рассмотрим сценарий: Registration -> Active (вызов Generate) -> AutoWin -> Completed.
				// Если currentStatus был Registration, и мы переводим в Active, а получаем AutoWin,
				// то финальный статус должен быть Completed.
				log.Printf("Transitioning tournament %d from %s to %s due to auto-win.", id, currentStatus, models.StatusCompleted)
			}

			err = s.tournamentRepo.UpdateStatus(ctx, id, models.StatusCompleted)
			if err != nil {
				log.Printf("CRITICAL: Auto-winner declared for tournament %d, but failed to update tournament status to Completed: %v", id, err)
				// Возвращаем ошибку, но информация о победителе уже есть в winnerPayload
				return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: failed to update status to Completed after auto-win", ErrTournamentUpdateFailed)
			}
			tournament.Status = models.StatusCompleted // Обновляем статус в объекте

			// TODO: Если есть поле для победителя в models.Tournament, обновить его здесь.
			// tournament.WinnerParticipantID = &winnerPayload.Winner.ID (пример)
			// err = s.tournamentRepo.Update(ctx, tournament) // и сохранить

			if s.hub != nil {
				roomID := "tournament_" + strconv.Itoa(id)
				s.hub.BroadcastToRoom(
					roomID,
					brackets.WebSocketMessage{Type: "TOURNAMENT_AUTO_WINNER", Payload: winnerPayload, RoomID: roomID},
				)
			}
			log.Printf("Tournament %d completed with auto-winner. WebSocket notification sent.", id)
			// Возвращаем обновленный турнир (хотя данные о сетке могут быть нерелевантны)
			// Можно загрузить актуальные данные турнира еще раз или просто вернуть текущий объект.
			// Для простоты возвращаем текущий.
			s.populateTournamentLogoURL(tournament)
			return tournament, nil

		} else if bracketData, ok := bracketDataInterface.(*models.Tournament); ok {
			// Обычная генерация сетки, матчи созданы
			err = s.tournamentRepo.UpdateStatus(ctx, id, newStatus) // newStatus здесь models.StatusActive
			if err != nil {
				log.Printf("CRITICAL: Bracket generated for tournament %d, but failed to update tournament status to Active: %v", id, err)
				return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: failed to update status to Active after bracket generation", ErrTournamentUpdateFailed)
			}
			tournament.Status = newStatus

			if s.hub != nil {
				roomID := "tournament_" + strconv.Itoa(id)
				s.hub.BroadcastToRoom(
					roomID,
					brackets.WebSocketMessage{Type: "BRACKET_UPDATED", Payload: bracketData, RoomID: roomID},
				)
			}
			log.Printf("Bracket generated and status updated for tournament %d. WebSocket notification sent.", id)
			tournament = bracketData      // Используем данные, возвращенные BracketService
			tournament.Status = newStatus // Убеждаемся, что статус актуален
		} else {
			// Неожиданный тип данных от BracketService
			log.Printf("Error: Unexpected data type received from BracketService for tournament %d: %T", id, bracketDataInterface)
			// Можно вернуть ошибку или обработать как ошибку генерации сетки
			return nil, fmt.Errorf("unexpected data type from bracket generation for tournament %d", id)
		}

	} else { // Если статус меняется не на Active (или уже был Active), или это не переход в Active
		err = s.tournamentRepo.UpdateStatus(ctx, id, newStatus)
		if err != nil {
			return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: failed to update status in repository", ErrTournamentUpdateFailed)
		}
		tournament.Status = newStatus
		if s.hub != nil {
			roomID := "tournament_" + strconv.Itoa(id)
			payload := map[string]interface{}{"tournament_id": id, "new_status": newStatus}
			s.hub.BroadcastToRoom(
				roomID,
				brackets.WebSocketMessage{Type: "TOURNAMENT_STATUS_UPDATED", Payload: payload, RoomID: roomID},
			)
		}
	}

	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) DeleteTournament(ctx context.Context, id int, currentUserID int) error {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for deletion check", id)
	}

	if tournament.OrganizerID != currentUserID { // TODO: Add admin role check
		return ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status '%s'", ErrTournamentDeletionNotAllowed, tournament.Status)
	}

	oldLogoKey := tournament.LogoKey
	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentInUse) {
			return fmt.Errorf("%w: tournament might have participants or matches: %w", ErrTournamentDeletionNotAllowed, err)
		}
		return handleRepositoryError(err, ErrTournamentNotFound, "%w", ErrTournamentDeleteFailed)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && s.uploader != nil {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				log.Printf("Warning: Failed to delete tournament logo %s during tournament deletion: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}
	return nil
}

func (s *tournamentService) UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for logo upload", tournamentID)
	}

	if tournament.OrganizerID != currentUserID { // TODO: Add admin role check
		return nil, ErrForbiddenOperation
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat
	}
	if s.uploader == nil {
		return nil, errors.New("file uploader is not configured")
	}

	oldLogoKey := tournament.LogoKey
	ext, err := GetExtensionFromContentType(contentType)
	if err != nil {
		return nil, err
	}
	newKey := fmt.Sprintf("%s/%d/logo_%d%s", tournamentLogoPrefix, tournamentID, time.Now().UnixNano(), ext)

	if _, err = s.uploader.Upload(ctx, newKey, contentType, file); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrLogoUploadFailed, newKey, err)
	}

	if err = s.tournamentRepo.UpdateLogoKey(ctx, tournamentID, &newKey); err != nil {
		go s.uploader.Delete(context.Background(), newKey) // Attempt to clean up uploaded file
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: failed to update logo key in db", ErrLogoUpdateDatabaseFailed)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) { // Delete old logo in background
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				log.Printf("Warning: Failed to delete old tournament logo %s: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}

	tournament.LogoKey = &newKey
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantID int, currentUserID int) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "FinalizeTournament: failed to get tournament %d", tournamentID)
	}

	// Проверка прав: только организатор может финализировать (или админ)
	if tournament.OrganizerID != currentUserID {
		// TODO: Добавить проверку на роль администратора, если это предусмотрено
		return nil, ErrForbiddenOperation
	}

	if tournament.Status == models.StatusCompleted {
		log.Printf("Tournament %d already completed.", tournamentID)
		// Можно вернуть текущее состояние или ошибку, что уже завершен. Пока вернем текущее.
		s.populateTournamentDetails(ctx, tournament) // Загрузим детали для ответа
		return tournament, nil
	}

	if tournament.Status != models.StatusActive {
		return nil, fmt.Errorf("cannot finalize tournament %d with status '%s', expected '%s'",
			tournamentID, tournament.Status, models.StatusActive)
	}

	// Проверка, что переданный winnerParticipantID действительно участник этого турнира
	winnerParticipant, err := s.participantRepo.FindByID(ctx, winnerParticipantID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrParticipantNotFound, "FinalizeTournament: winner participant ID %d not found", winnerParticipantID)
	}
	if winnerParticipant.TournamentID != tournamentID {
		return nil, fmt.Errorf("winner participant ID %d does not belong to tournament %d", winnerParticipantID, tournamentID)
	}

	// Обновляем статус турнира
	err = s.tournamentRepo.UpdateStatus(ctx, tournamentID, models.StatusCompleted)
	if err != nil {
		return nil, fmt.Errorf("FinalizeTournament: failed to update tournament %d status to completed: %w", tournamentID, err)
	}
	tournament.Status = models.StatusCompleted

	// Опционально: сохранить победителя в модели Tournament, если есть такое поле
	// if tournament.WinnerOverallParticipantID == nil { // Пример
	// 	tournament.WinnerOverallParticipantID = &winnerParticipantID
	// 	err = s.tournamentRepo.Update(ctx, tournament) // Если есть общий Update метод, который это поле учтет
	// 	if err != nil {
	// 		log.Printf("Warning: Failed to set overall winner for tournament %d: %v", tournamentID, err)
	// 	}
	// }

	log.Printf("Tournament %d finalized. Winner Participant ID: %d", tournamentID, winnerParticipantID)

	// Отправка WebSocket уведомления о завершении турнира
	if s.hub != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		completionPayload := map[string]interface{}{
			"tournament_id":            tournamentID,
			"winner_participant_db_id": winnerParticipantID,
			// Можно обогатить информацией о победителе (имя, лого и т.д.)
		}
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_COMPLETED", Payload: completionPayload, RoomID: roomID})
		log.Printf("Sent TOURNAMENT_COMPLETED for tournament %d to room %s, Winner: %d", tournamentID, roomID, winnerParticipantID)
	}

	s.populateTournamentDetails(ctx, tournament) // Загрузить детали для ответа, включая нового победителя если он есть в модели турнира
	return tournament, nil
}

func (s *tournamentService) GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error) {
	// 1. Загружаем основную информацию о турнире
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "GetTournamentBracketData: failed to get tournament %d", tournamentID)
	}
	s.populateTournamentLogoURL(tournament) // Лого турнира

	// 2. Загружаем формат (если он еще не загружен с турниром)
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			log.Printf("Warning: GetTournamentBracketData: could not load format %d for tournament %d: %v", tournament.FormatID, tournamentID, formatErr)
			// Можно вернуть ошибку или продолжить без формата, если это допустимо
		}
		tournament.Format = format
	}
	if tournament.Format == nil {
		return nil, fmt.Errorf("format information missing for tournament %d", tournamentID)
	}

	// 3. Загружаем спорт (если еще не загружен)
	if tournament.Sport == nil && tournament.SportID > 0 {
		sport, sportErr := s.sportRepo.GetByID(ctx, tournament.SportID)
		if sportErr == nil && sport != nil {
			s.populateSportLogoURL(sport)
			tournament.Sport = sport
		}
	}

	// 4. Загружаем всех подтвержденных участников турнира с деталями
	statusConfirmed := models.StatusParticipant
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournamentID, &statusConfirmed, true) // true для загрузки User/Team
	if err != nil {
		// Можно решить, возвращать ли ошибку или пустую карту участников
		log.Printf("Warning: GetTournamentBracketData: failed to list participants for tournament %d: %v", tournamentID, err)
	}
	participantsMap := make(map[int]ParticipantView)
	for _, p := range dbParticipants {
		if p == nil {
			continue
		}
		view := ParticipantView{
			ParticipantDBID: p.ID,
			OriginalUserID:  p.UserID,
			OriginalTeamID:  p.TeamID,
		}
		if p.User != nil {
			view.Type = "user"
			view.Name = getParticipantName(p) // Использует Nickname или FirstName+LastName
			if p.User.LogoKey != nil {
				logoURL := s.uploader.GetPublicURL(*p.User.LogoKey)
				view.LogoURL = &logoURL
			}
		} else if p.Team != nil {
			view.Type = "team"
			view.Name = p.Team.Name
			if p.Team.LogoKey != nil {
				logoURL := s.uploader.GetPublicURL(*p.Team.LogoKey)
				view.LogoURL = &logoURL
			}
		}
		participantsMap[p.ID] = view
	}

	// 5. Загружаем все матчи (соло или командные)
	roundsMap := make(map[int][]*MatchView)

	if tournament.Format.ParticipantType == models.FormatParticipantSolo {
		soloMatches, err := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list solo matches for tournament %d: %w", tournamentID, err)
		}
		for _, sm := range soloMatches {
			if sm == nil {
				continue
			}
			orderInRound := 0 // Нужно определить OrderInRound, если он не хранится. Пока 0.
			// Если bracket_match_uid вида "R1M5", то 5 - это OrderInRound
			if sm.BracketMatchUID != nil {
				parts := strings.SplitN(strings.TrimPrefix(*sm.BracketMatchUID, "R"+strconv.Itoa(*sm.Round)+"M"), "S", 2) // R1M5 -> 5
				if len(parts) > 0 {
					order, _ := strconv.Atoi(parts[0])
					if order > 0 {
						orderInRound = order
					}
				}
			}

			mv := MatchView{
				MatchID:               sm.ID,
				BracketMatchUID:       sm.BracketMatchUID,
				Status:                sm.Status,
				Round:                 *sm.Round, // Предполагаем, что Round не nil для созданных матчей
				OrderInRound:          orderInRound,
				ScoreString:           sm.Score,
				WinnerParticipantDBID: sm.WinnerParticipantID,
				NextMatchDBID:         sm.NextMatchDBID,
				WinnerToSlot:          sm.WinnerToSlot,
				MatchTime:             sm.MatchTime,
			}
			if sm.P1ParticipantID != nil {
				if pView, ok := participantsMap[*sm.P1ParticipantID]; ok {
					mv.Participant1 = &pView
				}
			}
			if sm.P2ParticipantID != nil {
				if pView, ok := participantsMap[*sm.P2ParticipantID]; ok {
					mv.Participant2 = &pView
				}
			}
			// TODO: Распарсить sm.Score в ScoreP1 и ScoreP2, если формат счета это позволяет

			roundNum := *sm.Round
			roundsMap[roundNum] = append(roundsMap[roundNum], &mv)
		}
	} else if tournament.Format.ParticipantType == models.FormatParticipantTeam {
		teamMatches, err := s.teamMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list team matches for tournament %d: %w", tournamentID, err)
		}
		for _, tm := range teamMatches {
			if tm == nil {
				continue
			}
			orderInRound := 0
			if tm.BracketMatchUID != nil {
				parts := strings.SplitN(strings.TrimPrefix(*tm.BracketMatchUID, "R"+strconv.Itoa(*tm.Round)+"M"), "S", 2)
				if len(parts) > 0 {
					order, _ := strconv.Atoi(parts[0])
					if order > 0 {
						orderInRound = order
					}
				}
			}
			mv := MatchView{
				MatchID:               tm.ID,
				BracketMatchUID:       tm.BracketMatchUID,
				Status:                tm.Status,
				Round:                 *tm.Round,
				OrderInRound:          orderInRound,
				ScoreString:           tm.Score,
				WinnerParticipantDBID: tm.WinnerParticipantID,
				NextMatchDBID:         tm.NextMatchDBID,
				WinnerToSlot:          tm.WinnerToSlot,
				MatchTime:             tm.MatchTime,
			}
			if tm.T1ParticipantID != nil {
				if pView, ok := participantsMap[*tm.T1ParticipantID]; ok {
					mv.Participant1 = &pView
				}
			}
			if tm.T2ParticipantID != nil {
				if pView, ok := participantsMap[*tm.T2ParticipantID]; ok {
					mv.Participant2 = &pView
				}
			}
			roundNum := *tm.Round
			roundsMap[roundNum] = append(roundsMap[roundNum], &mv)
		}
	}

	// 6. Формируем список раундов в правильном порядке
	var roundsViewList []RoundView
	roundNumbers := make([]int, 0, len(roundsMap))
	for rNum := range roundsMap {
		roundNumbers = append(roundNumbers, rNum)
	}
	sort.Ints(roundNumbers)

	for _, rNum := range roundNumbers {
		matchesInRound := roundsMap[rNum]
		// Сортируем матчи внутри раунда по OrderInRound (если он был извлечен из UID) или по ID
		sort.Slice(matchesInRound, func(i, j int) bool {
			if matchesInRound[i].OrderInRound != 0 && matchesInRound[j].OrderInRound != 0 {
				return matchesInRound[i].OrderInRound < matchesInRound[j].OrderInRound
			}
			return matchesInRound[i].MatchID < matchesInRound[j].MatchID // Fallback sort by ID
		})
		roundsViewList = append(roundsViewList, RoundView{
			RoundNumber: rNum,
			Matches:     dereferenceMatchViews(matchesInRound),
		})
	}

	var overallWinnerID *int
	// TODO: Если в tournament есть поле для общего победителя, установить overallWinnerID

	return &FullTournamentBracketView{
		TournamentID:               tournament.ID,
		Name:                       tournament.Name,
		Status:                     tournament.Status,
		Sport:                      tournament.Sport,
		Format:                     tournament.Format,
		Rounds:                     roundsViewList,
		ParticipantsMap:            participantsMap,
		OverallWinnerParticipantID: overallWinnerID,
	}, nil
}

// Вспомогательная функция для разыменования слайса указателей на MatchView
func dereferenceMatchViews(slice []*MatchView) []MatchView {
	if slice == nil {
		return []MatchView{}
	}
	result := make([]MatchView, len(slice))
	for i, ptr := range slice {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}

func (s *tournamentService) populateTournamentDetails(ctx context.Context, tournament *models.Tournament) {
	if tournament == nil {
		return
	}
	s.populateTournamentLogoURL(tournament)

	if tournament.Sport == nil && tournament.SportID > 0 {
		sport, err := s.sportRepo.GetByID(ctx, tournament.SportID)
		if err == nil && sport != nil {
			s.populateSportLogoURL(sport)
			tournament.Sport = sport
		} else if err != nil {
			log.Printf("Warning: failed to populate sport %d for tournament %d: %v", tournament.SportID, tournament.ID, err)
		}
	}
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if err == nil && format != nil {
			tournament.Format = format
		} else if err != nil {
			log.Printf("Warning: failed to populate format %d for tournament %d: %v", tournament.FormatID, tournament.ID, err)
		}
	}
	if tournament.Organizer == nil && tournament.OrganizerID > 0 {
		organizer, err := s.userRepo.GetByID(ctx, tournament.OrganizerID)
		if err == nil && organizer != nil {
			s.populateUserDetails(organizer) // Убедитесь, что эта функция есть и работает
			tournament.Organizer = organizer
		} else if err != nil {
			log.Printf("Warning: failed to populate organizer %d for tournament %d: %v", tournament.OrganizerID, tournament.ID, err)
		}
	}
	// Загрузка подтвержденных участников (если нужно для общего отображения Tournament, а не только для сетки)
	// statusConfirmed := models.StatusParticipant
	// participants, _ := s.participantRepo.ListByTournament(ctx, tournament.ID, &statusConfirmed, true)
	// s.populateParticipantListDetails(participants)
	// tournament.Participants = ParticipantsToInterface(participants)
}

func validateTournamentDates(reg, start, end time.Time) error {
	if reg.IsZero() || start.IsZero() || end.IsZero() {
		return ErrTournamentDatesRequired
	}
	if reg.After(start) {
		return fmt.Errorf("%w: registration date (%s) cannot be after start date (%s)", ErrTournamentInvalidRegDate, reg.Format(time.RFC3339), start.Format(time.RFC3339))
	}
	if !start.Before(end) {
		return fmt.Errorf("%w: start date (%s) must be before end date (%s)", ErrTournamentInvalidDateRange, start.Format(time.RFC3339), end.Format(time.RFC3339))
	}
	return nil
}

func isValidTournamentStatus(status models.TournamentStatus) bool {
	switch status {
	case models.StatusSoon, models.StatusRegistration, models.StatusActive, models.StatusCompleted, models.StatusCanceled:
		return true
	}
	return false
}

func isValidStatusTransition(current, next models.TournamentStatus) bool {
	if current == next {
		return true
	}
	allowedTransitions := map[models.TournamentStatus][]models.TournamentStatus{
		models.StatusSoon:         {models.StatusRegistration, models.StatusCanceled},
		models.StatusRegistration: {models.StatusActive, models.StatusCanceled},
		models.StatusActive:       {models.StatusCompleted, models.StatusCanceled},
		models.StatusCompleted:    {}, // No transitions from completed (except maybe to canceled if needed, but not typical)
		models.StatusCanceled:     {}, // No transitions from canceled
	}
	for _, allowedNextStatus := range allowedTransitions[current] {
		if next == allowedNextStatus {
			return true
		}
	}
	return false
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ParticipantsToInterface(slice []*models.Participant) []models.Participant {
	if slice == nil {
		return []models.Participant{} // Return empty slice instead of nil for consistency
	}
	result := make([]models.Participant, len(slice))
	for i, ptr := range slice {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}

func SoloMatchesToInterface(slice []*models.SoloMatch) []models.SoloMatch {
	if slice == nil {
		return []models.SoloMatch{} // Return empty slice
	}
	result := make([]models.SoloMatch, len(slice))
	for i, ptr := range slice {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}

func TeamMatchesToInterface(slice []*models.TeamMatch) []models.TeamMatch {
	if slice == nil {
		return []models.TeamMatch{} // Return empty slice
	}
	result := make([]models.TeamMatch, len(slice))
	for i, ptr := range slice {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}

func (s *tournamentService) populateSportLogoURL(sport *models.Sport) {
	if sport != nil && sport.LogoKey != nil && *sport.LogoKey != "" && s.uploader != nil {
		url := s.uploader.GetPublicURL(*sport.LogoKey)
		if url != "" {
			sport.LogoURL = &url
		}
	}
}

func (s *tournamentService) populateParticipantListDetails(participants []*models.Participant) {
	if s.uploader == nil {
		return
	}
	for _, p := range participants {
		if p == nil {
			continue
		}
		if p.User != nil {
			s.populateUserDetails(p.User)
		}
		if p.Team != nil && p.Team.LogoKey != nil && *p.Team.LogoKey != "" {
			url := s.uploader.GetPublicURL(*p.Team.LogoKey)
			if url != "" {
				p.Team.LogoURL = &url
			}
		}
	}
}
