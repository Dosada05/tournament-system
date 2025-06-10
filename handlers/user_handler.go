package handlers

import (
	"errors"
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

	if input.FirstName == nil && input.LastName == nil && input.Nickname == nil && input.Email == nil && input.Password == nil {
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

func (h *UserHandler) UploadUserLogo(w http.ResponseWriter, r *http.Request) {
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

	if requestedUserID != currentUserID {
		forbiddenResponse(w, r, "operation not allowed for the current user")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		badRequestResponse(w, r, errors.New("content type required"))
		return
	}

	user, err := h.userService.UpdateUserLogo(r.Context(), requestedUserID, currentUserID, file, contentType)
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

func getUserIDFromURL(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		return 0, errors.New("missing user ID in URL path")
	}

	userID, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errors.New("invalid user ID format")
	}

	if userID <= 0 {
		return 0, errors.New("invalid user ID value")
	}

	return userID, nil
}
