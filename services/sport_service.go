package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
)

const (
	sportLogoPrefix = "logos/sports"
)

var (
	ErrSportNameRequired       = errors.New("sport name is required")
	ErrSportNameConflict       = errors.New("sport name already exists")
	ErrSportInUse              = errors.New("sport cannot be deleted as it is currently in use")
	ErrSportCreationFailed     = errors.New("failed to create sport")
	ErrSportUpdateFailed       = errors.New("failed to update sport")
	ErrSportDeleteFailed       = errors.New("failed to delete sport")
	ErrSportLogoUpdateDBFailed = errors.New("failed to update sport logo information in database")
	ErrSportLogoUploadFailed   = errors.New("failed to upload sport logo")
)

type SportService interface {
	CreateSport(ctx context.Context, input CreateSportInput) (*models.Sport, error)
	GetSportByID(ctx context.Context, id int) (*models.Sport, error)
	GetAllSports(ctx context.Context) ([]models.Sport, error)
	UpdateSport(ctx context.Context, id int, input UpdateSportInput) (*models.Sport, error)
	DeleteSport(ctx context.Context, id int) error
	UploadSportLogo(ctx context.Context, sportID int, currentUserID int, file io.Reader, contentType string) (*models.Sport, error)
}

type CreateSportInput struct {
	Name string
}

type UpdateSportInput struct {
	Name string
}

type sportService struct {
	sportRepo repositories.SportRepository
	userRepo  repositories.UserRepository // Для проверки прав админа
	uploader  storage.FileUploader
}

func NewSportService(
	sportRepo repositories.SportRepository,
	userRepo repositories.UserRepository,
	uploader storage.FileUploader,
) SportService {
	return &sportService{
		sportRepo: sportRepo,
		userRepo:  userRepo,
		uploader:  uploader,
	}
}

func (s *sportService) populateSportLogoURL(sport *models.Sport) {
	if sport != nil && sport.LogoKey != nil && *sport.LogoKey != "" && s.uploader != nil {
		url := s.uploader.GetPublicURL(*sport.LogoKey)
		if url != "" {
			sport.LogoURL = &url
		}
	}
}

func (s *sportService) populateSportListLogoURLs(sports []models.Sport) {
	for i := range sports {
		s.populateSportLogoURL(&sports[i])
	}
}

func (s *sportService) CreateSport(ctx context.Context, input CreateSportInput) (*models.Sport, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrSportNameRequired
	}

	sport := &models.Sport{
		Name: name,
		// LogoKey изначально nil
	}

	err := s.sportRepo.Create(ctx, sport)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNameConflict) {
			return nil, ErrSportNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrSportCreationFailed, err)
	}
	// LogoURL здесь не заполняем, т.к. лого еще нет
	return sport, nil
}

func (s *sportService) GetSportByID(ctx context.Context, id int) (*models.Sport, error) {
	sport, err := s.sportRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrSportNotFound
		}
		return nil, fmt.Errorf("failed to get sport by id %d: %w", id, err)
	}
	s.populateSportLogoURL(sport)
	return sport, nil
}

func (s *sportService) GetAllSports(ctx context.Context) ([]models.Sport, error) {
	sports, err := s.sportRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all sports: %w", err)
	}
	if sports == nil {
		return []models.Sport{}, nil
	}
	s.populateSportListLogoURLs(sports)
	return sports, nil
}

func (s *sportService) UpdateSport(ctx context.Context, id int, input UpdateSportInput) (*models.Sport, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrSportNameRequired
	}

	// Сначала получаем текущий спорт, чтобы сохранить LogoKey
	sportToUpdate, err := s.sportRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrSportNotFound
		}
		return nil, fmt.Errorf("failed to get sport %d for update: %w", id, err)
	}

	sportToUpdate.Name = name // Обновляем только имя

	err = s.sportRepo.Update(ctx, sportToUpdate) // Репозиторий обновит только имя
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrSportNotFound):
			return nil, ErrSportNotFound
		case errors.Is(err, repositories.ErrSportNameConflict):
			return nil, ErrSportNameConflict
		default:
			return nil, fmt.Errorf("%w (id: %d): %w", ErrSportUpdateFailed, id, err)
		}
	}
	s.populateSportLogoURL(sportToUpdate) // LogoKey остался прежним, просто обновляем URL
	return sportToUpdate, nil
}

func (s *sportService) DeleteSport(ctx context.Context, id int) error {
	// Перед удалением из БД, получим спорт, чтобы знать его logo_key
	sport, err := s.sportRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return ErrSportNotFound // Уже удален или не существует
		}
		return fmt.Errorf("failed to get sport %d for logo key before deletion: %w", id, err)
	}

	oldLogoKey := sport.LogoKey

	err = s.sportRepo.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrSportNotFound):
			return ErrSportNotFound
		case errors.Is(err, repositories.ErrSportInUse):
			return ErrSportInUse
		default:
			return fmt.Errorf("%w (id: %d): %w", ErrSportDeleteFailed, id, err)
		}
	}

	// Если удаление из БД прошло успешно и был логотип, удаляем его из хранилища
	if oldLogoKey != nil && *oldLogoKey != "" && s.uploader != nil {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete sport logo %s during sport deletion: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}
	return nil
}

func (s *sportService) UploadSportLogo(ctx context.Context, sportID int, currentUserID int, file io.Reader, contentType string) (*models.Sport, error) {
	// Проверка прав: только администратор может загружать лого для спорта
	// (Предполагаем, что GetUserRoleFromContext есть в middleware и используется в хендлере)
	// Здесь можно добавить еще одну проверку, если currentUserID передан и есть доступ к userRepo
	// user, err := s.userRepo.GetByID(ctx, currentUserID)
	// if err != nil || user.Role != models.RoleAdmin {
	//     return nil, ErrForbiddenOperation
	// }
	// Эта проверка обычно делается на уровне хэндлера, здесь для полноты картины.

	sport, err := s.sportRepo.GetByID(ctx, sportID)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrSportNotFound
		}
		return nil, fmt.Errorf("failed to get sport %d for logo upload: %w", sportID, err)
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat
	}

	if s.uploader == nil {
		return nil, errors.New("file uploader is not configured for sport service")
	}

	oldLogoKey := sport.LogoKey

	ext, err := GetExtensionFromContentType(contentType) // Используем общую функцию GetExtensionFromContentType
	if err != nil {
		return nil, err // err уже будет ErrCouldNotDetermineFileExtension
	}

	newKey := fmt.Sprintf("%s/%d/logo_%d%s", sportLogoPrefix, sportID, time.Now().UnixNano(), ext)

	_, err = s.uploader.Upload(ctx, newKey, contentType, file)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrSportLogoUploadFailed, newKey, err)
	}

	err = s.sportRepo.UpdateLogoKey(ctx, sportID, &newKey)
	if err != nil {
		if deleteErr := s.uploader.Delete(context.Background(), newKey); deleteErr != nil {
			fmt.Printf("CRITICAL: Failed to delete uploaded sport logo %s after DB update error: %v. DB error: %v\n", newKey, deleteErr, err)
		}
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrSportNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrSportLogoUpdateDBFailed, err)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete old sport logo %s: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}

	sport.LogoKey = &newKey
	s.populateSportLogoURL(sport)
	return sport, nil
}
