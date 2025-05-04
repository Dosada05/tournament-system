package handlers

import (
	"errors"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type InviteHandler struct {
	inviteService services.InviteService
}

func NewInviteHandler(is services.InviteService) *InviteHandler {
	return &InviteHandler{
		inviteService: is,
		// baseURL: baseURL,
	}
}

func (h *InviteHandler) CreateOrRenewInviteHandler(w http.ResponseWriter, r *http.Request) {
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

	invite, err := h.inviteService.CreateOrRenewInvite(r.Context(), teamID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{
		"invite": map[string]interface{}{
			"team_id":    invite.TeamID,
			"expires_at": invite.ExpiresAt,
		},
		"invite_token": invite.Token,
	}

	if err := writeJSON(w, http.StatusCreated, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *InviteHandler) GetTeamInviteHandler(w http.ResponseWriter, r *http.Request) {
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

	invite, err := h.inviteService.GetTeamInvite(r.Context(), teamID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{
		"invite": map[string]interface{}{
			"team_id":    invite.TeamID,
			"expires_at": invite.ExpiresAt,
		},
		"invite_token": invite.Token,
	}

	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *InviteHandler) RevokeInviteHandler(w http.ResponseWriter, r *http.Request) {
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

	err = h.inviteService.RevokeInvite(r.Context(), teamID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InviteHandler) JoinTeamHandler(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		badRequestResponse(w, r, errors.New("missing invite token in URL path"))
		return
	}

	joiningUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required to join a team")
		return
	}

	joinedTeam, err := h.inviteService.ValidateAndJoinTeam(r.Context(), token, joiningUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{
		"message": "Successfully joined team",
		"team":    joinedTeam,
	}

	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
