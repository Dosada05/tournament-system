package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"

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
	ErrSoloMatchNotFound             = errors.New("solo match not found")
	ErrTeamMatchNotFound             = errors.New("team match not found")
)

type UpdateMatchResultInput struct {
	Score               *string `json:"score" validate:"omitempty,max=50"`
	WinnerParticipantID int     `json:"winner_participant_id" validate:"required,gt=0"`
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
	hub             *brackets.Hub
}

func NewMatchService(
	db *sql.DB,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	tournamentRepo repositories.TournamentRepository,
	participantRepo repositories.ParticipantRepository,
	hub *brackets.Hub,
) MatchService {
	return &matchService{
		db:              db,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
		tournamentRepo:  tournamentRepo,
		participantRepo: participantRepo,
		hub:             hub,
	}
}

func (s *matchService) withTransaction(ctx context.Context, fn func(tx repositories.SQLExecutor) error) error {
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var opErr error
	defer func() {
		if p := recover(); p != nil {
			_ = dbTx.Rollback()
			panic(p)
		} else if opErr != nil {
			if rbErr := dbTx.Rollback(); rbErr != nil {
				log.Printf("Transaction error: %v (rollback failed: %v)", opErr, rbErr)
			}
		} else {
			if cErr := dbTx.Commit(); cErr != nil {
				opErr = fmt.Errorf("failed to commit transaction: %w", cErr)
			}
		}
	}()
	opErr = fn(dbTx)
	return opErr
}

func (s *matchService) ListSoloMatchesByTournament(ctx context.Context, tournamentID int) ([]*models.SoloMatch, error) {
	matches, err := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
	if err != nil {
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
		return nil, fmt.Errorf("failed to list team matches for tournament %d: %w", tournamentID, err)
	}
	if matches == nil {
		return []*models.TeamMatch{}, nil
	}
	return matches, nil
}

func (s *matchService) UpdateSoloMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.SoloMatch, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateSoloMatchResult: failed to get tournament %d", tournamentID)
	}
	if tournament.OrganizerID != currentUserID {
		return nil, ErrMatchUpdateForbidden
	}
	if tournament.Status != models.StatusActive {
		if tournament.Status == models.StatusCompleted || tournament.Status == models.StatusCanceled {
			return nil, fmt.Errorf("%w: tournament status is '%s'", ErrTournamentFinalized, tournament.Status)
		}
		return nil, fmt.Errorf("cannot update match result for tournament with status '%s'", tournament.Status)
	}

	currentMatch, err := s.soloMatchRepo.GetByID(ctx, matchID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrSoloMatchNotFound, "UpdateSoloMatchResult: failed to get solo match %d", matchID)
	}

	if currentMatch.TournamentID != tournamentID {
		return nil, fmt.Errorf("match %d does not belong to tournament %d", matchID, tournamentID)
	}
	if currentMatch.Status == models.MatchStatusCompleted || currentMatch.Status == models.MatchStatusCanceled {
		return nil, ErrMatchAlreadyCompleted
	}
	if currentMatch.P1ParticipantID == nil || currentMatch.P2ParticipantID == nil {
		return nil, fmt.Errorf("match %d is not ready, participants not set (P1: %v, P2: %v)", matchID, currentMatch.P1ParticipantID, currentMatch.P2ParticipantID)
	}
	if input.WinnerParticipantID != *currentMatch.P1ParticipantID && input.WinnerParticipantID != *currentMatch.P2ParticipantID {
		return nil, fmt.Errorf("%w: winner ID %d is not P1 (%d) or P2 (%d) of match %d",
			ErrMatchInvalidWinner, input.WinnerParticipantID, *currentMatch.P1ParticipantID, *currentMatch.P2ParticipantID, matchID)
	}

	var updatedMatch *models.SoloMatch
	var nextMatchToNotify *models.SoloMatch
	isFinalMatch := currentMatch.NextMatchDBID == nil // Определяем, является ли текущий матч финальным

	opErr := s.withTransaction(ctx, func(tx repositories.SQLExecutor) error {
		var txInternalErr error
		txInternalErr = s.soloMatchRepo.UpdateScoreStatusWinner(ctx, matchID, input.Score, models.MatchStatusCompleted, &input.WinnerParticipantID)
		if txInternalErr != nil {
			return fmt.Errorf("failed to update current solo match %d in transaction: %w", matchID, txInternalErr)
		}
		log.Printf("Solo match %d result updated in transaction. Winner: %d, Score: %s", matchID, input.WinnerParticipantID, derefString(input.Score))

		if currentMatch.NextMatchDBID != nil && currentMatch.WinnerToSlot != nil {
			nextMatchID := *currentMatch.NextMatchDBID
			slotToPlaceWinner := *currentMatch.WinnerToSlot

			nextMatchForUpdate, errGetNext := s.soloMatchRepo.GetByID(ctx, nextMatchID) // Читаем вне tx, т.к. GetByID не транзакционный
			if errGetNext != nil {
				log.Printf("Error: Next match DB ID %d not found for solo match %d winner advancement. This indicates a broken bracket link.", nextMatchID, matchID)
				return fmt.Errorf("%w: next match ID %d not found", ErrMatchCannotDetermineNext, nextMatchID)
			}

			if nextMatchForUpdate.Status == models.MatchStatusCompleted || nextMatchForUpdate.Status == models.MatchStatusCanceled {
				log.Printf("Warning: Next match DB ID %d for solo match %d is already completed or canceled. Cannot advance winner.", nextMatchID, matchID)
			} else {
				var p1, p2 *int
				if slotToPlaceWinner == 1 {
					p1 = &input.WinnerParticipantID
					p2 = nextMatchForUpdate.P2ParticipantID
				} else if slotToPlaceWinner == 2 {
					p1 = nextMatchForUpdate.P1ParticipantID
					p2 = &input.WinnerParticipantID
				} else {
					return fmt.Errorf("invalid winner_to_slot value %d for match %d", slotToPlaceWinner, matchID)
				}
				txInternalErr = s.soloMatchRepo.UpdateParticipants(ctx, tx, nextMatchID, p1, p2)
				if txInternalErr != nil {
					return fmt.Errorf("%w for next match %d: %w", ErrNextMatchParticipantSetFailed, nextMatchID, txInternalErr)
				}
				log.Printf("Winner of solo match %d (Participant %d) advanced to slot %d of next match %d in transaction", matchID, input.WinnerParticipantID, slotToPlaceWinner, nextMatchID)
			}
		} else {
			log.Printf("Solo match %d is a final match in its branch or for the tournament.", matchID)
		}
		return nil
	})

	if opErr != nil {
		return nil, opErr
	}

	var fetchErr error
	updatedMatch, fetchErr = s.soloMatchRepo.GetByID(ctx, matchID)
	if fetchErr != nil {
		log.Printf("Warning: failed to fetch updated solo match %d post-transaction: %v", matchID, fetchErr)
	}
	if currentMatch.NextMatchDBID != nil { // Используем currentMatch, т.к. updatedMatch мог не загрузиться
		nextMatchToNotify, fetchErr = s.soloMatchRepo.GetByID(ctx, *currentMatch.NextMatchDBID)
		if fetchErr != nil {
			log.Printf("Warning: failed to fetch next solo match %d post-transaction: %v", *currentMatch.NextMatchDBID, fetchErr)
			nextMatchToNotify = nil
		}
	}

	if s.hub != nil && updatedMatch != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: updatedMatch, RoomID: roomID})
		log.Printf("Sent MATCH_UPDATED for solo match %d to room %s", updatedMatch.ID, roomID)

		if updatedMatch.NextMatchDBID != nil && updatedMatch.WinnerToSlot != nil && updatedMatch.WinnerParticipantID != nil {
			advancementPayload := map[string]interface{}{
				"advancing_participant_db_id": *updatedMatch.WinnerParticipantID,
				"source_match_id":             updatedMatch.ID,
				"next_match_id":               *updatedMatch.NextMatchDBID,
				"next_match_slot":             *updatedMatch.WinnerToSlot,
				"tournament_id":               tournamentID,
			}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "PARTICIPANT_ADVANCED", Payload: advancementPayload, RoomID: roomID})
			log.Printf("Sent PARTICIPANT_ADVANCED from solo match %d to room %s", updatedMatch.ID, roomID)

			if nextMatchToNotify != nil {
				if (nextMatchToNotify.P1ParticipantID != nil && nextMatchToNotify.P2ParticipantID != nil) && nextMatchToNotify.Status == models.StatusScheduled {
					log.Printf("Next solo match %d is now ready. Sending MATCH_UPDATED.", nextMatchToNotify.ID)
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: nextMatchToNotify, RoomID: roomID})
				}
			}
		}

		if isFinalMatch && updatedMatch.WinnerParticipantID != nil {
			// Отправляем информацию о том, что финальный матч завершен
			// TournamentHandler затем вызовет TournamentService.FinalizeTournament,
			// который отправит TOURNAMENT_COMPLETED
			finalMatchPayload := map[string]interface{}{
				"match_id":                  updatedMatch.ID,
				"tournament_id":             tournamentID,
				"winner_participant_id":     *updatedMatch.WinnerParticipantID,
				"is_tournament_final_match": true,
			}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_FINAL_MATCH_COMPLETED", Payload: finalMatchPayload, RoomID: roomID})
			log.Printf("Sent TOURNAMENT_FINAL_MATCH_COMPLETED for tournament %d, match %d, winner %d", tournamentID, updatedMatch.ID, *updatedMatch.WinnerParticipantID)
		}
	}
	return updatedMatch, nil
}

func (s *matchService) UpdateTeamMatchResult(ctx context.Context, matchID int, tournamentID int, input UpdateMatchResultInput, currentUserID int) (*models.TeamMatch, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateTeamMatchResult: failed to get tournament %d", tournamentID)
	}
	if tournament.OrganizerID != currentUserID {
		return nil, ErrMatchUpdateForbidden
	}
	if tournament.Status != models.StatusActive {
		if tournament.Status == models.StatusCompleted || tournament.Status == models.StatusCanceled {
			return nil, fmt.Errorf("%w: tournament status is '%s'", ErrTournamentFinalized, tournament.Status)
		}
		return nil, fmt.Errorf("cannot update match result for tournament with status '%s'", tournament.Status)
	}

	currentMatch, err := s.teamMatchRepo.GetByID(ctx, matchID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTeamMatchNotFound, "UpdateTeamMatchResult: failed to get team match %d", matchID)
	}

	if currentMatch.TournamentID != tournamentID {
		return nil, fmt.Errorf("match %d does not belong to tournament %d", matchID, tournamentID)
	}
	if currentMatch.Status == models.MatchStatusCompleted || currentMatch.Status == models.MatchStatusCanceled {
		return nil, ErrMatchAlreadyCompleted
	}
	if currentMatch.T1ParticipantID == nil || currentMatch.T2ParticipantID == nil {
		return nil, fmt.Errorf("match %d is not ready, participants not set (T1: %v, T2: %v)", matchID, currentMatch.T1ParticipantID, currentMatch.T2ParticipantID)
	}
	if input.WinnerParticipantID != *currentMatch.T1ParticipantID && input.WinnerParticipantID != *currentMatch.T2ParticipantID {
		return nil, fmt.Errorf("%w: winner ID %d is not T1 (%d) or T2 (%d) of match %d",
			ErrMatchInvalidWinner, input.WinnerParticipantID, *currentMatch.T1ParticipantID, *currentMatch.T2ParticipantID, matchID)
	}

	var updatedMatch *models.TeamMatch
	var nextMatchToNotify *models.TeamMatch
	isFinalMatch := currentMatch.NextMatchDBID == nil

	opErr := s.withTransaction(ctx, func(tx repositories.SQLExecutor) error {
		var txInternalErr error
		txInternalErr = s.teamMatchRepo.UpdateScoreStatusWinner(ctx, matchID, input.Score, models.MatchStatusCompleted, &input.WinnerParticipantID)
		if txInternalErr != nil {
			return fmt.Errorf("failed to update current team match %d in transaction: %w", matchID, txInternalErr)
		}
		log.Printf("Team match %d result updated in transaction. Winner: %d, Score: %s", matchID, input.WinnerParticipantID, derefString(input.Score))

		if currentMatch.NextMatchDBID != nil && currentMatch.WinnerToSlot != nil {
			nextMatchID := *currentMatch.NextMatchDBID
			slotToPlaceWinner := *currentMatch.WinnerToSlot

			nextMatchForUpdate, errGetNext := s.teamMatchRepo.GetByID(ctx, nextMatchID)
			if errGetNext != nil {
				log.Printf("Error: Next match DB ID %d not found for team match %d winner advancement.", nextMatchID, matchID)
				return fmt.Errorf("%w: next match ID %d not found", ErrMatchCannotDetermineNext, nextMatchID)
			}

			if nextMatchForUpdate.Status == models.MatchStatusCompleted || nextMatchForUpdate.Status == models.MatchStatusCanceled {
				log.Printf("Warning: Next match DB ID %d for team match %d is already completed or canceled. Cannot advance winner.", nextMatchID, matchID)
			} else {
				var t1, t2 *int
				if slotToPlaceWinner == 1 {
					t1 = &input.WinnerParticipantID
					t2 = nextMatchForUpdate.T2ParticipantID
				} else if slotToPlaceWinner == 2 {
					t1 = nextMatchForUpdate.T1ParticipantID
					t2 = &input.WinnerParticipantID
				} else {
					return fmt.Errorf("invalid winner_to_slot value %d for match %d", slotToPlaceWinner, matchID)
				}
				txInternalErr = s.teamMatchRepo.UpdateParticipants(ctx, tx, nextMatchID, t1, t2)
				if txInternalErr != nil {
					return fmt.Errorf("%w for next match %d: %w", ErrNextMatchParticipantSetFailed, nextMatchID, txInternalErr)
				}
				log.Printf("Winner of team match %d (Participant %d) advanced to slot %d of next match %d in transaction", matchID, input.WinnerParticipantID, slotToPlaceWinner, nextMatchID)
			}
		} else {
			log.Printf("Team match %d is a final match in its branch or for the tournament.", matchID)
		}
		return nil
	})

	if opErr != nil {
		return nil, opErr
	}

	var fetchErr error
	updatedMatch, fetchErr = s.teamMatchRepo.GetByID(ctx, matchID)
	if fetchErr != nil {
		log.Printf("Warning: failed to fetch updated team match %d post-transaction: %v", matchID, fetchErr)
	}
	if currentMatch.NextMatchDBID != nil {
		nextMatchToNotify, fetchErr = s.teamMatchRepo.GetByID(ctx, *currentMatch.NextMatchDBID)
		if fetchErr != nil {
			log.Printf("Warning: failed to fetch next team match %d post-transaction: %v", *currentMatch.NextMatchDBID, fetchErr)
			nextMatchToNotify = nil
		}
	}

	if s.hub != nil && updatedMatch != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: updatedMatch, RoomID: roomID})
		log.Printf("Sent MATCH_UPDATED for team match %d to room %s", updatedMatch.ID, roomID)

		if updatedMatch.NextMatchDBID != nil && updatedMatch.WinnerToSlot != nil && updatedMatch.WinnerParticipantID != nil {
			advancementPayload := map[string]interface{}{
				"advancing_participant_db_id": *updatedMatch.WinnerParticipantID,
				"source_match_id":             updatedMatch.ID,
				"next_match_id":               *updatedMatch.NextMatchDBID,
				"next_match_slot":             *updatedMatch.WinnerToSlot,
				"tournament_id":               tournamentID,
			}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "PARTICIPANT_ADVANCED", Payload: advancementPayload, RoomID: roomID})
			log.Printf("Sent PARTICIPANT_ADVANCED from team match %d to room %s", updatedMatch.ID, roomID)

			if nextMatchToNotify != nil {
				if (nextMatchToNotify.T1ParticipantID != nil && nextMatchToNotify.T2ParticipantID != nil) && nextMatchToNotify.Status == models.StatusScheduled {
					log.Printf("Next team match %d is now ready. Sending MATCH_UPDATED.", nextMatchToNotify.ID)
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "MATCH_UPDATED", Payload: nextMatchToNotify, RoomID: roomID})
				}
			}
		}

		if isFinalMatch && updatedMatch.WinnerParticipantID != nil {
			finalMatchPayload := map[string]interface{}{
				"match_id":                  updatedMatch.ID,
				"tournament_id":             tournamentID,
				"winner_participant_id":     *updatedMatch.WinnerParticipantID,
				"is_tournament_final_match": true,
			}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_FINAL_MATCH_COMPLETED", Payload: finalMatchPayload, RoomID: roomID})
			log.Printf("Sent TOURNAMENT_FINAL_MATCH_COMPLETED for tournament %d, match %d, winner %d", tournamentID, updatedMatch.ID, *updatedMatch.WinnerParticipantID)
		}
	}
	return updatedMatch, nil
}
