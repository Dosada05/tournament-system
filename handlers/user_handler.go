package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	userService services.UserService
}

func NewUserHandler(us services.UserService) *UserHandler {
	return &UserHandler{
		userService: us,
	}
}

func (h *UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	requestedUserID, err := getUserIDFromURL(r)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	user, err := h.userService.GetProfileByID(r.Context(), requestedUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	user.PasswordHash = ""

	response := jsonResponse{
		"user": user,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *UserHandler) UpdateUserByID(w http.ResponseWriter, r *http.Request) {
	requestedUserID, err := getUserIDFromURL(r)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user")
		return
	}

	currentUserRole, err := middleware.GetUserRoleFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "failed to identify current user role")
		return
	}

	isAllowed := (requestedUserID == currentUserID) || (currentUserRole == models.RoleAdmin)
	if !isAllowed {
		forbiddenResponse(w, r, "operation not allowed for the current user")
		return
	}

	var input services.UpdateProfileInput
	err = readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	if input.FirstName == nil && input.LastName == nil && input.Nickname == nil {
		badRequestResponse(w, r, errors.New("no fields provided for update"))
		return
	}

	updatedUser, err := h.userService.UpdateProfile(r.Context(), requestedUserID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	updatedUser.PasswordHash = ""

	response := jsonResponse{
		"user": updatedUser,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

func getUserIDFromURL(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		return 0, errors.New("missing user ID in URL path")
	}

	userID, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID format: %q", idStr)
	}

	if userID <= 0 {
		return 0, fmt.Errorf("invalid user ID value: %d", userID)
	}

	return userID, nil
}
