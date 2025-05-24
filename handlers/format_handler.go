package handlers

import (
	"encoding/json" // Для UpdateFormatInput, если будем использовать json.RawMessage
	"errors"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"log"
	"net/http"
)

type FormatHandler struct {
	formatService services.FormatService
}

func NewFormatHandler(fs services.FormatService) *FormatHandler {
	return &FormatHandler{
		formatService: fs,
	}
}

// CreateFormat godoc
// @Summary Создать новый формат турнира
// @Tags formats
// @Description Создает новый формат турнира. Доступно только администраторам.
// @Accept json
// @Produce json
// @Param body body services.CreateFormatInput true "Данные для создания формата (включая name, bracket_type, participant_type, settings_json)"
// @Success 201 {object} map[string]interface{} "Формат создан"
// @Failure 400 {object} map[string]string "Ошибка валидации"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав (не админ)"
// @Failure 409 {object} map[string]string "Конфликт (например, имя уже занято)"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Security BearerAuth
// @Router /formats [post]
func (h *FormatHandler) CreateFormat(w http.ResponseWriter, r *http.Request) {
	var input services.CreateFormatInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// Валидацию входных данных (например, через go-playground/validator) лучше делать здесь или в сервисе.
	// Для примера:
	if input.Name == "" { // Предполагаем, что CreateFormatInput содержит Name
		badRequestResponse(w, r, errors.New("format name is required"))
		return
	}
	if input.BracketType == "" {
		badRequestResponse(w, r, errors.New("format bracket_type is required"))
		return
	}
	if input.ParticipantType == "" {
		badRequestResponse(w, r, errors.New("format participant_type is required"))
		return
	}

	format, err := h.formatService.CreateFormat(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Используем существующий маппер ошибок
		return
	}

	// Для ответа клиенту, можно сразу распарсить settings, если они есть
	var responseSettings interface{}
	if format.SettingsJSON != nil && *format.SettingsJSON != "" {
		if format.BracketType == "RoundRobin" {
			var rrSettings models.RoundRobinSettings
			if errJson := json.Unmarshal([]byte(*format.SettingsJSON), &rrSettings); errJson == nil {
				responseSettings = rrSettings
			} else {
				log.Printf("Warning: could not unmarshal RoundRobinSettings for created format %d: %v", format.ID, errJson)
				// Можно вернуть settings_json как строку или пустой объект
				_ = json.Unmarshal([]byte(*format.SettingsJSON), &responseSettings) // Попытка как generic map
			}
		} else {
			// Для других типов просто пытаемся анмаршалить в map[string]interface{}
			_ = json.Unmarshal([]byte(*format.SettingsJSON), &responseSettings)
		}
	}

	responsePayload := map[string]interface{}{
		"id":               format.ID,
		"name":             format.Name,
		"bracket_type":     format.BracketType,
		"participant_type": format.ParticipantType,
		"settings_json":    format.SettingsJSON, // Можно вернуть и сырой JSON
		"parsed_settings":  responseSettings,    // И распарсенные настройки
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"format": responsePayload}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// GetFormatByID godoc
// @Summary Получить формат по ID
// @Tags formats
// @Description Возвращает информацию о формате по его ID.
// @Produce json
// @Param formatID path int true "Format ID"
// @Success 200 {object} map[string]interface{} "Формат найден"
// @Failure 400 {object} map[string]string "Некорректный ID"
// @Failure 404 {object} map[string]string "Формат не найден"
// @Router /formats/{formatID} [get]
func (h *FormatHandler) GetFormatByID(w http.ResponseWriter, r *http.Request) {
	formatID, err := getIDFromURL(r, "formatID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	format, err := h.formatService.GetFormatByID(r.Context(), formatID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	var responseSettings interface{}
	if format.SettingsJSON != nil && *format.SettingsJSON != "" {
		if format.BracketType == "RoundRobin" {
			// Используем GetRoundRobinSettings из модели
			rrSettings, rrErr := format.GetRoundRobinSettings()
			if rrErr == nil && rrSettings != nil {
				responseSettings = rrSettings
			} else if rrErr != nil {
				log.Printf("Warning: could not parse RoundRobinSettings for format %d on get: %v", format.ID, rrErr)
				_ = json.Unmarshal([]byte(*format.SettingsJSON), &responseSettings) // Fallback
			}
		} else {
			_ = json.Unmarshal([]byte(*format.SettingsJSON), &responseSettings) // Fallback for other types
		}
	}

	responsePayload := map[string]interface{}{
		"id":               format.ID,
		"name":             format.Name,
		"bracket_type":     format.BracketType,
		"participant_type": format.ParticipantType,
		"settings_json":    format.SettingsJSON, // Можно вернуть и сырой JSON
		"parsed_settings":  responseSettings,    // И распарсенные настройки
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"format": responsePayload}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// GetAllFormats godoc
// @Summary Получить все форматы
// @Tags formats
// @Description Возвращает список всех доступных форматов турниров, включая распарсенные настройки.
// @Produce json
// @Success 200 {object} map[string]interface{} "Список форматов (каждый включая 'parsed_settings')"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Router /formats [get]
func (h *FormatHandler) GetAllFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := h.formatService.GetAllFormats(r.Context())
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	responseFormats := make([]map[string]interface{}, len(formats))
	for i, f := range formats {
		var parsedSettings interface{}
		if f.SettingsJSON != nil && *f.SettingsJSON != "" {
			if f.BracketType == "RoundRobin" {
				rrSettings, rrErr := f.GetRoundRobinSettings()
				if rrErr == nil && rrSettings != nil {
					parsedSettings = rrSettings
				} else if rrErr != nil {
					log.Printf("Warning: could not parse RoundRobinSettings for format %d in list: %v", f.ID, rrErr)
					_ = json.Unmarshal([]byte(*f.SettingsJSON), &parsedSettings)
				}
			} else {
				_ = json.Unmarshal([]byte(*f.SettingsJSON), &parsedSettings)
			}
		}
		responseFormats[i] = map[string]interface{}{
			"id":               f.ID,
			"name":             f.Name,
			"bracket_type":     f.BracketType,
			"participant_type": f.ParticipantType,
			"settings_json":    f.SettingsJSON,
			"parsed_settings":  parsedSettings,
		}
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"formats": responseFormats}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateFormat godoc
// @Summary Обновить формат турнира
// @Tags formats
// @Description Обновляет существующий формат турнира. Доступно только администраторам.
// @Accept json
// @Produce json
// @Param formatID path int true "Format ID"
// @Param body body services.UpdateFormatInput true "Данные для обновления формата (поля опциональны)"
// @Success 200 {object} map[string]interface{} "Формат обновлен (включая 'parsed_settings')"
// @Failure 400 {object} map[string]string "Ошибка валидации или некорректный ID"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав (не админ)"
// @Failure 404 {object} map[string]string "Формат не найден"
// @Failure 409 {object} map[string]string "Конфликт (например, имя уже занято)"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Security BearerAuth
// @Router /formats/{formatID} [put]
func (h *FormatHandler) UpdateFormat(w http.ResponseWriter, r *http.Request) {
	formatID, err := getIDFromURL(r, "formatID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	var input services.UpdateFormatInput
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// Проверка, что хотя бы одно поле передано (если все поля в UpdateFormatInput - указатели)
	if input.Name == nil && input.BracketType == nil && input.ParticipantType == nil && input.SettingsJSON == nil {
		badRequestResponse(w, r, errors.New("at least one field must be provided for update"))
		return
	}

	updatedFormat, err := h.formatService.UpdateFormat(r.Context(), formatID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	var responseSettings interface{}
	if updatedFormat.SettingsJSON != nil && *updatedFormat.SettingsJSON != "" {
		if updatedFormat.BracketType == "RoundRobin" {
			rrSettings, rrErr := updatedFormat.GetRoundRobinSettings()
			if rrErr == nil && rrSettings != nil {
				responseSettings = rrSettings
			} else if rrErr != nil {
				log.Printf("Warning: could not parse RoundRobinSettings for updated format %d: %v", updatedFormat.ID, rrErr)
				_ = json.Unmarshal([]byte(*updatedFormat.SettingsJSON), &responseSettings)
			}
		} else {
			_ = json.Unmarshal([]byte(*updatedFormat.SettingsJSON), &responseSettings)
		}
	}
	responsePayload := map[string]interface{}{
		"id":               updatedFormat.ID,
		"name":             updatedFormat.Name,
		"bracket_type":     updatedFormat.BracketType,
		"participant_type": updatedFormat.ParticipantType,
		"settings_json":    updatedFormat.SettingsJSON,
		"parsed_settings":  responseSettings,
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"format": responsePayload}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// DeleteFormat godoc
// @Summary Удалить формат турнира
// @Tags formats
// @Description Удаляет формат турнира. Доступно только администраторам.
// @Produce json
// @Param formatID path int true "Format ID"
// @Success 204 "Формат удален"
// @Failure 400 {object} map[string]string "Некорректный ID"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав (не админ)"
// @Failure 404 {object} map[string]string "Формат не найден"
// @Failure 409 {object} map[string]string "Конфликт (формат используется турнирами)"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Security BearerAuth
// @Router /formats/{formatID} [delete]
func (h *FormatHandler) DeleteFormat(w http.ResponseWriter, r *http.Request) {
	formatID, err := getIDFromURL(r, "formatID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	err = h.formatService.DeleteFormat(r.Context(), formatID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
