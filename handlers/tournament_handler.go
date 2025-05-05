package handlers

import (
	"errors"
	"net/http"
	"strconv" // Для парсинга query параметров

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models" // Для статусов
	"github.com/Dosada05/tournament-system/services"
)

type TournamentHandler struct {
	tournamentService services.TournamentService
	// Добавить validator, если используется
}

func NewTournamentHandler(ts services.TournamentService) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: ts,
	}
}

// createHandler обрабатывает POST /tournaments
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

	// Здесь можно добавить валидацию input с помощью validator, если он есть

	tournament, err := h.tournamentService.CreateTournament(r.Context(), currentUserID, input)
	if err != nil {
		mapTournamentServiceErrorToHTTP(w, r, err) // Используем специфичный маппер
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// getByIDHandler обрабатывает GET /tournaments/{tournamentID}
func (h *TournamentHandler) GetByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	tournament, err := h.tournamentService.GetTournamentByID(r.Context(), id)
	if err != nil {
		mapTournamentServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// listHandler обрабатывает GET /tournaments
func (h *TournamentHandler) ListHandler(w http.ResponseWriter, r *http.Request) {
	// Парсинг query параметров для фильтрации
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
		status := models.TournamentStatus(statusStr)
		// Проверка, валиден ли статус (можно использовать isValidTournamentStatus из сервиса)
		// if !isValidTournamentStatus(status) { ... }
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
	}

	tournaments, err := h.tournamentService.ListTournaments(r.Context(), filter)
	if err != nil {
		mapTournamentServiceErrorToHTTP(w, r, err)
		return
	}

	// Возвращаем список (даже если он пустой)
	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournaments": tournaments}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// updateDetailsHandler обрабатывает PUT /tournaments/{tournamentID}
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

	// Валидация input с помощью validator...

	tournament, err := h.tournamentService.UpdateTournamentDetails(r.Context(), id, currentUserID, input)
	if err != nil {
		mapTournamentServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// updateStatusHandler обрабатывает PATCH /tournaments/{tournamentID}/status
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

	// Валидация statusInput.Status...

	tournament, err := h.tournamentService.UpdateTournamentStatus(r.Context(), id, currentUserID, statusInput.Status)
	if err != nil {
		mapTournamentServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// deleteHandler обрабатывает DELETE /tournaments/{tournamentID}
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
		mapTournamentServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent) // Успешное удаление
}

// mapTournamentServiceErrorToHTTP преобразует ошибки TournamentService в HTTP статусы.
// Важно: Эта функция должна быть реализована или расширена в вашем пакете handlers
// или общем пакете утилит API.
func mapTournamentServiceErrorToHTTP(w http.ResponseWriter, r *http.Request, err error) {
	// Используем errors.Is для проверки конкретных ошибок сервиса
	switch {
	case errors.Is(err, services.ErrTournamentNotFound):
		notFoundResponse(w, r)
	case errors.Is(err, services.ErrForbiddenOperation):
		forbiddenResponse(w, r, err.Error()) // Передаем причину
	case errors.Is(err, services.ErrTournamentNameRequired),
		errors.Is(err, services.ErrTournamentDatesRequired),
		errors.Is(err, services.ErrTournamentInvalidRegDate),
		errors.Is(err, services.ErrTournamentInvalidDateRange),
		errors.Is(err, services.ErrTournamentInvalidCapacity),
		errors.Is(err, services.ErrTournamentInvalidStatus),
		errors.Is(err, services.ErrTournamentInvalidStatusTransition):
		badRequestResponse(w, r, err) // Ошибки валидации -> 400
	case errors.Is(err, services.ErrTournamentSportNotFound),
		errors.Is(err, services.ErrTournamentFormatNotFound),
		errors.Is(err, services.ErrTournamentOrganizerNotFound):
		// Ошибка из-за неверного ID зависимости - тоже 400 Bad Request или 404?
		// 400 Bad Request кажется более подходящим, т.к. проблема во входных данных.
		badRequestResponse(w, r, err)
	case errors.Is(err, services.ErrTournamentNameConflict):
		conflictResponse(w, r, err.Error()) // Конфликт имени -> 409
	case errors.Is(err, services.ErrTournamentUpdateNotAllowed),
		errors.Is(err, services.ErrTournamentDeletionNotAllowed):
		// Попытка изменить/удалить в неверном состоянии -> 409 Conflict или 403 Forbidden?
		// 409 Conflict подходит, если ресурс существует, но операция невозможна в текущем состоянии.
		conflictResponse(w, r, err.Error())
	case errors.Is(err, services.ErrTournamentInUse):
		// Попытка удалить турнир, на который есть ссылки -> 409 Conflict
		conflictResponse(w, r, err.Error())
	default:
		// Все остальные ошибки считаем внутренними ошибками сервера
		serverErrorResponse(w, r, err)
	}
}
