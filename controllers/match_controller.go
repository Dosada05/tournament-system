package controllers

import (
	"encoding/json"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func CreateMatch(w http.ResponseWriter, r *http.Request) {
	var match models.Match
	if err := json.NewDecoder(r.Body).Decode(&match); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := services.CreateMatch(&match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func GetMatch(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid match ID", http.StatusBadRequest)
		return
	}

	match, err := services.GetMatchByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(match)
}

func UpdateMatch(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid match ID", http.StatusBadRequest)
		return
	}

	var match models.Match
	if err := json.NewDecoder(r.Body).Decode(&match); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := services.UpdateMatch(id, &match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteMatch(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid match ID", http.StatusBadRequest)
		return
	}

	if err := services.DeleteMatch(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
