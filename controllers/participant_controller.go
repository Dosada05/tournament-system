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

// ParticipantController инкапсулирует сервис участников.
type ParticipantController struct {
	service *services.ParticipantService
}

// NewParticipantController создаёт ParticipantController с внедрённым сервисом.
func NewParticipantController(service *services.ParticipantService) *ParticipantController {
	return &ParticipantController{service: service}
}

// RegisterUser добавляет пользователя как участника турнира.
// POST /participants/user
func (c *ParticipantController) RegisterUser(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		TournamentID int `json:"tournament_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.service.RegisterUserAsParticipant(r.Context(), int(userID), req.TournamentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

// RegisterTeam добавляет команду как участника турнира.
// POST /participants/team
func (c *ParticipantController) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	//userClaims, ok := r.Context().Value("user").(jwt.MapClaims)
	//if !ok {
	//	http.Error(w, "Не удалось получить данные пользователя", http.StatusUnauthorized)
	//	return
	//}
	// Можно добавить валидацию, что пользователь — капитан команды

	var req struct {
		TeamID       int `json:"team_id"`
		TournamentID int `json:"tournament_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.service.RegisterTeamAsParticipant(r.Context(), req.TeamID, req.TournamentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

// ChangeParticipantStatus меняет статус участника (например, на "принят" или "отклонён").
// PUT /participants/{id}/status
func (c *ParticipantController) ChangeParticipantStatus(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id участника", http.StatusBadRequest)
		return
	}
	var req struct {
		Status models.ParticipantStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.service.ChangeParticipantStatus(r.Context(), id, req.Status); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// GetByID возвращает участника по ID.
// GET /participants/{id}
func (c *ParticipantController) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id участника", http.StatusBadRequest)
		return
	}
	participant, err := c.service.GetParticipantByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(participant)
}

// ListByTournament возвращает список участников по турниру.
// GET /participants?tournament_id=123
func (c *ParticipantController) ListByTournament(w http.ResponseWriter, r *http.Request) {
	tournamentIDStr := r.URL.Query().Get("tournament_id")
	tournamentID, err := strconv.Atoi(tournamentIDStr)
	if err != nil || tournamentID <= 0 {
		http.Error(w, "Некорректный id турнира", http.StatusBadRequest)
		return
	}
	participants, err := c.service.ListParticipantsByTournament(r.Context(), tournamentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(participants)
}

// Delete удаляет участника по ID.
// DELETE /participants/{id}
func (c *ParticipantController) Delete(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id участника", http.StatusBadRequest)
		return
	}
	if err := c.service.DeleteParticipant(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
