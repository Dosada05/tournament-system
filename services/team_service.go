package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrTeamCreationFailed       = errors.New("failed to create team")
	ErrTeamUpdateFailed         = errors.New("failed to update team")
	ErrTeamDeleteFailed         = errors.New("failed to delete team")
	ErrTeamCannotDeleteNotEmpty = errors.New("team cannot be deleted because it is not empty")
	ErrUserNotInThisTeam        = errors.New("user is not a member of this team")
	ErrMemberAddFailed          = errors.New("failed to add member to team")
	ErrMemberRemoveFailed       = errors.New("failed to remove member from team")
	ErrInvalidSportID           = errors.New("invalid sport ID provided")
	// ErrUserNotFound предполагается определенным в user_service.go или общем errors.go
	// ErrForbiddenOperation предполагается определенным здесь или в общем errors.go
)

type TeamService interface {
	CreateTeam(ctx context.Context, input CreateTeamInput) (*models.Team, error)
	GetTeamByID(ctx context.Context, teamID int) (*models.Team, error)
	UpdateTeamDetails(ctx context.Context, teamID int, input UpdateTeamInput, currentUserID int) (*models.Team, error)
	AddMember(ctx context.Context, teamID int, userID int, currentUserID int) error
	RemoveMember(ctx context.Context, teamID int, userIDToRemove int, currentUserID int) error
	DeleteTeam(ctx context.Context, teamID int, currentUserID int) error
}

type CreateTeamInput struct {
	Name      string `json:"name"`
	SportID   int    `json:"sport_id"`
	CreatorID int
}

type UpdateTeamInput struct {
	Name    *string
	SportID *int
}

type teamService struct {
	teamRepo  repositories.TeamRepository
	userRepo  repositories.UserRepository // Предполагается наличие метода ListByTeamID
	sportRepo repositories.SportRepository
}

func NewTeamService(
	teamRepo repositories.TeamRepository,
	userRepo repositories.UserRepository,
	sportRepo repositories.SportRepository,
) TeamService {
	return &teamService{
		teamRepo:  teamRepo,
		userRepo:  userRepo,
		sportRepo: sportRepo,
	}
}

func (s *teamService) CreateTeam(ctx context.Context, input CreateTeamInput) (*models.Team, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTeamNameRequired
	}

	if _, err := s.sportRepo.GetByID(ctx, input.SportID); err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrInvalidSportID
		}
		return nil, fmt.Errorf("failed to verify sport %d: %w", input.SportID, err)
	}

	creator, err := s.userRepo.GetByID(ctx, input.CreatorID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound // Используем ошибку из user_service или общую
		}
		return nil, fmt.Errorf("failed to get creator user %d: %w", input.CreatorID, err)
	}

	if creator.TeamID != nil {
		return nil, ErrUserAlreadyInTeam
	}

	team := &models.Team{
		Name:      name,
		SportID:   input.SportID,
		CaptainID: input.CreatorID, // Используем CaptainID как в репозитории
	}

	err = s.teamRepo.Create(ctx, team)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrTeamNameConflict):
			return nil, ErrTeamNameConflict
		case errors.Is(err, repositories.ErrTeamCaptainInvalid):
			// Эта ошибка репозитория означает, что CreatorID невалиден
			return nil, ErrUserNotFound
		case errors.Is(err, repositories.ErrTeamSportInvalid):
			// Эта ошибка репозитория означает, что SportID невалиден
			return nil, ErrInvalidSportID
		default:
			return nil, fmt.Errorf("%w: %w", ErrTeamCreationFailed, err)
		}
	}

	creator.TeamID = &team.ID // team.ID присваивается после успешного Create
	err = s.userRepo.Update(ctx, creator)
	if err != nil {
		// Попытка отката
		_ = s.teamRepo.Delete(ctx, team.ID) // Игнорируем ошибку отката
		// Возвращаем осмысленную ошибку
		return nil, fmt.Errorf("failed to assign creator %d to new team %d: %w", creator.ID, team.ID, err)
	}

	return team, nil
}

func (s *teamService) GetTeamByID(ctx context.Context, teamID int) (*models.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team by id %d: %w", teamID, err)
	}
	return team, nil
}

func (s *teamService) UpdateTeamDetails(ctx context.Context, teamID int, input UpdateTeamInput, currentUserID int) (*models.Team, error) {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Проверка прав: только капитан может редактировать
	if team.CaptainID != currentUserID {
		return nil, ErrCaptainActionForbidden
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrTeamNameRequired
		}
		if trimmedName != team.Name {
			team.Name = trimmedName
			updated = true
		}
	}
	if input.SportID != nil && *input.SportID != team.SportID {
		// Проверяем существование нового SportID
		if _, err := s.sportRepo.GetByID(ctx, *input.SportID); err != nil {
			if errors.Is(err, repositories.ErrSportNotFound) {
				return nil, ErrInvalidSportID
			}
			return nil, fmt.Errorf("failed to verify sport %d for update: %w", *input.SportID, err)
		}
		team.SportID = *input.SportID
		updated = true
	}

	if !updated {
		return team, nil // Нет изменений, не вызываем Update
	}

	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrTeamNameConflict):
			return nil, ErrTeamNameConflict
		case errors.Is(err, repositories.ErrTeamCaptainInvalid):
			// Ошибка FK на капитана при Update маловероятна, но обрабатываем
			return nil, ErrUserNotFound
		case errors.Is(err, repositories.ErrTeamSportInvalid):
			// Ошибка FK на спорт при Update
			return nil, ErrInvalidSportID
		case errors.Is(err, repositories.ErrTeamNotFound):
			// Команда была удалена между GetByID и Update
			return nil, ErrTeamNotFound
		default:
			return nil, fmt.Errorf("%w: %w", ErrTeamUpdateFailed, err)
		}
	}

	return team, nil
}

func (s *teamService) AddMember(ctx context.Context, teamID int, userID int, currentUserID int) error {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Проверка прав: только капитан может добавлять
	if team.CaptainID != currentUserID {
		return ErrCaptainActionForbidden
	}

	userToAdd, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user %d to add: %w", userID, err)
	}

	// Проверка: пользователь уже в команде?
	if userToAdd.TeamID != nil {
		// Дополнительно можно проверить: if *userToAdd.TeamID == teamID { return errors.New("user already in this team") }
		return ErrUserAlreadyInTeam
	}

	userToAdd.TeamID = &team.ID // Присваиваем ID команды
	err = s.userRepo.Update(ctx, userToAdd)
	if err != nil {
		// Обработка ошибки обновления пользователя (например, конфликт)
		return fmt.Errorf("%w: %w", ErrMemberAddFailed, err)
	}

	return nil
}

func (s *teamService) RemoveMember(ctx context.Context, teamID int, userIDToRemove int, currentUserID int) error {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	userToRemove, err := s.userRepo.GetByID(ctx, userIDToRemove)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user %d to remove: %w", userIDToRemove, err)
	}

	// Проверка: пользователь состоит в *этой* команде?
	if userToRemove.TeamID == nil || *userToRemove.TeamID != team.ID {
		return ErrUserNotInThisTeam
	}

	// Проверка прав: действие выполняет капитан ИЛИ пользователь удаляет сам себя
	isCaptainAction := team.CaptainID == currentUserID
	isSelfLeave := userIDToRemove == currentUserID

	if !isCaptainAction && !isSelfLeave {
		return ErrSelfLeaveForbidden // Ни капитан, ни сам пользователь
	}

	// Проверка: нельзя удалить капитана
	if userToRemove.ID == team.CaptainID {
		return ErrCannotRemoveCaptain
	}

	userToRemove.TeamID = nil // Обнуляем TeamID
	err = s.userRepo.Update(ctx, userToRemove)
	if err != nil {
		// Обработка ошибки обновления пользователя
		return fmt.Errorf("%w: %w", ErrMemberRemoveFailed, err)
	}

	return nil
}

// DeleteTeam требует, чтобы UserRepository имел метод ListByTeamID
func (s *teamService) DeleteTeam(ctx context.Context, teamID int, currentUserID int) error {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Проверка прав: только капитан может удалить команду
	if team.CaptainID != currentUserID {
		return ErrCaptainActionForbidden
	}

	// Проверка: команда должна быть пустой (кроме капитана)
	// **ПРЕДПОЛАГАЕТСЯ НАЛИЧИЕ userRepo.ListByTeamID**
	members, err := s.userRepo.ListByTeamID(ctx, teamID)
	if err != nil {
		// Обработка ошибки получения списка членов
		return fmt.Errorf("failed to check team members for deletion: %w", err)
	}

	if len(members) > 1 { // Если в команде больше одного человека
		return fmt.Errorf("%w: team has %d members", ErrTeamCannotDeleteNotEmpty, len(members))
	}
	// Дополнительная проверка: если остался один, то это должен быть капитан
	if len(members) == 1 && members[0].ID != team.CaptainID {
		// Эта ситуация не должна возникать при правильной логике Add/Remove, но проверим
		return fmt.Errorf("%w: the only remaining member is not the captain", ErrTeamCannotDeleteNotEmpty)
	}

	// Удаляем команду из репозитория
	err = s.teamRepo.Delete(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return ErrTeamNotFound // Маловероятно после GetByID
		}
		// Другие возможные ошибки удаления (например, FK, если не обработано репозиторием)
		return fmt.Errorf("%w: %w", ErrTeamDeleteFailed, err)
	}

	// Если удаление команды прошло успешно и в команде был только капитан,
	// обнуляем TeamID у этого капитана.
	if len(members) == 1 && members[0].ID == team.CaptainID {
		captain := members[0] // Получаем модель капитана из списка
		captain.TeamID = nil
		errUpdate := s.userRepo.Update(ctx, &captain) // Передаем указатель на модель
		if errUpdate != nil {
			// Логируем ошибку, но не возвращаем ее, т.к. команда уже удалена
			fmt.Printf("Warning: failed to remove team ID from captain %d after team %d deletion: %v\n", captain.ID, teamID, errUpdate)
		}
	}

	return nil
}
