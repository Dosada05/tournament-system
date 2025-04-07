package controllers

import (
	"encoding/json"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func CreateTournament(w http.ResponseWriter, r *http.Request) {
	var tournament models.Tournament
	if err := json.NewDecoder(r.Body).Decode(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := services.CreateTournament(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func GetTournament(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	tournament, err := services.GetTournamentByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(tournament)
}

func UpdateTournament(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	var tournament models.Tournament
	if err := json.NewDecoder(r.Body).Decode(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := services.UpdateTournament(id, &tournament); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteTournament(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	if err := services.DeleteTournament(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
