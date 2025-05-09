package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	// "github.com/go-chi/chi/v5" // Если getIDFromURL будет использоваться
)

type SportHandler struct {
	sportService services.SportService
}

func NewSportHandler(ss services.SportService) *SportHandler {
	return &SportHandler{
		sportService: ss,
	}
}

func (h *SportHandler) CreateSport(w http.ResponseWriter, r *http.Request) {
	currentUserRole, err := middleware.GetUserRoleFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user role")
		return
	}
	if currentUserRole != models.RoleAdmin {
		forbiddenResponse(w, r, "admin privileges required to create sport")
		return
	}

	var input services.CreateSportInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	sport, err := h.sportService.CreateSport(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"sport": sport}
	if err := writeJSON(w, http.StatusCreated, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *SportHandler) GetSportByID(w http.ResponseWriter, r *http.Request) {
	sportID, err := getIDFromURL(r, "sportID") // Предполагаем, что getIDFromURL доступна
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	sport, err := h.sportService.GetSportByID(r.Context(), sportID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"sport": sport}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *SportHandler) GetAllSports(w http.ResponseWriter, r *http.Request) {
	sports, err := h.sportService.GetAllSports(r.Context())
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"sports": sports}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *SportHandler) UpdateSport(w http.ResponseWriter, r *http.Request) {
	currentUserRole, err := middleware.GetUserRoleFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user role")
		return
	}
	if currentUserRole != models.RoleAdmin {
		forbiddenResponse(w, r, "admin privileges required to update sport")
		return
	}

	sportID, err := getIDFromURL(r, "sportID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	var input services.UpdateSportInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	updatedSport, err := h.sportService.UpdateSport(r.Context(), sportID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"sport": updatedSport}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *SportHandler) DeleteSport(w http.ResponseWriter, r *http.Request) {
	currentUserRole, err := middleware.GetUserRoleFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user role")
		return
	}
	if currentUserRole != models.RoleAdmin {
		forbiddenResponse(w, r, "admin privileges required to delete sport")
		return
	}

	sportID, err := getIDFromURL(r, "sportID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	err = h.sportService.DeleteSport(r.Context(), sportID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SportHandler) UploadSportLogoHandler(w http.ResponseWriter, r *http.Request) {
	currentUserRole, err := middleware.GetUserRoleFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user role")
		return
	}
	if currentUserRole != models.RoleAdmin {
		forbiddenResponse(w, r, "admin privileges required to upload sport logo")
		return
	}

	sportID, err := getIDFromURL(r, "sportID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user for logo upload")
		return
	}

	err = r.ParseMultipartForm(32 << 20)
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

	sport, err := h.sportService.UploadSportLogo(r.Context(), sportID, currentUserID, file, contentType)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"sport": sport}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
