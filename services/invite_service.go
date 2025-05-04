package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

const (
	inviteTokenBytes = 32
	inviteDuration   = 1 * time.Hour
)

var (
	ErrTeamFull            = errors.New("team is full, cannot accept invite")
	ErrInviteCreateOrRenew = errors.New("failed to create or renew invite")
	ErrInviteValidation    = errors.New("failed to validate or use invite")
	ErrInviteRevokeFailed  = errors.New("failed to revoke invite")
	ErrInviteGetFailed     = errors.New("failed to get team invite")
	ErrInviteTokenConflict = repositories.ErrInviteTokenConflict
	ErrInviteTeamInvalid   = repositories.ErrInviteTeamInvalid
)

type InviteService interface {
	CreateOrRenewInvite(ctx context.Context, teamID int, currentUserID int) (*models.Invite, error)
	ValidateAndJoinTeam(ctx context.Context, token string, joiningUserID int) (*models.Team, error)
	GetTeamInvite(ctx context.Context, teamID int, currentUserID int) (*models.Invite, error)
	RevokeInvite(ctx context.Context, teamID int, currentUserID int) error
}

type inviteService struct {
	inviteRepo repositories.InviteRepository
	teamRepo   repositories.TeamRepository
	userRepo   repositories.UserRepository
}

func NewInviteService(
	ir repositories.InviteRepository,
	tr repositories.TeamRepository,
	ur repositories.UserRepository,
) InviteService {
	return &inviteService{
		inviteRepo: ir,
		teamRepo:   tr,
		userRepo:   ur,
	}
}

func (s *inviteService) CreateOrRenewInvite(ctx context.Context, teamID int, currentUserID int) (*models.Invite, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrInviteCreateOrRenew, ErrTeamNotFound)
		}
		return nil, fmt.Errorf("%w: failed to get team: %w", ErrInviteCreateOrRenew, err)
	}

	if team.CaptainID != currentUserID {
		return nil, fmt.Errorf("%w: only team captain can manage invites", ErrForbiddenOperation)
	}

	existingInvite, err := s.inviteRepo.GetValidByTeamID(ctx, teamID)
	if err != nil && !errors.Is(err, repositories.ErrInviteNotFound) {
		return nil, fmt.Errorf("%w: failed to check existing invite: %w", ErrInviteCreateOrRenew, err)
	}

	newToken, err := generateSecureToken(inviteTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate token: %w", ErrInviteCreateOrRenew, err)
	}
	expiresAt := time.Now().Add(inviteDuration)

	if existingInvite != nil {
		existingInvite.Token = newToken
		existingInvite.ExpiresAt = expiresAt
		err = s.inviteRepo.Update(ctx, existingInvite)
		if err != nil {
			if errors.Is(err, repositories.ErrInviteTokenConflict) {
				return nil, fmt.Errorf("%w: token conflict during update, try again: %w", ErrInviteCreateOrRenew, err)
			}
			return nil, fmt.Errorf("%w: failed to update invite: %w", ErrInviteCreateOrRenew, err)
		}
		return existingInvite, nil
	}

	newInvite := &models.Invite{
		TeamID:    teamID,
		Token:     newToken,
		ExpiresAt: expiresAt,
	}
	err = s.inviteRepo.Create(ctx, newInvite)
	if err != nil {
		if errors.Is(err, repositories.ErrInviteTokenConflict) {
			return nil, fmt.Errorf("%w: token conflict during create, try again: %w", ErrInviteCreateOrRenew, err)
		}
		if errors.Is(err, repositories.ErrInviteTeamInvalid) {
			return nil, fmt.Errorf("%w: invalid team id during create: %w", ErrInviteCreateOrRenew, err)
		}
		return nil, fmt.Errorf("%w: failed to create invite: %w", ErrInviteCreateOrRenew, err)
	}

	return newInvite, nil
}

func (s *inviteService) ValidateAndJoinTeam(ctx context.Context, token string, joiningUserID int) (*models.Team, error) {
	invite, err := s.inviteRepo.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repositories.ErrInviteNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrInviteValidation, ErrInviteNotFound)
		}
		return nil, fmt.Errorf("%w: failed to get invite by token: %w", ErrInviteValidation, err)
	}

	if time.Now().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("%w: expired at %s", ErrInviteExpired, invite.ExpiresAt.Format(time.RFC3339))
	}

	joiningUser, err := s.userRepo.GetByID(ctx, joiningUserID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrInviteValidation, ErrUserNotFound)
		}
		return nil, fmt.Errorf("%w: failed to get joining user: %w", ErrInviteValidation, err)
	}

	if joiningUser.TeamID != nil {
		if *joiningUser.TeamID == invite.TeamID {
			return nil, fmt.Errorf("%w: user already in this team", ErrUserAlreadyInTeam)
		}
		return nil, fmt.Errorf("%w: user already belongs to team %d", ErrUserAlreadyInTeam, *joiningUser.TeamID)
	}

	team, err := s.teamRepo.GetByID(ctx, invite.TeamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, fmt.Errorf("%w: target %w", ErrInviteValidation, ErrTeamNotFound)
		}
		return nil, fmt.Errorf("%w: failed to get target team: %w", ErrInviteValidation, err)
	}

	joiningUser.TeamID = &team.ID
	err = s.userRepo.Update(ctx, joiningUser)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to assign user to team: %w", ErrInviteValidation, err)
	}

	return team, nil
}

func (s *inviteService) GetTeamInvite(ctx context.Context, teamID int, currentUserID int) (*models.Invite, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrInviteGetFailed, ErrTeamNotFound)
		}
		return nil, fmt.Errorf("%w: failed to get team: %w", ErrInviteGetFailed, err)
	}

	if team.CaptainID != currentUserID {
		return nil, fmt.Errorf("%w: only team captain can view the invite link", ErrForbiddenOperation)
	}

	invite, err := s.inviteRepo.GetValidByTeamID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrInviteNotFound) {
			return nil, ErrInviteNotFound
		}
		return nil, fmt.Errorf("%w: failed to get valid invite from repo: %w", ErrInviteGetFailed, err)
	}

	return invite, nil
}

func (s *inviteService) RevokeInvite(ctx context.Context, teamID int, currentUserID int) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return fmt.Errorf("%w: %w", ErrInviteRevokeFailed, ErrTeamNotFound)
		}
		return fmt.Errorf("%w: failed to get team: %w", ErrInviteRevokeFailed, err)
	}

	if team.CaptainID != currentUserID {
		return fmt.Errorf("%w: only team captain can revoke invites", ErrForbiddenOperation)
	}

	_, err = s.inviteRepo.DeleteByTeamID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("%w: failed to delete invites from repo: %w", ErrInviteRevokeFailed, err)
	}

	return nil
}

func generateSecureToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
