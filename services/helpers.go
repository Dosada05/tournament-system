// File: tournament-system/services/helpers.go
package services

import (
	"context" // Нужен для populateTournamentDetails, если он останется здесь
	"fmt"
	"log/slog" // Для функций, которые могут логировать
	"strconv"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories" // Нужен для handleRepositoryError
	"github.com/Dosada05/tournament-system/storage"      // Нужен для populate... функций
)

// --- Общие хелперы ---

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func validateTournamentDates(reg, start, end time.Time) error {
	if reg.IsZero() || start.IsZero() || end.IsZero() {
		// Используем ошибку, определенную в tournament_service.go или errors.go
		return ErrTournamentDatesRequired
	}
	if reg.After(start) {
		// Используем ошибку, определенную в tournament_service.go или errors.go
		return fmt.Errorf("%w: registration date (%s) cannot be after start date (%s)", ErrTournamentInvalidRegDate, reg.Format(time.RFC3339), start.Format(time.RFC3339))
	}
	if !start.Before(end) {
		// Используем ошибку, определенную в tournament_service.go или errors.go
		return fmt.Errorf("%w: start date (%s) must be before end date (%s)", ErrTournamentInvalidDateRange, start.Format(time.RFC3339), end.Format(time.RFC3339))
	}
	return nil
}

func isValidStatusTransition(current, next models.TournamentStatus) bool {
	if current == next {
		return true
	}
	allowedTransitions := map[models.TournamentStatus][]models.TournamentStatus{
		models.StatusSoon:         {models.StatusRegistration, models.StatusCanceled},
		models.StatusRegistration: {models.StatusActive, models.StatusCanceled},
		models.StatusActive:       {models.StatusCompleted, models.StatusCanceled},
		models.StatusCompleted:    {},
		models.StatusCanceled:     {},
	}
	for _, allowedNextStatus := range allowedTransitions[current] {
		if next == allowedNextStatus {
			return true
		}
	}
	return false
}

// handleRepositoryError - общий хелпер для ошибок репозитория

// --- Хелперы для преобразования моделей в DTO/View ---

func ParticipantsToInterface(slice []*models.Participant) []models.Participant {
	if slice == nil {
		return []models.Participant{}
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
		return []models.SoloMatch{}
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
		return []models.TeamMatch{}
	}
	result := make([]models.TeamMatch, len(slice))
	for i, ptr := range slice {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}

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

// --- Хелперы для заполнения URL логотипов и деталей (теперь это функции, а не методы) ---

func populateTournamentLogoURLFunc(tournament *models.Tournament, uploader storage.FileUploader) {
	if tournament != nil && tournament.LogoKey != nil && *tournament.LogoKey != "" && uploader != nil {
		url := uploader.GetPublicURL(*tournament.LogoKey)
		if url != "" {
			tournament.LogoURL = &url
		}
	}
}

func populateSportLogoURLFunc(sport *models.Sport, uploader storage.FileUploader) {
	if sport != nil && sport.LogoKey != nil && *sport.LogoKey != "" && uploader != nil {
		url := uploader.GetPublicURL(*sport.LogoKey)
		if url != "" {
			sport.LogoURL = &url
		}
	}
}

func populateUserDetailsFunc(user *models.User, uploader storage.FileUploader) {
	if user == nil {
		return
	}
	user.PasswordHash = "" // Важно для безопасности
	if user.LogoKey != nil && *user.LogoKey != "" && uploader != nil {
		url := uploader.GetPublicURL(*user.LogoKey)
		if url != "" {
			user.LogoURL = &url
		}
	}
}

func populateParticipantListDetailsFunc(participants []*models.Participant, uploader storage.FileUploader) {
	if uploader == nil {
		return
	}
	for _, p := range participants {
		if p == nil {
			continue
		}
		if p.User != nil {
			populateUserDetailsFunc(p.User, uploader) // Вызов обновленной функции
		}
		if p.Team != nil && p.Team.LogoKey != nil && *p.Team.LogoKey != "" {
			url := uploader.GetPublicURL(*p.Team.LogoKey)
			if url != "" {
				p.Team.LogoURL = &url
			}
		}
	}
}

// getParticipantDisplayNameFunc - версия для helpers.go
func getParticipantDisplayNameFunc(p *models.Participant) string {
	if p == nil {
		return "N/A"
	}
	if p.User != nil {
		if p.User.Nickname != nil && *p.User.Nickname != "" {
			return *p.User.Nickname
		}
		name := p.User.FirstName
		if p.User.LastName != "" {
			name += " " + p.User.LastName
		}
		if name != "" {
			return name
		}
	}
	if p.Team != nil && p.Team.Name != "" {
		return p.Team.Name
	}
	if p.ID != 0 {
		return fmt.Sprintf("Participant %d", p.ID)
	}
	return "Unnamed Participant"
}

// participantToParticipantViewFunc - версия для helpers.go
func participantToParticipantViewFunc(p *models.Participant, uploader storage.FileUploader) ParticipantView {
	view := ParticipantView{
		ParticipantDBID: p.ID,
		OriginalUserID:  p.UserID,
		OriginalTeamID:  p.TeamID,
		Name:            getParticipantDisplayNameFunc(p), // Используем локальный хелпер
	}
	if p.User != nil {
		view.Type = "user"
		if p.User.LogoKey != nil && uploader != nil {
			logoURL := uploader.GetPublicURL(*p.User.LogoKey)
			if logoURL != "" {
				view.LogoURL = &logoURL
			}
		}
	} else if p.Team != nil {
		view.Type = "team"
		if p.Team.LogoKey != nil && uploader != nil {
			logoURL := uploader.GetPublicURL(*p.Team.LogoKey)
			if logoURL != "" {
				view.LogoURL = &logoURL
			}
		}
	}
	return view
}

// toMatchViewFunc - версия для helpers.go
func toMatchViewFunc(sm *models.SoloMatch, tm *models.TeamMatch, participantsMap map[int]ParticipantView, logger *slog.Logger) MatchView {
	mv := MatchView{}
	var p1ID, p2ID, winnerID, nextMatchID, winnerSlot *int
	var roundVal int
	var bracketUID, scoreStr *string
	var matchTimeVal time.Time
	var statusVal models.MatchStatus

	if sm != nil {
		mv.MatchID = sm.ID
		bracketUID = sm.BracketMatchUID
		p1ID = sm.P1ParticipantID
		p2ID = sm.P2ParticipantID
		scoreStr = sm.Score
		statusVal = sm.Status
		winnerID = sm.WinnerParticipantID
		if sm.Round != nil {
			roundVal = *sm.Round
		} else {
			if logger != nil {
				logger.WarnContext(context.Background(), "Solo match with nil round", slog.Int("match_id", sm.ID))
			}
		}
		nextMatchID = sm.NextMatchDBID
		winnerSlot = sm.WinnerToSlot
		matchTimeVal = sm.MatchTime
	} else if tm != nil {
		mv.MatchID = tm.ID
		bracketUID = tm.BracketMatchUID
		p1ID = tm.T1ParticipantID
		p2ID = tm.T2ParticipantID
		scoreStr = tm.Score
		statusVal = tm.Status
		winnerID = tm.WinnerParticipantID
		if tm.Round != nil {
			roundVal = *tm.Round
		} else {
			if logger != nil {
				logger.WarnContext(context.Background(), "Team match with nil round", slog.Int("match_id", tm.ID))
			}
		}
		nextMatchID = tm.NextMatchDBID
		winnerSlot = tm.WinnerToSlot
		matchTimeVal = tm.MatchTime
	} else {
		return mv
	}

	mv.BracketMatchUID = bracketUID
	mv.Status = statusVal
	mv.Round = roundVal
	mv.ScoreString = scoreStr
	mv.WinnerParticipantDBID = winnerID
	mv.NextMatchDBID = nextMatchID
	mv.WinnerToSlot = winnerSlot
	mv.MatchTime = matchTimeVal

	if mv.BracketMatchUID != nil && mv.Round != 0 {
		prefix := "R" + strconv.Itoa(mv.Round) + "M"
		uidPart := strings.TrimPrefix(*mv.BracketMatchUID, prefix)
		orderPart := strings.SplitN(uidPart, "S", 2)[0]
		order, err := strconv.Atoi(orderPart)
		if err == nil && order > 0 {
			mv.OrderInRound = order
		}
	}
	if p1ID != nil {
		if pView, ok := participantsMap[*p1ID]; ok {
			mv.Participant1 = &pView
		} else if *p1ID != 0 {
			mv.Participant1 = &ParticipantView{ParticipantDBID: *p1ID, Name: fmt.Sprintf("Participant %d (Details Missing)", *p1ID)}
		}
	}
	if p2ID != nil {
		if pView, ok := participantsMap[*p2ID]; ok {
			mv.Participant2 = &pView
		} else if *p2ID != 0 {
			mv.Participant2 = &ParticipantView{ParticipantDBID: *p2ID, Name: fmt.Sprintf("Participant %d (Details Missing)", *p2ID)}
		}
	}
	return mv
}

// populateTournamentDetailsFunc - версия для helpers.go
// Требует передачи зависимостей репозиториев или изменения логики для их получения
func populateTournamentDetailsFunc(
	ctx context.Context,
	tournament *models.Tournament,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
	// participantRepo repositories.ParticipantRepository, // Если нужна загрузка участников здесь
	uploader storage.FileUploader,
	logger *slog.Logger,
) {
	if tournament == nil {
		return
	}
	populateTournamentLogoURLFunc(tournament, uploader)

	if tournament.Sport == nil && tournament.SportID > 0 {
		sport, err := sportRepo.GetByID(ctx, tournament.SportID)
		if err == nil && sport != nil {
			populateSportLogoURLFunc(sport, uploader)
			tournament.Sport = sport
		} else if err != nil {
			if logger != nil {
				logger.WarnContext(ctx, "Failed to populate sport details", slog.Int("tournament_id", tournament.ID), slog.Int("sport_id", tournament.SportID), slog.Any("error", err))
			}
		}
	}
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, err := formatRepo.GetByID(ctx, tournament.FormatID)
		if err == nil && format != nil {
			tournament.Format = format
		} else if err != nil {
			if logger != nil {
				logger.WarnContext(ctx, "Failed to populate format details", slog.Int("tournament_id", tournament.ID), slog.Int("format_id", tournament.FormatID), slog.Any("error", err))
			}
		}
	}
	if tournament.Organizer == nil && tournament.OrganizerID > 0 {
		organizer, err := userRepo.GetByID(ctx, tournament.OrganizerID)
		if err == nil && organizer != nil {
			populateUserDetailsFunc(organizer, uploader)
			tournament.Organizer = organizer
		} else if err != nil {
			if logger != nil {
				logger.WarnContext(ctx, "Failed to populate organizer details", slog.Int("tournament_id", tournament.ID), slog.Int("organizer_id", tournament.OrganizerID), slog.Any("error", err))
			}
		}
	}
}

// GetExtensionFromContentType (из services/user_service.go, можно сделать общим)
func GetExtensionFromContentType(contentType string) (string, error) {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	default:
		parts := strings.Split(contentType, "/")
		if len(parts) == 2 && strings.HasPrefix(parts[0], "image") && parts[1] != "" {
			// Убираем возможные суффиксы типа "+xml" (например, "image/svg+xml")
			ext := "." + strings.Split(parts[1], "+")[0]
			// Проверяем на известные, но не стандартные случаи
			if ext == ".svg" {
				return ext, nil
			} // Явно разрешаем svg, если нужно
			// Можно добавить другие проверки или просто вернуть то, что получилось
			// return "", fmt.Errorf("unsupported image content type for extension: %s", contentType) // Если строгая проверка
			return ext, nil // Если более мягкая
		}
		return "", fmt.Errorf("could not determine file extension from content type: '%s'", contentType)
	}
}
