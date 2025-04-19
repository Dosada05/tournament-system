package controllers

import (
	"encoding/json"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func CreateTournament(w http.ResponseWriter, r *http.Request) {
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

	if err := services.CreateTournament(&tournament); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tournament)
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

func GetAllTournaments(w http.ResponseWriter, r *http.Request) {
	// Получаем параметры для пагинации из запроса (если они есть)
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	var limit, offset int
	var err error

	// Если параметр limit указан, преобразуем его в число
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 10 // Значение по умолчанию
		}
	} else {
		limit = 10
	}

	// Если параметр offset указан, преобразуем его в число
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0 // Значение по умолчанию
		}
	} else {
		offset = 0
	}

	// Получаем список турниров через сервисный слой
	tournaments, err := services.GetAllTournaments(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовок Content-Type
	w.Header().Set("Content-Type", "application/json")

	// Отправляем список турниров клиенту
	json.NewEncoder(w).Encode(tournaments)
}

func UpdateTournament(w http.ResponseWriter, r *http.Request) {
	// Получаем данные пользователя из JWT
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

	// Получаем ID турнира
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		http.Error(w, "Неверный ID турнира", http.StatusBadRequest)
		return
	}

	// Проверяем, является ли пользователь организатором турнира
	tournament, err := services.GetTournamentByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if tournament.OrganizerID != int(userID) {
		http.Error(w, "У вас нет прав на редактирование этого турнира", http.StatusForbidden)
		return
	}

	// Декодируем тело запроса
	var updatedTournament models.Tournament
	if err := json.NewDecoder(r.Body).Decode(&updatedTournament); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Обновляем турнир
	if err := services.UpdateTournament(id, &updatedTournament); err != nil {
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
