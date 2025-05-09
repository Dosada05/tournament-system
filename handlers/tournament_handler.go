package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
)

type TournamentHandler struct {
	tournamentService services.TournamentService
}

func NewTournamentHandler(ts services.TournamentService) *TournamentHandler {
	return &TournamentHandler{
		tournamentService: ts,
	}
}

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
		mapServiceErrorToHTTP(w, r, err) // Используем общий маппер
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TournamentHandler) GetByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	tournament, err := h.tournamentService.GetTournamentByID(r.Context(), id)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

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
		status := models.TournamentStatus(statusStr)
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
		filter.Limit = 20
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
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournaments": tournaments}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

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

	tournament, err := h.tournamentService.UpdateTournamentStatus(r.Context(), id, currentUserID, statusInput.Status)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"tournament": tournament}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

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

	file, header, err := r.FormFile("logo") // "logo" - имя поля в форме
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
		mapServiceErrorToHTTP(w, r, err) // Используем общий маппер
		return
	}

	response := jsonResponse{"tournament": tournament}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
