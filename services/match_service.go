package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

// Определения ошибок на уровне пакета
var (
	ErrMatchUpdateForbidden          = errors.New("user not allowed to update this match")
	ErrMatchInvalidWinner            = errors.New("winner participant ID does not match any of the match participants")
	ErrMatchAlreadyCompleted         = errors.New("match is already completed or canceled")
	ErrMatchCannotDetermineNext      = errors.New("cannot determine next match for winner advancement")
	ErrNextMatchParticipantSetFailed = errors.New("failed to set participant in the next match")
	ErrTournamentFinalized           = errors.New("tournament is already finalized, no more match updates allowed")
	ErrSoloMatchNotFound             = repositories.ErrSoloMatchNotFound
	ErrTeamMatchNotFound             = repositories.ErrTeamMatchNotFound
	ErrScoreParsingFailed            = errors.New("failed to parse score string")
)

type UpdateMatchResultInput struct {
	Score               *string `json:"score" validate:"omitempty,max=50"` // e.g., "3-1", "2-2"
	WinnerParticipantID *int    `json:"winner_participant_id,omitempty"`   // Pointer to allow nil for draws
}

type MatchService interface {
	ListSoloMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.SoloMatch, error)
	ListTeamMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.TeamMatch, error)
	UpdateSoloMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.SoloMatch, error)
	UpdateTeamMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.TeamMatch, error)
}

type matchService struct {
	db              *sql.DB
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
	tournamentRepo  repositories.TournamentRepository
	participantRepo repositories.ParticipantRepository
	formatRepo      repositories.FormatRepository             // Added
	standingRepo    repositories.TournamentStandingRepository // Added
	hub             *brackets.Hub
	logger          *slog.Logger // Added
}

func NewMatchService(
	db *sql.DB,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	tournamentRepo repositories.TournamentRepository,
	participantRepo repositories.ParticipantRepository,
	formatRepo repositories.FormatRepository, // Added
	standingRepo repositories.TournamentStandingRepository, // Added
	hub *brackets.Hub,
	logger *slog.Logger, // Added
) MatchService {
	return &matchService{
		db:              db,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
		tournamentRepo:  tournamentRepo,
		participantRepo: participantRepo,
		formatRepo:      formatRepo,   // Added
		standingRepo:    standingRepo, // Added
		hub:             hub,
		logger:          logger, // Added
	}
}

func (s *matchService) withTransaction(ctx context.Context, fn func(tx repositories.SQLExecutor) error) error {
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to begin transaction", slog.Any("error", err))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var opErr error
	defer func() {
		if p := recover(); p != nil {
			s.logger.ErrorContext(ctx, "Recovered from panic in transaction", slog.Any("panic_value", p))
			_ = dbTx.Rollback()
			panic(p)
		} else if opErr != nil {
			s.logger.ErrorContext(ctx, "Transaction error, rolling back", slog.Any("operation_error", opErr))
			if rbErr := dbTx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "Transaction rollback failed", slog.Any("rollback_error", rbErr))
			}
		} else {
			if cErr := dbTx.Commit(); cErr != nil {
				s.logger.ErrorContext(ctx, "Failed to commit transaction", slog.Any("commit_error", cErr))
				opErr = fmt.Errorf("failed to commit transaction: %w", cErr)
			} else {
				s.logger.DebugContext(ctx, "Transaction committed successfully")
			}
		}
	}()
	opErr = fn(dbTx)
	return opErr
}

// parseScore expects a score string like "S1-S2" (e.g., "3-1", "0-0").
// It returns scoreP1, scoreP2, isDraw, error.
func parseScore(scoreStr string) (int, int, bool, error) {
	if scoreStr == "" {
		return 0, 0, false, fmt.Errorf("%w: score string is empty", ErrScoreParsingFailed)
	}
	parts := strings.Split(scoreStr, "-")
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("%w: score string '%s' must be in 'S1-S2' format", ErrScoreParsingFailed, scoreStr)
	}
	s1, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	s2, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false, fmt.Errorf("%w: could not parse scores from '%s': %v, %v", ErrScoreParsingFailed, scoreStr, err1, err2)
	}
	if s1 < 0 || s2 < 0 {
		return 0, 0, false, fmt.Errorf("%w: scores cannot be negative in '%s'", ErrScoreParsingFailed, scoreStr)
	}
	return s1, s2, s1 == s2, nil
}

func (s *matchService) ListSoloMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.SoloMatch, error) {
	matches, err := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list solo matches", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
		return nil, fmt.Errorf("failed to list solo matches for tournament %d: %w", tournamentID, err)
	}
	if matches == nil {
		return []*models.SoloMatch{}, nil
	}
	return matches, nil
}

func (s *matchService) ListTeamMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.TeamMatch, error) {
	matches, err := s.teamMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list team matches", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
		return nil, fmt.Errorf("failed to list team matches for tournament %d: %w", tournamentID, err)
	}
	if matches == nil {
		return []*models.TeamMatch{}, nil
	}
	return matches, nil
}

func (s *matchService) UpdateSoloMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.SoloMatch, error) {
	s.logger.InfoContext(ctx, "UpdateSoloMatchResult: Attempting to update", slog.Int("match_id", matchID), slog.Int("tournament_id", tournamentID))
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateSoloMatchResult: failed to get tournament %d", tournamentID)
	}
	// Load format for tournament type check
	if tournament.Format == nil {
		format, ferr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if ferr != nil {
			return nil, handleRepositoryError(ferr, ErrFormatNotFound, "UpdateSoloMatchResult: failed to load format %d for tournament %d", tournament.FormatID, tournamentID)
		}
		tournament.Format = format
	}

	if tournament.OrganizerID != currentUserID {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Forbidden", slog.Int("organizer_id", tournament.OrganizerID), slog.Int("current_user_id", currentUserID))
		return nil, ErrMatchUpdateForbidden
	}
	if tournament.Status != models.StatusActive {
		if tournament.Status == models.StatusCompleted || tournament.Status == models.StatusCanceled {
			return nil, fmt.Errorf("%w: tournament status is '%s'", ErrTournamentFinalized, tournament.Status)
		}
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Tournament not active", slog.String("status", string(tournament.Status)))
		return nil, fmt.Errorf("cannot update match result for tournament with status '%s'", tournament.Status)
	}

	currentMatch, err := s.soloMatchRepo.GetByID(ctx, matchID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrSoloMatchNotFound, "UpdateSoloMatchResult: failed to get solo match %d", matchID)
	}

	if currentMatch.TournamentID != tournamentID {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Match does not belong to tournament", slog.Int("match_id", matchID), slog.Int("actual_tournament_id", currentMatch.TournamentID))
		return nil, fmt.Errorf("match %d does not belong to tournament %d", matchID, tournamentID)
	}
	if currentMatch.Status == models.MatchStatusCompleted || currentMatch.Status == models.MatchStatusCanceled {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Match already completed/canceled", slog.String("status", string(currentMatch.Status)))
		return nil, ErrMatchAlreadyCompleted
	}
	if currentMatch.P1ParticipantID == nil || currentMatch.P2ParticipantID == nil {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Match not ready, participants not set")
		return nil, fmt.Errorf("match %d is not ready, participants not set (P1: %v, P2: %v)", matchID, currentMatch.P1ParticipantID, currentMatch.P2ParticipantID)
	}

	// Score and Winner validation based on format type
	isDraw := false
	var scoreP1, scoreP2 int
	if input.Score != nil && *input.Score != "" {
		var parseErr error
		scoreP1, scoreP2, isDraw, parseErr = parseScore(*input.Score)
		if parseErr != nil {
			s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Score parsing failed", slog.String("score_string", *input.Score), slog.Any("error", parseErr))
			return nil, parseErr
		}
	} else {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Score string is missing")
		return nil, fmt.Errorf("%w: score string is required", ErrScoreParsingFailed)
	}

	// Validate WinnerParticipantID against participants and draw status
	if isDraw {
		if input.WinnerParticipantID != nil {
			s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Winner specified for a draw", slog.Any("winner_id", input.WinnerParticipantID))
			return nil, fmt.Errorf("cannot specify winner for a draw match (score: %s)", *input.Score)
		}
	} else {
		if input.WinnerParticipantID == nil {
			s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Winner not specified for a non-draw match")
			return nil, errors.New("winner must be specified if the match is not a draw")
		}
		if *input.WinnerParticipantID != *currentMatch.P1ParticipantID && *input.WinnerParticipantID != *currentMatch.P2ParticipantID {
			s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Invalid winner ID",
				slog.Any("winner_id", input.WinnerParticipantID),
				slog.Any("p1_id", currentMatch.P1ParticipantID),
				slog.Any("p2_id", currentMatch.P2ParticipantID))
			return nil, fmt.Errorf("%w: winner ID %d is not P1 (%d) or P2 (%d) of match %d",
				ErrMatchInvalidWinner, *input.WinnerParticipantID, *currentMatch.P1ParticipantID, *currentMatch.P2ParticipantID, matchID)
		}
	}

	var updatedMatch *models.SoloMatch
	var nextMatchToNotify *models.SoloMatch
	isFinalMatchForBranch := currentMatch.NextMatchDBID == nil

	opErr := s.withTransaction(ctx, func(tx repositories.SQLExecutor) error {
		var txInternalErr error
		txInternalErr = s.soloMatchRepo.UpdateScoreStatusWinner(ctx, tx, matchID, input.Score, models.MatchStatusCompleted, input.WinnerParticipantID)
		if txInternalErr != nil {
			s.logger.ErrorContext(ctx, "UpdateSoloMatchResult: Failed to update match in DB", slog.Int("match_id", matchID), slog.Any("error", txInternalErr))
			return fmt.Errorf("failed to update current solo match %d in transaction: %w", matchID, txInternalErr)
		}
		s.logger.InfoContext(ctx, "UpdateSoloMatchResult: Match updated in DB", slog.Int("match_id", matchID), slog.Any("winner_id", input.WinnerParticipantID), slog.Any("score", input.Score))

		if tournament.Format.BracketType == "RoundRobin" {
			// Update standings
			p1Stand, errGetP1 := s.standingRepo.GetOrCreate(ctx, tx, tournamentID, *currentMatch.P1ParticipantID)
			if errGetP1 != nil {
				return fmt.Errorf("failed to get/create standing for P1 (%d): %w", *currentMatch.P1ParticipantID, errGetP1)
			}
			p2Stand, errGetP2 := s.standingRepo.GetOrCreate(ctx, tx, tournamentID, *currentMatch.P2ParticipantID)
			if errGetP2 != nil {
				return fmt.Errorf("failed to get/create standing for P2 (%d): %w", *currentMatch.P2ParticipantID, errGetP2)
			}

			p1Stand.GamesPlayed++
			p2Stand.GamesPlayed++
			p1Stand.ScoreFor += scoreP1
			p1Stand.ScoreAgainst += scoreP2
			p2Stand.ScoreFor += scoreP2
			p2Stand.ScoreAgainst += scoreP1
			p1Stand.ScoreDifference = p1Stand.ScoreFor - p1Stand.ScoreAgainst
			p2Stand.ScoreDifference = p2Stand.ScoreFor - p2Stand.ScoreAgainst

			if isDraw {
				p1Stand.Points++
				p1Stand.Draws++
				p2Stand.Points++
				p2Stand.Draws++
			} else {
				if input.WinnerParticipantID != nil {
					if *input.WinnerParticipantID == *currentMatch.P1ParticipantID {
						p1Stand.Points += 3
						p1Stand.Wins++
						p2Stand.Losses++
					} else { // Winner is P2
						p2Stand.Points += 3
						p2Stand.Wins++
						p1Stand.Losses++
					}
				}
			}
			if err := s.standingRepo.Update(ctx, tx, p1Stand); err != nil {
				return fmt.Errorf("failed to update standing for P1 (%d): %w", p1Stand.ParticipantID, err)
			}
			if err := s.standingRepo.Update(ctx, tx, p2Stand); err != nil {
				return fmt.Errorf("failed to update standing for P2 (%d): %w", p2Stand.ParticipantID, err)
			}
			s.logger.InfoContext(ctx, "UpdateSoloMatchResult: Standings updated for RoundRobin", slog.Int("match_id", matchID))

		} else if tournament.Format.BracketType == "SingleElimination" {
			// Existing logic for Single Elimination advancement
			if currentMatch.NextMatchDBID != nil && currentMatch.WinnerToSlot != nil && input.WinnerParticipantID != nil {
				nextMatchID := *currentMatch.NextMatchDBID
				slotToPlaceWinner := *currentMatch.WinnerToSlot
				nextMatchForUpdate, errGetNext := s.soloMatchRepo.GetByID(ctx, nextMatchID)
				if errGetNext != nil {
					s.logger.ErrorContext(ctx, "UpdateSoloMatchResult: Next match not found", slog.Int("next_match_id", nextMatchID), slog.Any("error", errGetNext))
					return fmt.Errorf("%w: next match ID %d not found", ErrMatchCannotDetermineNext, nextMatchID)
				}

				if nextMatchForUpdate.Status == models.MatchStatusCompleted || nextMatchForUpdate.Status == models.MatchStatusCanceled {
					s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Next match already completed/canceled", slog.Int("next_match_id", nextMatchID))
				} else {
					var p1, p2 *int
					if slotToPlaceWinner == 1 {
						p1 = input.WinnerParticipantID
						p2 = nextMatchForUpdate.P2ParticipantID
					} else if slotToPlaceWinner == 2 {
						p1 = nextMatchForUpdate.P1ParticipantID
						p2 = input.WinnerParticipantID
					} else {
						return fmt.Errorf("invalid winner_to_slot value %d for match %d", slotToPlaceWinner, matchID)
					}
					txInternalErr = s.soloMatchRepo.UpdateParticipants(ctx, tx, nextMatchID, p1, p2)
					if txInternalErr != nil {
						return fmt.Errorf("%w for next match %d: %w", ErrNextMatchParticipantSetFailed, nextMatchID, txInternalErr)
					}
					s.logger.InfoContext(ctx, "UpdateSoloMatchResult: Winner advanced in SingleElimination", slog.Int("match_id", matchID), slog.Int("next_match_id", nextMatchID))
				}
			}
		}
		return nil
	})

	if opErr != nil {
		return nil, opErr
	}

	// Fetch updated match details for notifications
	var fetchErr error
	updatedMatch, fetchErr = s.soloMatchRepo.GetByID(ctx, matchID)
	if fetchErr != nil {
		s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Failed to fetch updated match post-transaction", slog.Int("match_id", matchID), slog.Any("error", fetchErr))
		// Not returning error here, proceed with notifications if possible
	}

	if tournament.Format.BracketType == "SingleElimination" && currentMatch.NextMatchDBID != nil {
		nextMatchToNotify, fetchErr = s.soloMatchRepo.GetByID(ctx, *currentMatch.NextMatchDBID)
		if fetchErr != nil {
			s.logger.WarnContext(ctx, "UpdateSoloMatchResult: Failed to fetch next match post-transaction", slog.Any("next_match_id", currentMatch.NextMatchDBID), slog.Any("error", fetchErr))
			nextMatchToNotify = nil
		}
	}

	// WebSocket Notifications
	if s.hub != nil && updatedMatch != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: updatedMatch, RoomID: roomID})
		s.logger.InfoContext(ctx, "Sent MATCH_UPDATED", slog.Int("match_id", updatedMatch.ID), slog.String("room_id", roomID))

		if tournament.Format.BracketType == "RoundRobin" {
			standings, listErr := s.standingRepo.ListByTournament(ctx, s.db, tournamentID, true) // Use main db for read
			if listErr == nil {
				// Ideally, transform standings to a view model before sending
				s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "STANDINGS_UPDATED", Payload: standings, RoomID: roomID})
				s.logger.InfoContext(ctx, "Sent STANDINGS_UPDATED", slog.Int("tournament_id", tournamentID), slog.String("room_id", roomID))
			} else {
				s.logger.ErrorContext(ctx, "Failed to list standings for WebSocket broadcast", slog.Int("tournament_id", tournamentID), slog.Any("error", listErr))
			}
		} else if tournament.Format.BracketType == "SingleElimination" {
			if updatedMatch.NextMatchDBID != nil && updatedMatch.WinnerToSlot != nil && updatedMatch.WinnerParticipantID != nil {
				advPayload := map[string]interface{}{"advancing_participant_db_id": *updatedMatch.WinnerParticipantID, "source_match_id": updatedMatch.ID, "next_match_id": *updatedMatch.NextMatchDBID, "next_match_slot": *updatedMatch.WinnerToSlot, "tournament_id": tournamentID}
				s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "PARTICIPANT_ADVANCED", Payload: advPayload, RoomID: roomID})
				s.logger.InfoContext(ctx, "Sent PARTICIPANT_ADVANCED", slog.Int("match_id", updatedMatch.ID))
				if nextMatchToNotify != nil && (nextMatchToNotify.P1ParticipantID != nil && nextMatchToNotify.P2ParticipantID != nil) && nextMatchToNotify.Status == models.StatusScheduled {
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: nextMatchToNotify, RoomID: roomID})
					s.logger.InfoContext(ctx, "Sent MATCH_UPDATED for next match", slog.Int("next_match_id", nextMatchToNotify.ID))
				}
			}
		}

		// Check for tournament final match completion (applies to both SE and could apply to RR if there's a final deciding match)
		// For RR, tournament finalization will typically be by overall standings.
		// For SE, isFinalMatchForBranch checks if it's the end of a branch.
		// A true "tournament final match" needs a clearer definition, perhaps if it's the only match in the last round.
		// For now, this logic is more geared towards SE.
		if isFinalMatchForBranch && updatedMatch.WinnerParticipantID != nil && tournament.Format.BracketType == "SingleElimination" {
			// Check if this is THE final match of the tournament
			// This might require querying how many matches are in the highest round, or if this match has no further next_match_db_id set by any other match.
			// For simplicity, we assume if NextMatchDBID is nil, it's a candidate for final match.
			// The actual finalization decision is better handled by TournamentService.FinalizeTournament.
			finalMatchPayload := map[string]interface{}{"match_id": updatedMatch.ID, "tournament_id": tournamentID, "winner_participant_id": *updatedMatch.WinnerParticipantID, "is_tournament_final_match": true /* This flag needs more robust logic */}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_FINAL_MATCH_COMPLETED", Payload: finalMatchPayload, RoomID: roomID})
			s.logger.InfoContext(ctx, "Sent TOURNAMENT_FINAL_MATCH_COMPLETED (candidate)", slog.Int("match_id", updatedMatch.ID))
		}
	}
	return updatedMatch, nil
}

// UpdateTeamMatchResult is analogous to UpdateSoloMatchResult but for team matches.
// The logic for RoundRobin standings update and SingleElimination advancement will be similar.
func (s *matchService) UpdateTeamMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.TeamMatch, error) {
	s.logger.InfoContext(ctx, "UpdateTeamMatchResult: Attempting to update", slog.Int("match_id", matchID), slog.Int("tournament_id", tournamentID))
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateTeamMatchResult: failed to get tournament %d", tournamentID)
	}
	if tournament.Format == nil {
		format, ferr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if ferr != nil {
			return nil, handleRepositoryError(ferr, ErrFormatNotFound, "UpdateTeamMatchResult: failed to load format %d for tournament %d", tournament.FormatID, tournamentID)
		}
		tournament.Format = format
	}

	if tournament.OrganizerID != currentUserID {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Forbidden", slog.Int("organizer_id", tournament.OrganizerID), slog.Int("current_user_id", currentUserID))
		return nil, ErrMatchUpdateForbidden
	}
	if tournament.Status != models.StatusActive {
		if tournament.Status == models.StatusCompleted || tournament.Status == models.StatusCanceled {
			return nil, fmt.Errorf("%w: tournament status is '%s'", ErrTournamentFinalized, tournament.Status)
		}
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Tournament not active", slog.String("status", string(tournament.Status)))
		return nil, fmt.Errorf("cannot update match result for tournament with status '%s'", tournament.Status)
	}

	currentMatch, err := s.teamMatchRepo.GetByID(ctx, matchID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTeamMatchNotFound, "UpdateTeamMatchResult: failed to get team match %d", matchID)
	}

	if currentMatch.TournamentID != tournamentID {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Match does not belong to tournament")
		return nil, fmt.Errorf("match %d does not belong to tournament %d", matchID, tournamentID)
	}
	if currentMatch.Status == models.MatchStatusCompleted || currentMatch.Status == models.MatchStatusCanceled {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Match already completed/canceled")
		return nil, ErrMatchAlreadyCompleted
	}
	if currentMatch.T1ParticipantID == nil || currentMatch.T2ParticipantID == nil {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Match not ready, participants not set")
		return nil, fmt.Errorf("match %d is not ready, participants not set (T1: %v, T2: %v)", matchID, currentMatch.T1ParticipantID, currentMatch.T2ParticipantID)
	}

	isDraw := false
	var scoreT1, scoreT2 int
	if input.Score != nil && *input.Score != "" {
		var parseErr error
		scoreT1, scoreT2, isDraw, parseErr = parseScore(*input.Score)
		if parseErr != nil {
			s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Score parsing failed", slog.String("score_string", *input.Score), slog.Any("error", parseErr))
			return nil, parseErr
		}
	} else {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Score string is missing")
		return nil, fmt.Errorf("%w: score string is required", ErrScoreParsingFailed)
	}

	if isDraw {
		if input.WinnerParticipantID != nil {
			s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Winner specified for a draw")
			return nil, fmt.Errorf("cannot specify winner for a draw match (score: %s)", *input.Score)
		}
	} else {
		if input.WinnerParticipantID == nil {
			s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Winner not specified for a non-draw")
			return nil, errors.New("winner must be specified if the match is not a draw")
		}
		if *input.WinnerParticipantID != *currentMatch.T1ParticipantID && *input.WinnerParticipantID != *currentMatch.T2ParticipantID {
			s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Invalid winner ID",
				slog.Any("winner_id", input.WinnerParticipantID),
				slog.Any("t1_id", currentMatch.T1ParticipantID),
				slog.Any("t2_id", currentMatch.T2ParticipantID))
			return nil, fmt.Errorf("%w: winner ID %d is not T1 (%d) or T2 (%d) of match %d",
				ErrMatchInvalidWinner, *input.WinnerParticipantID, *currentMatch.T1ParticipantID, *currentMatch.T2ParticipantID, matchID)
		}
	}

	var updatedMatch *models.TeamMatch
	var nextMatchToNotify *models.TeamMatch
	isFinalMatchForBranch := currentMatch.NextMatchDBID == nil

	opErr := s.withTransaction(ctx, func(tx repositories.SQLExecutor) error {
		var txInternalErr error
		txInternalErr = s.teamMatchRepo.UpdateScoreStatusWinner(ctx, tx, matchID, input.Score, models.MatchStatusCompleted, input.WinnerParticipantID)
		if txInternalErr != nil {
			s.logger.ErrorContext(ctx, "UpdateTeamMatchResult: Failed to update match in DB", slog.Any("error", txInternalErr))
			return fmt.Errorf("failed to update current team match %d in transaction: %w", matchID, txInternalErr)
		}
		s.logger.InfoContext(ctx, "UpdateTeamMatchResult: Match updated in DB", slog.Int("match_id", matchID))

		if tournament.Format.BracketType == "RoundRobin" {
			p1Stand, errGetP1 := s.standingRepo.GetOrCreate(ctx, tx, tournamentID, *currentMatch.T1ParticipantID)
			if errGetP1 != nil {
				return fmt.Errorf("failed to get/create standing for T1 (%d): %w", *currentMatch.T1ParticipantID, errGetP1)
			}
			p2Stand, errGetP2 := s.standingRepo.GetOrCreate(ctx, tx, tournamentID, *currentMatch.T2ParticipantID)
			if errGetP2 != nil {
				return fmt.Errorf("failed to get/create standing for T2 (%d): %w", *currentMatch.T2ParticipantID, errGetP2)
			}

			p1Stand.GamesPlayed++
			p2Stand.GamesPlayed++
			p1Stand.ScoreFor += scoreT1
			p1Stand.ScoreAgainst += scoreT2
			p2Stand.ScoreFor += scoreT2
			p2Stand.ScoreAgainst += scoreT1
			p1Stand.ScoreDifference = p1Stand.ScoreFor - p1Stand.ScoreAgainst
			p2Stand.ScoreDifference = p2Stand.ScoreFor - p2Stand.ScoreAgainst

			if isDraw {
				p1Stand.Points++
				p1Stand.Draws++
				p2Stand.Points++
				p2Stand.Draws++
			} else {
				if input.WinnerParticipantID != nil {
					if *input.WinnerParticipantID == *currentMatch.T1ParticipantID {
						p1Stand.Points += 3
						p1Stand.Wins++
						p2Stand.Losses++
					} else {
						p2Stand.Points += 3
						p2Stand.Wins++
						p1Stand.Losses++
					}
				}
			}
			if err := s.standingRepo.Update(ctx, tx, p1Stand); err != nil {
				return fmt.Errorf("failed to update standing for T1 (%d): %w", p1Stand.ParticipantID, err)
			}
			if err := s.standingRepo.Update(ctx, tx, p2Stand); err != nil {
				return fmt.Errorf("failed to update standing for T2 (%d): %w", p2Stand.ParticipantID, err)
			}
			s.logger.InfoContext(ctx, "UpdateTeamMatchResult: Standings updated for RoundRobin", slog.Int("match_id", matchID))

		} else if tournament.Format.BracketType == "SingleElimination" {
			if currentMatch.NextMatchDBID != nil && currentMatch.WinnerToSlot != nil && input.WinnerParticipantID != nil {
				nextMatchID := *currentMatch.NextMatchDBID
				slotToPlaceWinner := *currentMatch.WinnerToSlot
				nextMatchForUpdate, errGetNext := s.teamMatchRepo.GetByID(ctx, nextMatchID) // Read outside tx
				if errGetNext != nil {
					s.logger.ErrorContext(ctx, "UpdateTeamMatchResult: Next match not found", slog.Any("error", errGetNext))
					return fmt.Errorf("%w: next match ID %d not found", ErrMatchCannotDetermineNext, nextMatchID)
				}
				if nextMatchForUpdate.Status == models.MatchStatusCompleted || nextMatchForUpdate.Status == models.MatchStatusCanceled {
					s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Next match already completed/canceled")
				} else {
					var t1, t2 *int
					if slotToPlaceWinner == 1 {
						t1 = input.WinnerParticipantID
						t2 = nextMatchForUpdate.T2ParticipantID
					} else if slotToPlaceWinner == 2 {
						t1 = nextMatchForUpdate.T1ParticipantID
						t2 = input.WinnerParticipantID
					} else {
						return fmt.Errorf("invalid winner_to_slot value %d for match %d", slotToPlaceWinner, matchID)
					}
					txInternalErr = s.teamMatchRepo.UpdateParticipants(ctx, tx, nextMatchID, t1, t2)
					if txInternalErr != nil {
						return fmt.Errorf("%w for next match %d: %w", ErrNextMatchParticipantSetFailed, nextMatchID, txInternalErr)
					}
					s.logger.InfoContext(ctx, "UpdateTeamMatchResult: Winner advanced in SingleElimination", slog.Int("match_id", matchID), slog.Int("next_match_id", nextMatchID))
				}
			}
		}
		return nil
	})

	if opErr != nil {
		return nil, opErr
	}

	var fetchErr error
	updatedMatch, fetchErr = s.teamMatchRepo.GetByID(ctx, matchID)
	if fetchErr != nil {
		s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Failed to fetch updated match post-transaction", slog.Any("error", fetchErr))
	}
	if tournament.Format.BracketType == "SingleElimination" && currentMatch.NextMatchDBID != nil {
		nextMatchToNotify, fetchErr = s.teamMatchRepo.GetByID(ctx, *currentMatch.NextMatchDBID)
		if fetchErr != nil {
			s.logger.WarnContext(ctx, "UpdateTeamMatchResult: Failed to fetch next match post-transaction", slog.Any("error", fetchErr))
			nextMatchToNotify = nil
		}
	}

	if s.hub != nil && updatedMatch != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: updatedMatch, RoomID: roomID})
		s.logger.InfoContext(ctx, "Sent MATCH_UPDATED", slog.Int("match_id", updatedMatch.ID), slog.String("room_id", roomID))

		if tournament.Format.BracketType == "RoundRobin" {
			standings, listErr := s.standingRepo.ListByTournament(ctx, s.db, tournamentID, true) // Use main db for read
			if listErr == nil {
				s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "STANDINGS_UPDATED", Payload: standings, RoomID: roomID})
				s.logger.InfoContext(ctx, "Sent STANDINGS_UPDATED", slog.Int("tournament_id", tournamentID))
			} else {
				s.logger.ErrorContext(ctx, "Failed to list standings for WebSocket broadcast", slog.Any("error", listErr))
			}
		} else if tournament.Format.BracketType == "SingleElimination" {
			if updatedMatch.NextMatchDBID != nil && updatedMatch.WinnerToSlot != nil && updatedMatch.WinnerParticipantID != nil {
				advPayload := map[string]interface{}{"advancing_participant_db_id": *updatedMatch.WinnerParticipantID, "source_match_id": updatedMatch.ID, "next_match_id": *updatedMatch.NextMatchDBID, "next_match_slot": *updatedMatch.WinnerToSlot, "tournament_id": tournamentID}
				s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "PARTICIPANT_ADVANCED", Payload: advPayload, RoomID: roomID})
				s.logger.InfoContext(ctx, "Sent PARTICIPANT_ADVANCED", slog.Int("match_id", updatedMatch.ID))
				if nextMatchToNotify != nil && (nextMatchToNotify.T1ParticipantID != nil && nextMatchToNotify.T2ParticipantID != nil) && nextMatchToNotify.Status == models.StatusScheduled {
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: nextMatchToNotify, RoomID: roomID})
					s.logger.InfoContext(ctx, "Sent MATCH_UPDATED for next match", slog.Int("next_match_id", nextMatchToNotify.ID))
				}
			}
		}
		if isFinalMatchForBranch && updatedMatch.WinnerParticipantID != nil && tournament.Format.BracketType == "SingleElimination" {
			finalMatchPayload := map[string]interface{}{"match_id": updatedMatch.ID, "tournament_id": tournamentID, "winner_participant_id": *updatedMatch.WinnerParticipantID, "is_tournament_final_match": true}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_FINAL_MATCH_COMPLETED", Payload: finalMatchPayload, RoomID: roomID})
			s.logger.InfoContext(ctx, "Sent TOURNAMENT_FINAL_MATCH_COMPLETED (candidate)", slog.Int("match_id", updatedMatch.ID))
		}
	}
	return updatedMatch, nil
}
