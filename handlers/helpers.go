package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Dosada05/tournament-system/services" // Импортируем для маппинга ошибок сервисов
)

type jsonResponse map[string]interface{}

type contextKey string

const userContextKey contextKey = "user"

func readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		case errors.As(err, &invalidUnmarshalError):
			panic(err) // Паника, т.к. это ошибка программиста (передан не указатель)
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(js)
	if err != nil {
		return err
	}

	return nil
}

func errorResponse(w http.ResponseWriter, r *http.Request, status int, message interface{}) {
	env := jsonResponse{"error": message}
	err := writeJSON(w, status, env, nil)
	if err != nil {
		// Логируем ошибку записи JSON (важно!)
		fmt.Printf("Error writing error JSON response: %v\n", err) // Замените на ваш логгер
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	// Логируем реальную ошибку (важно!)
	fmt.Printf("Internal server error: %v\n", err) // Замените на ваш логгер
	message := "the server encountered a problem and could not process your request"
	errorResponse(w, r, http.StatusInternalServerError, message)
}

func badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	errorResponse(w, r, http.StatusNotFound, message)
}

func conflictResponse(w http.ResponseWriter, r *http.Request, message string) {
	errorResponse(w, r, http.StatusConflict, message)
}

func unauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	errorResponse(w, r, http.StatusUnauthorized, message)
}

func forbiddenResponse(w http.ResponseWriter, r *http.Request, message string) {
	errorResponse(w, r, http.StatusForbidden, message)
}

// mapServiceErrorToHTTP преобразует ошибки сервисного слоя в HTTP-ответы
func mapServiceErrorToHTTP(w http.ResponseWriter, r *http.Request, err error) {
	// Здесь добавляем маппинг конкретных ошибок сервисов
	switch {
	// Общие ошибки
	case errors.Is(err, services.ErrNotFound),
		errors.Is(err, services.ErrUserNotFound),
		errors.Is(err, services.ErrTeamNotFound),
		errors.Is(err, services.ErrSportNotFound),
		errors.Is(err, services.ErrFormatNotFound),
		errors.Is(err, services.ErrTournamentNotFound),
		errors.Is(err, services.ErrParticipantNotFound),
		errors.Is(err, services.ErrInviteNotFound):
		notFoundResponse(w, r)

	// Конфликты
	case errors.Is(err, services.ErrUserEmailConflict),
		errors.Is(err, services.ErrUserNicknameConflict),
		errors.Is(err, services.ErrTeamNameConflict),
		errors.Is(err, services.ErrTournamentNameConflict),
		errors.Is(err, services.ErrRegistrationConflict):
		conflictResponse(w, r, err.Error())

	// Невалидные данные / бизнес-правила (часто 400 или 422)
	case errors.Is(err, services.ErrValidationFailed), // Если будет общая ошибка валидации
		errors.Is(err, services.ErrPasswordTooShort),
		errors.Is(err, services.ErrInvalidCredentials), // Можно 401, но 400 тоже вариант
		errors.Is(err, services.ErrTeamNameRequired),
		errors.Is(err, services.ErrTournamentNameRequired),
		errors.Is(err, services.ErrTournamentInvalidRegDate),
		errors.Is(err, services.ErrTournamentInvalidDateRange),
		errors.Is(err, services.ErrTournamentInvalidCapacity),
		errors.Is(err, services.ErrTournamentInvalidStatus),
		errors.Is(err, services.ErrTournamentInvalidStatusTransition),
		errors.Is(err, services.ErrUserCannotRegisterSolo),
		errors.Is(err, services.ErrUserAlreadyInTeam),
		errors.Is(err, services.ErrCannotRemoveCaptain),
		errors.Is(err, services.ErrInviteExpired):
		// Используем StatusBadRequest для большинства бизнес-ошибок, если не указано иное
		badRequestResponse(w, r, err)

	// Ошибки авторизации/доступа
	case errors.Is(err, services.ErrAuthenticationFailed):
		unauthorizedResponse(w, r, err.Error())
	case errors.Is(err, services.ErrForbiddenOperation),
		errors.Is(err, services.ErrCaptainActionForbidden),
		errors.Is(err, services.ErrSelfLeaveForbidden),
		errors.Is(err, services.ErrUserMustBeCaptain):
		forbiddenResponse(w, r, err.Error())

	case errors.Is(err, services.ErrAuthInvalidCredentials):
		unauthorizedResponse(w, r, err.Error())
	case errors.Is(err, services.ErrAuthEmailTaken):
		conflictResponse(w, r, err.Error())

	// Другие специфичные ошибки, которые могут требовать особого статуса
	case errors.Is(err, services.ErrRegistrationNotOpen):
		forbiddenResponse(w, r, err.Error()) // Или 400/409? Зависит от семантики.
	case errors.Is(err, services.ErrTournamentFull):
		conflictResponse(w, r, err.Error()) // 409 Conflict - подходящий статус

	// Непредвиденные ошибки / ошибки по умолчанию
	default:
		serverErrorResponse(w, r, err)
	}
}
