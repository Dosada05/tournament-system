package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
)

type TeamHandler struct {
	teamService services.TeamService
	userService services.UserService
}

func NewTeamHandler(ts services.TeamService, us services.UserService) *TeamHandler {
	return &TeamHandler{
		teamService: ts,
		userService: us,
	}
}

func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var input services.CreateTeamInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}
	input.CreatorID = currentUserID
	team, err := h.teamService.CreateTeam(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	response := jsonResponse{"team": team}
	if err := writeJSON(w, http.StatusCreated, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) GetTeamByID(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	team, err := h.teamService.GetTeamByID(r.Context(), teamID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	response := jsonResponse{"team": team}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	members, err := h.userService.ListUsersByTeamID(r.Context(), teamID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	for i := range members {
		members[i].PasswordHash = ""
		members[i].Team = nil
	}
	response := jsonResponse{"members": members}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) UpdateTeamDetails(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}
	var input services.UpdateTeamInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	if input.Name == nil && input.SportID == nil {
		badRequestResponse(w, r, errors.New("no fields provided for update"))
		return
	}
	updatedTeam, err := h.teamService.UpdateTeamDetails(r.Context(), teamID, input, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	response := jsonResponse{"team": updatedTeam}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	userIDToRemove, err := getIDFromURL(r, "userID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}
	if err := h.teamService.RemoveMember(r.Context(), teamID, userIDToRemove, currentUserID); err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TeamHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}
	if err := h.teamService.DeleteTeam(r.Context(), teamID, currentUserID); err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TeamHandler) UploadTeamLogo(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
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

	team, err := h.teamService.UploadLogo(r.Context(), teamID, currentUserID, file, contentType)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{"team": team}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func getIDFromURL(r *http.Request, paramName string) (int, error) {
	idStr := chi.URLParam(r, paramName)
	if idStr == "" {
		idStr = chi.URLParam(r, "id")
		if idStr == "" {
			return 0, fmt.Errorf("missing %s or id in URL path", paramName)
		}
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s format: %q", paramName, idStr)
	}
	if id <= 0 {
		return 0, fmt.Errorf("invalid %s value: %d", paramName, id)
	}
	return id, nil
}
