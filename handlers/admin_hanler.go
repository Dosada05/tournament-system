package handlers

import (
	"errors"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type AdminUserHandler struct {
	adminUserService services.AdminUserService
}

func NewAdminUserHandler(s services.AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{adminUserService: s}
}

func (h *AdminUserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.UserFilter{
		Search: q.Get("search"),
		Page:   toInt(q.Get("page"), 1),
		Limit:  toInt(q.Get("limit"), 20),
	}
	if role := q.Get("role"); role != "" {
		filter.Role = &role
	}
	if status := q.Get("status"); status != "" {
		filter.Status = &status
	}
	res, err := h.adminUserService.ListUsers(r.Context(), filter)
	if err != nil {
		serverErrorResponse(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res, nil)
}

func (h *AdminUserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(idStr)
	if err != nil || userID <= 0 {
		badRequestResponse(w, r, errors.New("invalid user id"))
		return
	}

	err = h.adminUserService.DeleteUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			notFoundResponse(w, r, "user not found")
			return
		}
		serverErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toInt(s string, def int) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return def
}
