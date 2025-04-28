package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/go-chi/chi/v5"
)

// SportController инкапсулирует сервис спорта для работы с видами спорта.
type SportController struct {
	service *services.SportService
}

// NewSportController создает новый контроллер спорта с внедренным сервисом.
func NewSportController(service *services.SportService) *SportController {
	return &SportController{service: service}
}

// CreateSport создает новый вид спорта.
func (c *SportController) CreateSport(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var sport models.Sport
	if err := json.NewDecoder(r.Body).Decode(&sport); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.service.CreateSport(&sport); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sport)
}

// GetAllSports возвращает все виды спорта.
func (c *SportController) GetAllSports(w http.ResponseWriter, r *http.Request) {
	sports, err := c.service.GetAllSports()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sports)
}

// GetSport возвращает вид спорта по ID.
func (c *SportController) GetSport(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid sport ID", http.StatusBadRequest)
		return
	}
	sport, err := c.service.GetSportByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sport)
}

// UpdateSport обновляет существующий вид спорта.
func (c *SportController) UpdateSport(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid sport ID", http.StatusBadRequest)
		return
	}
	var sport models.Sport
	if err := json.NewDecoder(r.Body).Decode(&sport); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.service.UpdateSport(id, &sport); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sport)
}

// DeleteSport удаляет вид спорта по ID.
func (c *SportController) DeleteSport(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid sport ID", http.StatusBadRequest)
		return
	}
	if err := c.service.DeleteSport(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
