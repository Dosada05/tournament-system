package handlers

import (
	"encoding/json" // Для UpdateFormatInput, если будем использовать json.RawMessage
	"errors"
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
// @Param body body services.CreateFormatInput true "Данные для создания формата"
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
	// В models.Format у нас сейчас есть Name, BracketType, ParticipantType, SettingsJSON
	// services.CreateFormatInput сейчас содержит только Name. Нужно будет его расширить.
	// Пока оставим так, подразумевая, что CreateFormatInput в сервисе будет доработан.
	// Для примера, предположим, что CreateFormatInput теперь выглядит так:
	// type CreateFormatInput struct {
	//    Name            string                 `json:"name" validate:"required"`
	//    BracketType     string                 `json:"bracket_type" validate:"required"`
	//    ParticipantType models.FormatParticipantType `json:"participant_type" validate:"required,oneof=solo team"`
	//    SettingsJSON    *string                `json:"settings_json"` // Может быть nil
	// }

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
	// Добавьте валидацию для BracketType, ParticipantType, SettingsJSON, если они есть в CreateFormatInput

	format, err := h.formatService.CreateFormat(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Используем существующий маппер ошибок
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"format": format}, nil); err != nil {
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
	formatID, err := getIDFromURL(r, "formatID") // Используем getIDFromURL из team_handler.go или helpers.go
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	format, err := h.formatService.GetFormatByID(r.Context(), formatID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	// Размаршаливаем SettingsJSON, если он есть, для ответа клиенту
	var settings map[string]interface{}
	if format.SettingsJSON != nil && *format.SettingsJSON != "" {
		if errJson := json.Unmarshal([]byte(*format.SettingsJSON), &settings); errJson != nil {
			// Логируем ошибку, но продолжаем, возможно, JSON некорректен в БД
			log.Printf("Warning: could not unmarshal SettingsJSON for format %d: %v", format.ID, errJson)
		}
	}

	responsePayload := map[string]interface{}{
		"id":               format.ID,
		"name":             format.Name,
		"bracket_type":     format.BracketType,
		"participant_type": format.ParticipantType,
		// SettingsJSON не отдаем в сыром виде, если не нужно. Отдаем settings.
		"settings": settings, // Будет null, если JSON пустой или невалидный
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"format": responsePayload}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// GetAllFormats godoc
// @Summary Получить все форматы
// @Tags formats
// @Description Возвращает список всех доступных форматов турниров.
// @Produce json
// @Success 200 {object} map[string]interface{} "Список форматов"
// @Failure 500 {object} map[string]string "Внутренняя ошибка сервера"
// @Router /formats [get]
func (h *FormatHandler) GetAllFormats(w http.ResponseWriter, r *http.Request) {
	formats, err := h.formatService.GetAllFormats(r.Context())
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	// Для каждого формата можно также размаршалить SettingsJSON, если нужно их показывать в списке
	responseFormats := make([]map[string]interface{}, len(formats))
	for i, f := range formats {
		var settings map[string]interface{}
		if f.SettingsJSON != nil && *f.SettingsJSON != "" {
			_ = json.Unmarshal([]byte(*f.SettingsJSON), &settings) // Ошибку здесь можно проигнорировать для списка
		}
		responseFormats[i] = map[string]interface{}{
			"id":               f.ID,
			"name":             f.Name,
			"bracket_type":     f.BracketType,
			"participant_type": f.ParticipantType,
			"settings":         settings,
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
// @Param body body services.UpdateFormatInput true "Данные для обновления формата"
// @Success 200 {object} map[string]interface{} "Формат обновлен"
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
	// Аналогично Create, UpdateFormatInput в сервисе нужно будет расширить
	// для поддержки всех изменяемых полей: Name, BracketType, ParticipantType, SettingsJSON.
	// type UpdateFormatInput struct {
	//    Name            *string                `json:"name"`
	//    BracketType     *string                `json:"bracket_type"`
	//    ParticipantType *models.FormatParticipantType `json:"participant_type"`
	//    SettingsJSON    *string                `json:"settings_json"` // Может быть строка "null" для удаления, или json-строка
	// }

	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// Валидация: хотя бы одно поле должно быть для обновления
	// (эта логика должна быть в сервисе или здесь, если input содержит указатели)
	// if input.Name == nil && input.BracketType == nil && input.ParticipantType == nil && input.SettingsJSON == nil {
	// 	badRequestResponse(w, r, errors.New("at least one field must be provided for update"))
	// 	return
	// }

	updatedFormat, err := h.formatService.UpdateFormat(r.Context(), formatID, input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"format": updatedFormat}, nil); err != nil {
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

// getIDFromURL - вспомогательная функция для извлечения ID из URL.
// Если у вас уже есть такая в общем файле helpers.go, используйте ее.
// Если нет, можно скопировать из team_handler.go или определить здесь.
// Для примера, я предполагаю, что она будет доступна (например, из helpers.go, который уже используется другими хендлерами).
// func getIDFromURL(r *http.Request, paramName string) (int, error) { ... }
