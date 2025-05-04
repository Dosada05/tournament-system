package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/services"
	// models не нужны напрямую, т.к. авторизация в сервисе
	// "github.com/Dosada05/tournament-system/models"

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
	err := readJSON(w, r, &input)
	if err != nil {
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

	response := jsonResponse{
		"team": team,
	}

	err = writeJSON(w, http.StatusCreated, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) GetTeamByID(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID") // Используем общий хелпер
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	team, err := h.teamService.GetTeamByID(r.Context(), teamID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{
		"team": team,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
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
		members[i].Team = nil // Убираем вложенность
	}

	response := jsonResponse{
		"members": members,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
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
	err = readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	if input.Name == nil && input.SportID == nil {
		badRequestResponse(w, r, errors.New("no fields provided for update"))
		return
	}

	updatedTeam, err := h.teamService.UpdateTeamDetails(r.Context(), teamID, input, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Сервис вернет ErrCaptainActionForbidden, если нужно
		return
	}

	response := jsonResponse{
		"team": updatedTeam,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := getIDFromURL(r, "teamID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	userIDToAdd, err := getIDFromURL(r, "userID") // Получаем ID пользователя из URL
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}

	err = h.teamService.AddMember(r.Context(), teamID, userIDToAdd, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Сервис вернет нужные ошибки (403, 404, 409)
		return
	}

	// Успешное добавление, возвращаем 200 OK без тела или 204 No Content
	w.WriteHeader(http.StatusOK)
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

	err = h.teamService.RemoveMember(r.Context(), teamID, userIDToRemove, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Сервис вернет нужные ошибки (403, 404)
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

	err = h.teamService.DeleteTeam(r.Context(), teamID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Сервис вернет 403 или 400/409 (если не пустая)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Общая вспомогательная функция для извлечения ID из URL
func getIDFromURL(r *http.Request, paramName string) (int, error) {
	idStr := chi.URLParam(r, paramName)
	if idStr == "" {
		// Попробуем общий "id", если специфичный параметр не найден
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
