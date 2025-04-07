package controllers

import (
	"encoding/json"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func RegisterParticipant(w http.ResponseWriter, r *http.Request) {
	var participant models.Participant
	if err := json.NewDecoder(r.Body).Decode(&participant); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := services.RegisterParticipant(&participant); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func GetParticipants(w http.ResponseWriter, r *http.Request) {
	tournamentIDParam := chi.URLParam(r, "tournamentId")
	tournamentID, err := strconv.Atoi(tournamentIDParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	participants, err := services.GetParticipantsByTournamentID(tournamentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(participants)
}
