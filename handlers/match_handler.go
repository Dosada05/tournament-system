package handlers

import "net/http"

func (h *TournamentHandler) ListTournamentSoloMatchesHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	matches, err := h.matchService.ListSoloMatchesByTournament(r.Context(), tournamentID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Используем общий маппер ошибок
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"solo_matches": matches}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *TournamentHandler) ListTournamentTeamMatchesHandler(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	matches, err := h.matchService.ListTeamMatchesByTournament(r.Context(), tournamentID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"team_matches": matches}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
