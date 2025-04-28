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

// TeamController инкапсулирует сервис команд.
type TeamController struct {
	service *services.TeamService
}

// NewTeamController конструктор для внедрения зависимости.
func NewTeamController(service *services.TeamService) *TeamController {
	return &TeamController{service: service}
}

// @Summary Создать команду
// @Description Создаёт новую команду
// @Tags teams
// @Accept json
// @Produce json
// @Param team body models.Team true "Данные команды"
// @Success 201 {object} models.Team
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security ApiKeyAuth
// @Router /teams/ [post]
func (c *TeamController) CreateTeam(w http.ResponseWriter, r *http.Request) {
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

	var newTeam models.Team
	if err := json.NewDecoder(r.Body).Decode(&newTeam); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newTeam.CaptainID = int(userID)

	if err := c.service.CreateTeam(&newTeam); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTeam)
}

// @Summary Получить все команды
// @Description Возвращает список всех команд
// @Tags teams
// @Produce json
// @Success 200 {array} models.Team
// @Failure 500 {object} map[string]string
// @Router /teams/ [get]
func (c *TeamController) GetAllTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := c.service.GetAllTeams()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

// @Summary Получить команду по id
// @Description Возвращает команду по её id
// @Tags teams
// @Produce json
// @Param id path int true "ID команды"
// @Success 200 {object} models.Team
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /teams/{id} [get]
func (c *TeamController) GetTeamByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id команды", http.StatusBadRequest)
		return
	}
	team, err := c.service.GetTeamByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if team == nil {
		http.Error(w, "Команда не найдена", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// @Summary Обновить команду
// @Description Обновляет данные команды
// @Tags teams
// @Accept json
// @Produce json
// @Param id path int true "ID команды"
// @Param team body models.Team true "Данные команды"
// @Success 200 {object} models.Team
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security ApiKeyAuth
// @Router /teams/{id} [put]
func (c *TeamController) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id команды", http.StatusBadRequest)
		return
	}
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
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	team.CaptainID = int(userID)

	if err := c.service.UpdateTeam(id, &team); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// @Summary Удалить команду
// @Description Удаляет команду по id
// @Tags teams
// @Produce json
// @Param id path int true "ID команды"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security ApiKeyAuth
// @Router /teams/{id} [delete]
func (c *TeamController) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Некорректный id команды", http.StatusBadRequest)
		return
	}
	if err := c.service.DeleteTeam(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
