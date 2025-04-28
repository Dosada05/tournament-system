package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5"
)

// TournamentController инкапсулирует сервис турниров и реализует методы-обработчики.
type TournamentController struct {
	service *services.TournamentService
}

// NewTournamentController конструктор для внедрения зависимости.
func NewTournamentController(service *services.TournamentService) *TournamentController {
	return &TournamentController{service: service}
}

// CreateTournament создает турнир с учетом организатора из JWT.
func (c *TournamentController) CreateTournament(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := r.Context().Value("user").(jwt.MapClaims)
	if !ok {
		http.Error(w, "Не удалось получить данные пользователя", http.StatusUnauthorized)
		return
	}
	userID, ok := userClaims["id"].(float64)
	if !ok {
		http.Error(w, "Некорректные данные пользователя", http.StatusBadRequest)
		return
	}
	var tournament models.Tournament
	if err := json.NewDecoder(r.Body).Decode(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tournament.OrganizerID = int(userID)

	if err := c.service.CreateTournament(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tournament)
}

// GetTournament возвращает турнир по ID.
func (c *TournamentController) GetTournament(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}
	tournament, err := c.service.GetTournamentByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tournament)
}

// GetAllTournaments возвращает список турниров с пагинацией.
func (c *TournamentController) GetAllTournaments(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	var (
		limit, offset int
		err           error
	)
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 10
		}
	} else {
		limit = 10
	}
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
	} else {
		offset = 0
	}

	tournaments, err := c.service.GetAllTournaments(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tournaments)
}

// UpdateTournament обновляет турнир (проверяет, что пользователь — организатор).
func (c *TournamentController) UpdateTournament(w http.ResponseWriter, r *http.Request) {
	userClaims, ok := r.Context().Value("user").(jwt.MapClaims)
	if !ok {
		http.Error(w, "Не удалось получить данные пользователя", http.StatusUnauthorized)
		return
	}
	userID, ok := userClaims["id"].(float64)
	if !ok {
		http.Error(w, "Некорректные данные пользователя", http.StatusBadRequest)
		return
	}
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Неверный ID турнира", http.StatusBadRequest)
		return
	}
	tournament, err := c.service.GetTournamentByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if tournament.OrganizerID != int(userID) {
		http.Error(w, "У вас нет прав на редактирование этого турнира", http.StatusForbidden)
		return
	}
	var updatedTournament models.Tournament
	if err := json.NewDecoder(r.Body).Decode(&updatedTournament); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.service.UpdateTournament(id, &updatedTournament); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DeleteTournament удаляет турнир по ID.
func (c *TournamentController) DeleteTournament(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}
	if err := c.service.DeleteTournament(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
