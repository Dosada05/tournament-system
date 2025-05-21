// File: tournament-system/services/tournament_service.go
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/db" // For advisory lock
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
	OriginalUserID  *int    `json:"original_user_id,omitempty"`
	OriginalTeamID  *int    `json:"original_team_id,omitempty"`
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

type TournamentService interface {
	CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus, exec repositories.SQLExecutor) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int, currentUserID int) error
	UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error)
	FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantID int, currentUserID int) (*models.Tournament, error)
	GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error)
	AutoUpdateTournamentStatusesByDates(ctx context.Context) error
}

type tournamentService struct {
	db              *sql.DB // Added for managing transactions directly
	tournamentRepo  repositories.TournamentRepository
	sportRepo       repositories.SportRepository
	formatRepo      repositories.FormatRepository
	userRepo        repositories.UserRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
	bracketService  BracketService
	matchService    MatchService // Assuming MatchService might be needed
	uploader        storage.FileUploader
	hub             *brackets.Hub
	logger          *slog.Logger // Added logger
}

func NewTournamentService(
	db *sql.DB, // Added db
	tournamentRepo repositories.TournamentRepository,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
	participantRepo repositories.ParticipantRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	bracketService BracketService,
	matchService MatchService,
	uploader storage.FileUploader,
	hub *brackets.Hub,
	logger *slog.Logger, // Added logger
) TournamentService {
	return &tournamentService{
		db:              db,
		tournamentRepo:  tournamentRepo,
		sportRepo:       sportRepo,
		formatRepo:      formatRepo,
		userRepo:        userRepo,
		participantRepo: participantRepo,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
		bracketService:  bracketService,
		matchService:    matchService,
		uploader:        uploader,
		hub:             hub,
		logger:          logger,
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
		Status:          models.StatusSoon, // Initial status
	}

	err = s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentCreationFailed, err)
	}
	s.populateTournamentDetails(ctx, tournament) // Populate details including logo URL if any
	return tournament, nil
}

func (s *tournamentService) GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament by id %d", id)
	}
	s.populateTournamentDetails(ctx, tournament) // Populates Sport, Format, Organizer, and LogoURL

	// Enhanced details loading
	g, gCtx := errgroup.WithContext(ctx)

	// Load confirmed participants
	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		dbParticipants, participantsErr := s.participantRepo.ListByTournament(gCtx, id, &confirmedStatus, true) // true for nested details
		if participantsErr != nil {
			s.logger.WarnContext(gCtx, "Failed to fetch confirmed participants", slog.Int("tournament_id", id), slog.Any("error", participantsErr))
			// Decide if this is a critical error or if we can proceed
		} else {
			populateParticipantListDetailsFunc(dbParticipants, s.uploader) // Using helper
			tournament.Participants = ParticipantsToInterface(dbParticipants)
		}
		return nil // Return nil to not fail the group for this non-critical load
	})

	// Load matches if tournament is active or completed
	if tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted {
		if tournament.Format != nil { // Ensure format is loaded before trying to list matches
			g.Go(func() error {
				if tournament.Format.ParticipantType == models.FormatParticipantSolo {
					soloMatches, soloErr := s.matchService.ListSoloMatchesByTournament(gCtx, id)
					if soloErr != nil {
						s.logger.WarnContext(gCtx, "Failed to fetch solo matches", slog.Int("tournament_id", id), slog.Any("error", soloErr))
					} else {
						tournament.SoloMatches = SoloMatchesToInterface(soloMatches)
					}
				}
				return nil
			})

			g.Go(func() error {
				if tournament.Format.ParticipantType == models.FormatParticipantTeam {
					teamMatches, teamErr := s.matchService.ListTeamMatchesByTournament(gCtx, id)
					if teamErr != nil {
						s.logger.WarnContext(gCtx, "Failed to fetch team matches", slog.Int("tournament_id", id), slog.Any("error", teamErr))
					} else {
						tournament.TeamMatches = TeamMatchesToInterface(teamMatches)
					}
				}
				return nil
			})
		} else {
			s.logger.WarnContext(ctx, "Format not loaded for tournament, cannot fetch matches", slog.Int("tournament_id", id))
		}
	}

	if err := g.Wait(); err != nil {
		// This error will be from a goroutine that returned a non-nil error.
		// We are currently returning nil from participant/match loading goroutines on error,
		// so this path might only be hit by critical errors in the future.
		s.logger.ErrorContext(ctx, "Error during parallel fetching of additional tournament details", slog.Int("tournament_id", id), slog.Any("error", err))
		// Potentially return the partially filled tournament or the error
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
	for i := range tournaments { // Iterate by index to modify the actual element
		s.populateTournamentDetails(ctx, &tournaments[i])
	}
	return tournaments, nil
}

func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for update", id)
	}

	if tournament.OrganizerID != currentUserID { // Add admin check if necessary
		return nil, ErrForbiddenOperation
	}

	// Only allow updates if status is 'soon' or 'registration'
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
	// Date updates require re-validation
	datesChanged := false
	if input.RegDate != nil && !input.RegDate.IsZero() && !input.RegDate.Equal(tournament.RegDate) {
		tournament.RegDate = *input.RegDate
		updated = true
		datesChanged = true
	}
	if input.StartDate != nil && !input.StartDate.IsZero() && !input.StartDate.Equal(tournament.StartDate) {
		tournament.StartDate = *input.StartDate
		updated = true
		datesChanged = true
	}
	if input.EndDate != nil && !input.EndDate.IsZero() && !input.EndDate.Equal(tournament.EndDate) {
		tournament.EndDate = *input.EndDate
		updated = true
		datesChanged = true
	}
	if datesChanged {
		if err := validateTournamentDates(tournament.RegDate, tournament.StartDate, tournament.EndDate); err != nil {
			return nil, err
		}
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

	if !updated {
		s.populateTournamentDetails(ctx, tournament)
		return tournament, nil
	}

	err = s.tournamentRepo.Update(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: error from repository on update", ErrTournamentUpdateFailed)
	}
	s.populateTournamentDetails(ctx, tournament)
	return tournament, nil
}

// UpdateTournamentStatus handles status changes, including bracket generation.
// If exec is nil, it manages its own transaction. Otherwise, it uses the provided SQLExecutor.
func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus, exec repositories.SQLExecutor) (*models.Tournament, error) {
	s.logger.InfoContext(ctx, "UpdateTournamentStatus called", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)), slog.Int("user_id", currentUserID))

	// 1. Fetch tournament details (outside any transaction for now, or use exec if provided for reads too)
	tournament, err := s.tournamentRepo.GetByID(ctx, id) // This GetByID doesn't use SQLExecutor
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateTournamentStatus: failed to get tournament %d", id)
	}

	// 2. Authorization (if currentUserID is not 0, meaning it's not a system call)
	if currentUserID != 0 && tournament.OrganizerID != currentUserID { // Add admin check if needed
		return nil, ErrForbiddenOperation
	}

	// 3. Load format if not already loaded and needed (especially for 'active' transition)
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			s.logger.WarnContext(ctx, "UpdateTournamentStatus: could not load format", slog.Int("tournament_id", id), slog.Int("format_id", tournament.FormatID), slog.Any("error", formatErr))
			// If format is critical for *any* status update logic beyond bracket gen, this might need to be an error
		}
		tournament.Format = format
	}

	// 4. Validate status transition
	currentStatus := tournament.Status
	if !isValidStatusTransition(currentStatus, newStatus) {
		return nil, fmt.Errorf("%w: from '%s' to '%s' for tournament %d", ErrTournamentInvalidStatusTransition, currentStatus, newStatus, id)
	}
	if currentStatus == newStatus { // No change
		s.populateTournamentDetails(ctx, tournament)
		return tournament, nil
	}

	// 5. Transaction management
	var ownTx *sql.Tx
	var opErr error
	executor := exec

	if executor == nil {
		tmpTx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			s.logger.ErrorContext(ctx, "UpdateTournamentStatus: Failed to begin new transaction", slog.Int("tournament_id", id), slog.Any("error", txErr))
			return nil, fmt.Errorf("failed to begin transaction for status update: %w", txErr)
		}
		ownTx = tmpTx
		executor = ownTx
		defer func() {
			if ownTx != nil { // If ownTx is not nil, it means Commit wasn't called or failed
				s.logger.WarnContext(ctx, "Rolling back own transaction in UpdateTournamentStatus", slog.Int("tournament_id", id), slog.Any("operation_error", opErr))
				if rbErr := ownTx.Rollback(); rbErr != nil {
					s.logger.ErrorContext(ctx, "Failed to rollback own transaction", slog.Int("tournament_id", id), slog.Any("rollback_error", rbErr))
				}
			}
		}()
	}

	// 6. Update status in DB
	opErr = s.tournamentRepo.UpdateStatus(ctx, executor, id, newStatus)
	if opErr != nil {
		s.logger.ErrorContext(ctx, "Failed to update tournament status in DB", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)), slog.Any("error", opErr))
		// If using ownTx, defer will handle rollback. If using passed exec, caller handles rollback.
		return nil, fmt.Errorf("UpdateTournamentStatus: failed to update status in repo for tournament %d: %w", id, opErr)
	}
	tournament.Status = newStatus // Reflect change in local model
	s.logger.InfoContext(ctx, "Tournament status updated in DB", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)))

	// 7. Handle side effects of status change (e.g., bracket generation)
	if newStatus == models.StatusActive && currentStatus != models.StatusActive {
		s.logger.InfoContext(ctx, "Tournament became active, attempting bracket generation", slog.Int("tournament_id", id))
		if tournament.Format == nil { // Should have been loaded earlier
			opErr = fmt.Errorf("format is nil, cannot generate bracket for tournament %d", id)
			s.logger.ErrorContext(ctx, opErr.Error(), slog.Int("tournament_id", id))
			return nil, opErr // This will trigger rollback if ownTx
		}

		// Check if matches already exist (idempotency for bracket generation)
		var existingMatches bool
		// This check should ideally also use the executor if reads need to be consistent with the current transaction.
		// For simplicity, if repo methods don't support exec for reads, this might use a separate connection.
		if tournament.Format.ParticipantType == models.FormatParticipantSolo {
			solos, _ := s.soloMatchRepo.ListByTournament(ctx, tournament.ID, nil, nil) // Assumes read doesn't need to be in this tx or repo supports it
			existingMatches = len(solos) > 0
		} else {
			teams, _ := s.teamMatchRepo.ListByTournament(ctx, tournament.ID, nil, nil)
			existingMatches = len(teams) > 0
		}

		if !existingMatches {
			_, bracketErr := s.bracketService.GenerateAndSaveBracket(ctx, executor, tournament)
			if bracketErr != nil {
				opErr = fmt.Errorf("status updated to active, but bracket generation failed: %w", bracketErr)
				s.logger.ErrorContext(ctx, "Bracket generation failed", slog.Int("tournament_id", id), slog.Any("error", opErr))
				return nil, opErr // This will trigger rollback if ownTx
			}
			s.logger.InfoContext(ctx, "Bracket generated successfully", slog.Int("tournament_id", id))
			// Bracket update notification can be sent after commit
		} else {
			s.logger.InfoContext(ctx, "Matches already exist, skipping bracket generation", slog.Int("tournament_id", id))
		}
	}

	// 8. Commit transaction if owned by this function
	if ownTx != nil {
		if errCommit := ownTx.Commit(); errCommit != nil {
			ownTx = nil // Nullify to prevent deferred rollback from trying again
			opErr = fmt.Errorf("failed to commit transaction for status update: %w", errCommit)
			s.logger.ErrorContext(ctx, "Failed to commit own transaction", slog.Int("tournament_id", id), slog.Any("error", opErr))
			return nil, opErr
		}
		ownTx = nil // Mark as committed
		s.logger.InfoContext(ctx, "Own transaction committed successfully", slog.Int("tournament_id", id))

		// Send WebSocket notifications only after successful commit of an owned transaction
		if s.hub != nil {
			roomID := "tournament_" + strconv.Itoa(tournament.ID)
			statusPayload := map[string]interface{}{"tournament_id": tournament.ID, "new_status": newStatus, "old_status": currentStatus}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_STATUS_UPDATED", Payload: statusPayload, RoomID: roomID})

			if newStatus == models.StatusActive && currentStatus != models.StatusActive { // And bracket was generated
				fullBracketData, errData := s.GetTournamentBracketData(ctx, tournament.ID) // Get fresh data post-commit
				if errData == nil {
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "BRACKET_UPDATED", Payload: fullBracketData, RoomID: roomID})
				} else {
					s.logger.WarnContext(ctx, "Failed to get full bracket data for WebSocket broadcast after status update", slog.Int("tournament_id", id), slog.Any("error", errData))
				}
			}
		}
	}
	// If exec was provided, the caller is responsible for commit and notifications.

	s.populateTournamentDetails(ctx, tournament) // Refresh details for the returned object
	return tournament, nil
}

func (s *tournamentService) DeleteTournament(ctx context.Context, id int, currentUserID int) error {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for deletion check", id)
	}

	if tournament.OrganizerID != currentUserID { // Add admin role check if necessary
		return ErrForbiddenOperation
	}

	// Allow deletion only for 'soon', 'registration', or 'canceled' tournaments
	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status '%s'", ErrTournamentDeletionNotAllowed, tournament.Status)
	}

	// Check for dependent entities (participants, matches) before attempting to delete
	// This might involve new repository methods or checks here.
	// For now, relying on DB foreign key constraints to signal ErrTournamentInUse.

	oldLogoKey := tournament.LogoKey
	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentInUse) {
			// This specific error from repo indicates FK violation, meaning it has participants/matches
			return fmt.Errorf("%w: tournament might have participants or matches: %w", ErrTournamentDeletionNotAllowed, err)
		}
		return handleRepositoryError(err, ErrTournamentNotFound, "%w on delete", ErrTournamentDeleteFailed)
	}

	// If deletion was successful, attempt to delete logo from storage
	if oldLogoKey != nil && *oldLogoKey != "" && s.uploader != nil {
		go func(keyToDelete string) { // Asynchronous deletion
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete tournament logo from storage",
					slog.String("key", keyToDelete), slog.Int("tournament_id", id), slog.Any("error", deleteErr))
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

	if tournament.OrganizerID != currentUserID { // Add admin role check if necessary
		return nil, ErrForbiddenOperation
	}
	// Potentially check tournament status to disallow logo changes for active/completed tournaments

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat // Assuming this error is defined in services/errors.go
	}
	if s.uploader == nil {
		return nil, errors.New("file uploader is not configured")
	}

	oldLogoKey := tournament.LogoKey
	ext, errExt := GetExtensionFromContentType(contentType) // Using helper
	if errExt != nil {
		return nil, errExt
	}
	newKey := fmt.Sprintf("%s/%d/logo_%d%s", tournamentLogoPrefix, tournamentID, time.Now().UnixNano(), ext)

	if _, errUpload := s.uploader.Upload(ctx, newKey, contentType, file); errUpload != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrLogoUploadFailed, newKey, errUpload)
	}

	if errDbUpdate := s.tournamentRepo.UpdateLogoKey(ctx, tournamentID, &newKey); errDbUpdate != nil {
		// Attempt to clean up the newly uploaded file if DB update fails
		go func(keyToDelete string) {
			s.logger.InfoContext(context.Background(), "Attempting to delete orphaned logo from storage", slog.String("key", keyToDelete))
			if delErr := s.uploader.Delete(context.Background(), keyToDelete); delErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete orphaned logo", slog.String("key", keyToDelete), slog.Any("error", delErr))
			}
		}(newKey)
		return nil, handleRepositoryError(errDbUpdate, ErrTournamentNotFound, "%w: failed to update logo key in db", ErrLogoUpdateDatabaseFailed)
	}

	// If update was successful and there was an old logo, delete it from storage
	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) { // Asynchronous deletion
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete old tournament logo from storage",
					slog.String("key", keyToDelete), slog.Int("tournament_id", tournamentID), slog.Any("error", deleteErr))
			}
		}(*oldLogoKey)
	}

	tournament.LogoKey = &newKey
	populateTournamentLogoURLFunc(tournament, s.uploader) // Update LogoURL in the model
	return tournament, nil
}

func (s *tournamentService) FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantID int, currentUserID int) (*models.Tournament, error) {
	s.logger.InfoContext(ctx, "FinalizeTournament called", slog.Int("tournament_id", tournamentID), slog.Int("winner_pid", winnerParticipantID), slog.Int("user_id", currentUserID))

	// 1. Fetch tournament (can use GetByID which populates details)
	tournament, err := s.GetTournamentByID(ctx, tournamentID, currentUserID) // Using GetByID which populates .Format
	if err != nil {
		// GetByID already uses handleRepositoryError, so just return if it's ErrTournamentNotFound
		if errors.Is(err, ErrTournamentNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("FinalizeTournament: failed to get tournament %d: %w", tournamentID, err)
	}

	// 2. Authorization
	if currentUserID != 0 && tournament.OrganizerID != currentUserID { // System call if currentUserID is 0
		return nil, ErrForbiddenOperation
	}

	// 3. Status Check
	if tournament.Status == models.StatusCompleted {
		s.logger.InfoContext(ctx, "Tournament already completed", slog.Int("tournament_id", tournamentID))
		// s.populateTournamentDetails(ctx, tournament) // Already populated by GetByID
		return tournament, nil // Return already completed tournament
	}
	if tournament.Status != models.StatusActive {
		return nil, fmt.Errorf("cannot finalize tournament %d with status '%s', expected '%s'",
			tournamentID, tournament.Status, models.StatusActive)
	}

	// 4. Validate Winner Participant
	winnerParticipant, err := s.participantRepo.FindByID(ctx, winnerParticipantID) // FindByID is simpler here
	if err != nil {
		return nil, handleRepositoryError(err, ErrParticipantNotFound, "FinalizeTournament: winner participant ID %d not found", winnerParticipantID)
	}
	if winnerParticipant.TournamentID != tournamentID {
		return nil, fmt.Errorf("winner participant ID %d does not belong to tournament %d", winnerParticipantID, tournamentID)
	}

	// 5. Update Tournament Status to Completed (within a transaction if not already in one)
	// For FinalizeTournament, it's likely called outside the auto-update, so it should manage its own transaction for this.
	// Or, it could be part of the transaction that updated the final match.
	// For now, let UpdateTournamentStatus handle transaction if needed.
	// We pass 0 for currentUserID to signify a system-like call if invoked by match completion logic,
	// or the actual currentUserID if invoked by an admin directly.
	updatedTournament, err := s.UpdateTournamentStatus(ctx, tournamentID, currentUserID, models.StatusCompleted, nil) // nil for exec, manages its own tx
	if err != nil {
		return nil, fmt.Errorf("FinalizeTournament: failed to update tournament status to completed: %w", err)
	}
	// TODO: Persist tournament.OverallWinnerParticipantID = &winnerParticipantID through tournamentRepo.Update
	// This would require adding OverallWinnerParticipantID to the Tournament model and repository update method.
	// For now, we'll just log and include in WebSocket.

	s.logger.InfoContext(ctx, "Tournament finalized", slog.Int("tournament_id", tournamentID), slog.Int("winner_pid", winnerParticipantID))

	// 6. WebSocket Notification (occurs within UpdateTournamentStatus if it commits its own transaction)
	// If UpdateTournamentStatus was called with an external transaction, the caller of FinalizeTournament
	// would be responsible for this notification after its transaction commits.
	// Since UpdateTournamentStatus with nil exec will send its own status update,
	// we might send a more specific TOURNAMENT_COMPLETED here.
	if s.hub != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		// Re-fetch winner participant details if needed, or use what we have
		// For simplicity, assume winnerParticipant is sufficiently detailed or populate it.
		// If participantRepo.FindByID doesn't populate User/Team, we might need GetWithDetails.
		// The populateParticipantListDetailsFunc is for lists, need one for single participant.
		// Let's assume participantToParticipantView can create a view from basic participant.
		winnerView := s.participantToParticipantView(winnerParticipant, s.uploader) // Use existing helper

		completionPayload := map[string]interface{}{
			"tournament_id":            tournamentID,
			"winner_participant_db_id": winnerParticipantID,
			"winner_details":           winnerView,
			// Add overall_winner_participant_id if it was persisted to updatedTournament
		}
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_COMPLETED", Payload: completionPayload, RoomID: roomID})
		s.logger.InfoContext(ctx, "Sent TOURNAMENT_COMPLETED to room", slog.String("room_id", roomID), slog.Int("winner_pid", winnerParticipantID))
	}

	// s.populateTournamentDetails(ctx, updatedTournament) // Already done by GetByID and potentially UpdateTournamentStatus
	return updatedTournament, nil
}

func (s *tournamentService) AutoUpdateTournamentStatusesByDates(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Scheduler: Starting automatic tournament status update cycle.")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Scheduler: failed to begin transaction", slog.Any("error", err))
		return fmt.Errorf("scheduler: failed to begin transaction: %w", err)
	}

	var opErr error
	defer func() {
		if p := recover(); p != nil {
			s.logger.ErrorContext(ctx, "Scheduler: recovered from panic, rolling back transaction", slog.Any("panic_value", p))
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to rollback transaction after panic", slog.Any("rollback_error", rbErr))
			}
			panic(p) // Re-panic after logging and attempting rollback
		} else if opErr != nil {
			s.logger.ErrorContext(ctx, "Scheduler: rolling back transaction due to error", slog.Any("error", opErr))
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to rollback transaction", slog.Any("rollback_error", rbErr))
			}
		} else {
			if cErr := tx.Commit(); cErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to commit transaction", slog.Any("error", cErr))
				opErr = fmt.Errorf("scheduler: failed to commit transaction: %w", cErr)
				// If commit fails, the lock is typically released by Postgres anyway.
			} else {
				s.logger.InfoContext(ctx, "Scheduler: Transaction committed successfully for status updates.")
			}
		}
	}()

	acquiredLock, lockErr := db.TryAcquireTransactionalLock(ctx, tx, db.SchedulerAdvisoryLockID, s.logger)
	if lockErr != nil {
		opErr = fmt.Errorf("scheduler: error trying to acquire lock: %w", lockErr)
		return opErr // This will trigger rollback via defer
	}

	if !acquiredLock {
		s.logger.InfoContext(ctx, "Scheduler: Could not acquire lock, another instance is likely working. Skipping cycle.")
		// No opErr, so the empty transaction will be committed (or rolled back if commit fails for other reasons).
		// This is fine as no work was done.
		return nil
	}
	s.logger.InfoContext(ctx, "Scheduler: Lock acquired. Proceeding with status updates.")

	currentTime := time.Now()
	tournamentsToUpdate, err := s.tournamentRepo.GetTournamentsForAutoStatusUpdate(ctx, tx, currentTime)
	if err != nil {
		opErr = fmt.Errorf("scheduler: failed to get tournaments for status update: %w", err)
		return opErr // This will trigger rollback
	}

	if len(tournamentsToUpdate) == 0 {
		s.logger.InfoContext(ctx, "Scheduler: No tournaments found requiring status update based on dates.")
		return nil // No opErr, transaction for lock acquisition will commit/rollback.
	}

	s.logger.InfoContext(ctx, "Scheduler: Found tournaments to potentially update.", slog.Int("count", len(tournamentsToUpdate)))

	updatedCount := 0
	for _, t := range tournamentsToUpdate {
		originalStatus := t.Status
		var newStatus models.TournamentStatus

		// Determine the new status based on current time and tournament dates
		if currentTime.Before(t.RegDate) { // Should not happen if GetTournamentsForAutoStatusUpdate is correct
			newStatus = models.StatusSoon
		} else if currentTime.Before(t.StartDate) { // Registration period
			newStatus = models.StatusRegistration
		} else if currentTime.Before(t.EndDate) { // Active period
			newStatus = models.StatusActive
		} else { // Tournament period has ended
			// If no winner is set (e.g., OverallWinnerParticipantID is nil in 't'),
			// we might not automatically move to 'completed'. This needs careful consideration.
			// For now, let's assume if EndDate is passed, we try to move to 'completed'.
			// FinalizeTournament should be the one to set the winner.
			// The scheduler could move it to a "pending_completion" or similar state,
			// or this automatic transition to "completed" assumes all matches are done.
			// For now, the query in GetTournamentsForAutoStatusUpdate handles this by exclusion.
			// If a tournament end_date has passed AND it was 'active', it's a candidate.
			newStatus = models.StatusCompleted
		}

		if newStatus != originalStatus {
			s.logger.InfoContext(ctx, "Scheduler: Attempting to update tournament status",
				slog.Int("tournament_id", t.ID), slog.String("name", t.Name),
				slog.String("old_status", string(originalStatus)), slog.String("new_status", string(newStatus)))

			// Call UpdateTournamentStatus with the current transaction (tx)
			// Pass 0 for currentUserID to indicate a system-initiated change.
			updatedTournament, updateErr := s.UpdateTournamentStatus(ctx, t.ID, 0, newStatus, tx)
			if updateErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: Error from UpdateTournamentStatus for tournament",
					slog.Int("tournament_id", t.ID), slog.Any("error", updateErr))
				opErr = fmt.Errorf("scheduler: error processing tournament %d (current status %s) to %s: %w", t.ID, originalStatus, newStatus, updateErr)
				return opErr // Critical error, rollback entire batch
			}

			if updatedTournament.Status == newStatus {
				s.logger.InfoContext(ctx, "Scheduler: Successfully updated tournament status",
					slog.Int("tournament_id", t.ID), slog.String("new_status", string(updatedTournament.Status)))
				updatedCount++
				// WebSocket notifications for status updates and bracket generation (if applicable)
				// are handled within UpdateTournamentStatus when it commits its *own* transaction.
				// Since we are passing `tx`, UpdateTournamentStatus will *not* commit and *not* send notifications.
				// Notifications should be sent after the main scheduler transaction commits.
				// This is complex. A simpler approach might be for UpdateTournamentStatus to always send notifications,
				// but that's not ideal for batch operations.
				// For now, we rely on the fact that UpdateTournamentStatus *does not* send if exec is provided.
				// We could collect all updated tournaments and send notifications after the commit.
			} else {
				// This case should ideally not happen if UpdateTournamentStatus is robust
				s.logger.WarnContext(ctx, "Scheduler: Tournament status was not updated as expected by UpdateTournamentStatus call",
					slog.Int("tournament_id", t.ID),
					slog.String("expected_status", string(newStatus)),
					slog.String("actual_status", string(updatedTournament.Status)))
			}
		}
	}

	s.logger.InfoContext(ctx, "Scheduler: Processed tournaments for status update.", slog.Int("eligible_count", len(tournamentsToUpdate)), slog.Int("updated_count", updatedCount))
	// opErr remains nil if all updates were successful
	return opErr
}

// Helper to populate all standard details for a tournament model
func (s *tournamentService) populateTournamentDetails(ctx context.Context, tournament *models.Tournament) {
	if tournament == nil {
		return
	}
	// Populate Logo URL first
	populateTournamentLogoURLFunc(tournament, s.uploader) // Using helper

	// Populate related entities if their IDs are present and entities are not already populated
	if tournament.Sport == nil && tournament.SportID > 0 {
		sport, err := s.sportRepo.GetByID(ctx, tournament.SportID)
		if err == nil && sport != nil {
			populateSportLogoURLFunc(sport, s.uploader) // Using helper
			tournament.Sport = sport
		} else if err != nil {
			s.logger.WarnContext(ctx, "Failed to populate sport details", slog.Int("tournament_id", tournament.ID), slog.Int("sport_id", tournament.SportID), slog.Any("error", err))
		}
	}

	if tournament.Format == nil && tournament.FormatID > 0 {
		format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if err == nil && format != nil {
			tournament.Format = format
		} else if err != nil {
			s.logger.WarnContext(ctx, "Failed to populate format details", slog.Int("tournament_id", tournament.ID), slog.Int("format_id", tournament.FormatID), slog.Any("error", err))
		}
	}

	if tournament.Organizer == nil && tournament.OrganizerID > 0 {
		organizer, err := s.userRepo.GetByID(ctx, tournament.OrganizerID)
		if err == nil && organizer != nil {
			populateUserDetailsFunc(organizer, s.uploader) // Using helper
			tournament.Organizer = organizer
		} else if err != nil {
			s.logger.WarnContext(ctx, "Failed to populate organizer details", slog.Int("tournament_id", tournament.ID), slog.Int("organizer_id", tournament.OrganizerID), slog.Any("error", err))
		}
	}
}

// GetTournamentBracketData retrieves all necessary data to display a tournament bracket.
func (s *tournamentService) GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error) {
	s.logger.InfoContext(ctx, "GetTournamentBracketData: fetching data for tournament", slog.Int("tournament_id", tournamentID))

	// 1. Load base tournament data and its direct relations (Sport, Format, Organizer)
	tournament, err := s.GetTournamentByID(ctx, tournamentID, 0) // currentUserID 0 as it's for data fetching
	if err != nil {
		// GetTournamentByID already handles ErrTournamentNotFound mapping
		return nil, fmt.Errorf("GetTournamentBracketData: failed to get tournament %d: %w", tournamentID, err)
	}

	if tournament.Format == nil { // Format is crucial for bracket display
		return nil, fmt.Errorf("format information is missing or could not be loaded for tournament %d", tournamentID)
	}

	// 2. Load all confirmed participants with their details (User/Team and their logos)
	statusConfirmed := models.StatusParticipant
	// ListByTournament in repo now has includeNested flag, true loads User/Team
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournamentID, &statusConfirmed, true)
	if err != nil {
		s.logger.WarnContext(ctx, "GetTournamentBracketData: failed to list participants for tournament",
			slog.Int("tournament_id", tournamentID), slog.Any("error", err))
		// Decide if this is critical or can proceed with empty/partial participant map
	}
	// Populate logo URLs for participants (User/Team logos)
	populateParticipantListDetailsFunc(dbParticipants, s.uploader) // Using helper from services/helpers.go

	participantsMap := make(map[int]ParticipantView)
	for _, p := range dbParticipants {
		if p == nil {
			continue
		}
		// participantToParticipantViewFunc needs the uploader
		participantsMap[p.ID] = participantToParticipantViewFunc(p, s.uploader) // Using helper
	}

	// 3. Load all matches (solo or team) for the tournament
	var roundsViewList []RoundView
	roundsMap := make(map[int][]*MatchView)

	if tournament.Format.ParticipantType == models.FormatParticipantSolo {
		soloMatches, listErr := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if listErr != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list solo matches for tournament %d: %w", tournamentID, listErr)
		}
		for _, sm := range soloMatches {
			if sm == nil {
				continue
			}
			// toMatchViewFunc needs logger
			mv := toMatchViewFunc(sm, nil, participantsMap, s.logger) // Using helper
			if sm.Round == nil {
				s.logger.WarnContext(ctx, "Solo match with nil round found, skipping", slog.Int("match_id", sm.ID), slog.Int("tournament_id", tournamentID))
				continue
			}
			roundNum := *sm.Round
			roundsMap[roundNum] = append(roundsMap[roundNum], &mv)
		}
	} else if tournament.Format.ParticipantType == models.FormatParticipantTeam {
		teamMatches, listErr := s.teamMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if listErr != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list team matches for tournament %d: %w", tournamentID, listErr)
		}
		for _, tm := range teamMatches {
			if tm == nil {
				continue
			}
			mv := toMatchViewFunc(nil, tm, participantsMap, s.logger) // Using helper
			if tm.Round == nil {
				s.logger.WarnContext(ctx, "Team match with nil round found, skipping", slog.Int("match_id", tm.ID), slog.Int("tournament_id", tournamentID))
				continue
			}
			roundNum := *tm.Round
			roundsMap[roundNum] = append(roundsMap[roundNum], &mv)
		}
	}

	// 4. Sort rounds and matches within rounds
	roundNumbers := make([]int, 0, len(roundsMap))
	for rNum := range roundsMap {
		roundNumbers = append(roundNumbers, rNum)
	}
	sort.Ints(roundNumbers)

	for _, rNum := range roundNumbers {
		matchesInRound := roundsMap[rNum]
		// Sort matches by OrderInRound primarily, then by MatchID for stable sort
		sort.Slice(matchesInRound, func(i, j int) bool {
			if matchesInRound[i].OrderInRound != 0 && matchesInRound[j].OrderInRound != 0 {
				if matchesInRound[i].OrderInRound != matchesInRound[j].OrderInRound {
					return matchesInRound[i].OrderInRound < matchesInRound[j].OrderInRound
				}
			}
			// Fallback or primary sort if OrderInRound is not set or equal
			return matchesInRound[i].MatchID < matchesInRound[j].MatchID
		})
		roundsViewList = append(roundsViewList, RoundView{
			RoundNumber: rNum,
			Matches:     dereferenceMatchViews(matchesInRound), // Using helper
		})
	}

	// 5. Determine overall winner if tournament is completed
	var overallWinnerParticipantID *int
	// if tournament.Status == models.StatusCompleted {
	// This logic might be more complex, e.g. finding the winner of the last match.
	// Or, if Tournament model gets an OverallWinnerParticipantID field, use that.
	// For now, this remains illustrative.
	// }

	return &FullTournamentBracketView{
		TournamentID:               tournament.ID,
		Name:                       tournament.Name,
		Status:                     tournament.Status,
		Sport:                      tournament.Sport,  // Already populated by GetTournamentByID
		Format:                     tournament.Format, // Already populated by GetTournamentByID
		Rounds:                     roundsViewList,
		ParticipantsMap:            participantsMap,
		OverallWinnerParticipantID: overallWinnerParticipantID,
	}, nil
}

// --- Helper Functions (already in services/helpers.go or similar, or to be placed there) ---
// validateTournamentDates, isValidStatusTransition,
// populateTournamentLogoURLFunc, populateSportLogoURLFunc, populateUserDetailsFunc, populateParticipantListDetailsFunc
// getParticipantDisplayNameFunc, participantToParticipantViewFunc, toMatchViewFunc, dereferenceMatchViews
// These are assumed to be available from services/helpers.go or defined within the service if specific.
// For brevity, their definitions are not repeated here but their usage is shown.

// getParticipantDisplayName is a local helper or can be moved to helpers.go
func (s *tournamentService) getParticipantDisplayName(p *models.Participant) string {
	return getParticipantDisplayNameFunc(p) // Delegate to common helper
}

// participantToParticipantView is a local helper or can be moved to helpers.go
func (s *tournamentService) participantToParticipantView(p *models.Participant, uploader storage.FileUploader) ParticipantView {
	return participantToParticipantViewFunc(p, uploader) // Delegate to common helper
}

// toMatchView is a local helper or can be moved to helpers.go
func (s *tournamentService) toMatchView(sm *models.SoloMatch, tm *models.TeamMatch, participantsMap map[int]ParticipantView) MatchView {
	return toMatchViewFunc(sm, tm, participantsMap, s.logger) // Delegate to common helper
}
