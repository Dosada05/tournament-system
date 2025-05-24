package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
)

type TournamentHandler struct {
	tournamentService services.TournamentService
	matchService      services.MatchService
}

func NewTournamentHandler(ts services.TournamentService, ms services.MatchService) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: ts,
		matchService:      ms,
	}
}

// CreateHandler создает новый турнир.
func (h *TournamentHandler) CreateHandler(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to create tournament")
		return
	}

	var input services.CreateTournamentInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	tournament, err := h.tournamentService.CreateTournament(r.Context(), currentUserID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// GetByIDHandler получает турнир по ID.
func (h *TournamentHandler) GetByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, _ := middleware.GetUserIDFromContext(r.Context()) // ID для возможной кастомизации ответа

	tournament, err := h.tournamentService.GetTournamentByID(r.Context(), id, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// ListHandler получает список турниров с фильтрацией.
func (h *TournamentHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	var filter services.ListTournamentsFilter
	query := r.URL.Query()

	if sportIDStr := query.Get("sport_id"); sportIDStr != "" {
		if id, err := strconv.Atoi(sportIDStr); err == nil && id > 0 {
			filter.SportID = &id
		} else {
			badRequestResponse(w, r, errors.New("invalid sport_id query parameter"))
			return
		}
	}
	if formatIDStr := query.Get("format_id"); formatIDStr != "" {
		if id, err := strconv.Atoi(formatIDStr); err == nil && id > 0 {
			filter.FormatID = &id
		} else {
			badRequestResponse(w, r, errors.New("invalid format_id query parameter"))
			return
		}
	}
	if organizerIDStr := query.Get("organizer_id"); organizerIDStr != "" {
		if id, err := strconv.Atoi(organizerIDStr); err == nil && id > 0 {
			filter.OrganizerID = &id
		} else {
			badRequestResponse(w, r, errors.New("invalid organizer_id query parameter"))
			return
		}
	}
	if statusStr := query.Get("status"); statusStr != "" {
		// Валидация значения статуса должна быть строже
		// Например, через switch или map
		validStatuses := map[models.TournamentStatus]bool{
			models.StatusSoon:         true,
			models.StatusRegistration: true,
			models.StatusActive:       true,
			models.StatusCompleted:    true,
			models.StatusCanceled:     true,
		}
		status := models.TournamentStatus(statusStr)
		if !validStatuses[status] {
			badRequestResponse(w, r, fmt.Errorf("invalid status query parameter: %s", statusStr))
			return
		}
		filter.Status = &status
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		} else {
			badRequestResponse(w, r, errors.New("invalid limit query parameter"))
			return
		}
	} else {
		filter.Limit = 20 // Значение по умолчанию
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		} else {
			badRequestResponse(w, r, errors.New("invalid offset query parameter"))
			return
		}
	} else {
		filter.Offset = 0 // Значение по умолчанию
	}

	tournaments, err := h.tournamentService.ListTournaments(r.Context(), filter)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournaments": tournaments}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateDetailsHandler обновляет детали турнира.
func (h *TournamentHandler) UpdateDetailsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to update tournament")
		return
	}

	var input services.UpdateTournamentDetailsInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	tournament, err := h.tournamentService.UpdateTournamentDetails(r.Context(), id, currentUserID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateStatusHandler обновляет статус турнира.
func (h *TournamentHandler) UpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to update tournament status")
		return
	}

	var statusInput struct {
		Status models.TournamentStatus `json:"status" validate:"required"`
	}
	if err := readJSON(w, r, &statusInput); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	// Дополнительная валидация значения статуса
	if !isValidTournamentStatus(statusInput.Status) {
		badRequestResponse(w, r, fmt.Errorf("invalid status value: %s", statusInput.Status))
		return
	}

	tournament, err := h.tournamentService.UpdateTournamentStatus(r.Context(), id, currentUserID, statusInput.Status, nil)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// DeleteHandler удаляет турнир.
func (h *TournamentHandler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to delete tournament")
		return
	}

	err = h.tournamentService.DeleteTournament(r.Context(), id, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UploadTournamentLogoHandler загружает логотип для турнира.
func (h *TournamentHandler) UploadTournamentLogoHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user for logo upload")
		return
	}

	err = r.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("failed to parse multipart form: %w", err))
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("failed to get logo file from form: %w", err))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		badRequestResponse(w, r, errors.New("content-type header is required for logo"))
		return
	}

	tournament, err := h.tournamentService.UploadTournamentLogo(r.Context(), tournamentID, currentUserID, file, contentType)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"tournament": tournament}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateSoloMatchResultHandler godoc
// @Summary Обновить результат одиночного матча
// @Tags tournaments
// @Description Обновляет результат (счет и победителя) одиночного матча. Доступно организатору турнира.
// @Accept json
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Param matchID path int true "Solo Match ID"
// @Param body body services.UpdateMatchResultInput true "Данные результата матча (score, winner_participant_id - может быть null для ничьи)"
// @Success 200 {object} map[string]interface{} "Результат матча обновлен"
// @Failure 400 {object} map[string]string "Некорректные ID или данные результата"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав (не организатор) или турнир не активен"
// @Failure 404 {object} map[string]string "Турнир или матч не найден"
// @Failure 409 {object} map[string]string "Матч уже завершен или невалидный победитель"
// @Security BearerAuth
// @Router /tournaments/{tournamentID}/matches/solo/{matchID}/result [patch]
func (h *TournamentHandler) UpdateSoloMatchResultHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("invalid tournament ID: %w", err))
		return
	}
	matchID, err := getIDFromURL(r, "matchID")
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("invalid match ID: %w", err))
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to update match result")
		return
	}

	var input services.UpdateMatchResultInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	updatedMatch, err := h.matchService.UpdateSoloMatchResult(r.Context(), matchID, tournamentID, input, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	// Проверяем, не завершился ли турнир
	if updatedMatch.NextMatchDBID == nil && updatedMatch.WinnerParticipantID != nil {
		// Это был финальный матч, и он завершен
		log.Printf("Handler: Final solo match %d completed for tournament %d. Attempting to finalize tournament.", updatedMatch.ID, tournamentID)
		_, finalizeErr := h.tournamentService.FinalizeTournament(r.Context(), tournamentID, updatedMatch.WinnerParticipantID, currentUserID)
		if finalizeErr != nil {
			// Логируем ошибку финализации, но результат матча уже обновлен.
			// Клиент получит уведомление о завершении матча, а затем, возможно, об ошибке финализации.
			// Или, если это критично, можно вернуть ошибку клиенту.
			log.Printf("Error finalizing tournament %d after match %d: %v", tournamentID, updatedMatch.ID, finalizeErr)
			// Пока не возвращаем ошибку клиенту, чтобы он увидел результат матча.
			// mapServiceErrorToHTTP(w, r, finalizeErr)
			// return
		}
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"solo_match": updatedMatch}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateTeamMatchResultHandler godoc
// @Summary Обновить результат командного матча
// @Tags tournaments
// @Description Обновляет результат (счет и победителя) командного матча. Доступно организатору турнира.
// @Accept json
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Param matchID path int true "Team Match ID"
// @Param body body services.UpdateMatchResultInput true "Данные результата матча (score, winner_participant_id - может быть null для ничьи)"
// @Success 200 {object} map[string]interface{} "Результат матча обновлен"
// @Failure 400 {object} map[string]string "Некорректные ID или данные результата"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав (не организатор) или турнир не активен"
// @Failure 404 {object} map[string]string "Турнир или матч не найден"
// @Failure 409 {object} map[string]string "Матч уже завершен или невалидный победитель"
// @Security BearerAuth
// @Router /tournaments/{tournamentID}/matches/team/{matchID}/result [patch]
func (h *TournamentHandler) UpdateTeamMatchResultHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("invalid tournament ID: %w", err))
		return
	}
	matchID, err := getIDFromURL(r, "matchID")
	if err != nil {
		badRequestResponse(w, r, fmt.Errorf("invalid match ID: %w", err))
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to update match result")
		return
	}

	var input services.UpdateMatchResultInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	updatedMatch, err := h.matchService.UpdateTeamMatchResult(r.Context(), matchID, tournamentID, input, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if updatedMatch.NextMatchDBID == nil && updatedMatch.WinnerParticipantID != nil {
		log.Printf("Handler: Final team match %d completed for tournament %d. Attempting to finalize tournament.", updatedMatch.ID, tournamentID)
		_, finalizeErr := h.tournamentService.FinalizeTournament(r.Context(), tournamentID, updatedMatch.WinnerParticipantID, currentUserID)
		if finalizeErr != nil {
			log.Printf("Error finalizing tournament %d after match %d: %v", tournamentID, updatedMatch.ID, finalizeErr)
		}
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"team_match": updatedMatch}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// GetTournamentBracketHandler godoc
// @Summary Получить данные сетки/таблицы турнира
// @Tags tournaments
// @Description Возвращает полную структуру сетки (для SingleElimination) или список матчей и турнирную таблицу (для RoundRobin).
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Success 200 {object} services.FullTournamentBracketView "Данные сетки/таблицы турнира"
// @Failure 400 {object} map[string]string "Некорректный ID"
// @Failure 404 {object} map[string]string "Турнир или его формат не найден"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Router /tournaments/{tournamentID}/bracket [get]
func (h *TournamentHandler) GetTournamentBracketHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	bracketData, err := h.tournamentService.GetTournamentBracketData(r.Context(), tournamentID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	// Для отладки можно вывести, что именно отправляется
	// responseBytes, _ := json.MarshalIndent(bracketData, "", "  ")
	// log.Printf("GetTournamentBracketHandler response: %s", string(responseBytes))

	if err := writeJSON(w, http.StatusOK, bracketData, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func isValidTournamentStatus(status models.TournamentStatus) bool {
	switch status {
	case models.StatusSoon, models.StatusRegistration, models.StatusActive, models.StatusCompleted, models.StatusCanceled:
		return true
	}
	return false
}
