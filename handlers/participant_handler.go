package handlers

import (
	"errors"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"net/http"
)

type ParticipantHandler struct {
	participantService services.ParticipantService
}

func NewParticipantHandler(ps services.ParticipantService) *ParticipantHandler {
	return &ParticipantHandler{
		participantService: ps,
	}
}

// RegisterSolo godoc
// @Summary Подать заявку на участие в турнире (соло)
// @Tags participants
// @Description Пользователь подает заявку от своего имени.
// @Accept json
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Success 201 {object} map[string]interface{} "Заявка создана"
// @Failure 400 {object} map[string]string "Ошибка валидации или бизнес-логики"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав / Регистрация закрыта / Турнир полон"
// @Failure 404 {object} map[string]string "Турнир или пользователь не найден"
// @Failure 409 {object} map[string]string "Конфликт (уже зарегистрирован)"
// @Security BearerAuth
// @Router /tournaments/{tournamentID}/register/solo [post]
func (h *ParticipantHandler) RegisterSolo(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required")
		return
	}

	// В данном случае userID для регистрации - это currentUserID
	participant, err := h.participantService.RegisterSoloParticipant(r.Context(), currentUserID, tournamentID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"participant": participant}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// RegisterTeam godoc
// @Summary Подать заявку на участие в турнире (команда)
// @Tags participants
// @Description Капитан команды подает заявку от имени команды.
// @Accept json
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Param body body services.RegisterTeamParticipantInput true "Team ID для регистрации" // Input struct в сервисе будет проще
// @Success 201 {object} map[string]interface{} "Заявка создана"
// @Failure 400 {object} map[string]string "Ошибка валидации или бизнес-логики"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав / Регистрация закрыта / Турнир полон"
// @Failure 404 {object} map[string]string "Турнир или команда не найдена"
// @Failure 409 {object} map[string]string "Конфликт (уже зарегистрирована)"
// @Security BearerAuth
// @Router /tournaments/{tournamentID}/register/team [post]
func (h *ParticipantHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required")
		return
	}

	var input struct {
		TeamID int `json:"team_id" validate:"required,gt=0"`
	}
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	if input.TeamID <= 0 { // Дополнительная валидация
		badRequestResponse(w, r, errors.New("invalid team_id in request body"))
		return
	}

	participant, err := h.participantService.RegisterTeamParticipant(r.Context(), input.TeamID, tournamentID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusCreated, jsonResponse{"participant": participant}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// CancelRegistration godoc
// @Summary Отменить свою заявку/регистрацию на турнир
// @Tags participants
// @Description Пользователь или капитан команды отменяет заявку/регистрацию.
// @Produce json
// @Param participantID path int true "Participant Registration ID"
// @Success 204 "Заявка отменена"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав / Отмена не разрешена"
// @Failure 404 {object} map[string]string "Заявка не найдена"
// @Security BearerAuth
// @Router /participants/{participantID}/cancel [delete]
func (h *ParticipantHandler) CancelRegistration(w http.ResponseWriter, r *http.Request) {
	participantID, err := getIDFromURL(r, "participantID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required")
		return
	}

	err = h.participantService.CancelRegistration(r.Context(), participantID, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListApplications godoc
// @Summary Список заявок/участников турнира
// @Tags participants
// @Description Получает список заявок или участников турнира. Организатор видит все, другие могут видеть подтвержденных.
// @Produce json
// @Param tournamentID path int true "Tournament ID"
// @Param status query string false "Фильтр по статусу (application_submitted, participant, application_rejected)"
// @Success 200 {object} map[string]interface{} "Список заявок/участников"
// @Failure 401 {object} map[string]string "Неавторизован (если требуется для просмотра заявок)"
// @Failure 403 {object} map[string]string "Нет прав (если не организатор пытается посмотреть заявки)"
// @Failure 404 {object} map[string]string "Турнир не найден"
// @Security BearerAuth
// @Router /tournaments/{tournamentID}/participants [get]
func (h *ParticipantHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := getIDFromURL(r, "tournamentID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// currentUserID нужен сервису для проверки прав на просмотр заявок
	currentUserID, _ := middleware.GetUserIDFromContext(r.Context()) // Ошибка здесь не критична, если ID = 0, сервис разберется

	participants, err := h.participantService.ListTournamentApplications(r.Context(), tournamentID, currentUserID, nil)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"participants": participants}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// UpdateApplicationStatus godoc
// @Summary Обновить статус заявки на участие (одобрить/отклонить)
// @Tags participants
// @Description Организатор турнира одобряет или отклоняет заявку.
// @Accept json
// @Produce json
// @Param participantID path int true "Participant Registration ID"
// @Param body body services.UpdateApplicationStatusInput true "Новый статус заявки ('participant' или 'application_rejected')"
// @Success 200 {object} map[string]interface{} "Статус заявки обновлен"
// @Failure 400 {object} map[string]string "Ошибка валидации или бизнес-логики (неверный статус)"
// @Failure 401 {object} map[string]string "Неавторизован"
// @Failure 403 {object} map[string]string "Нет прав / Обновление не разрешено"
// @Failure 404 {object} map[string]string "Заявка или турнир не найден"
// @Security BearerAuth
// @Router /participants/{participantID}/status [patch]
func (h *ParticipantHandler) UpdateApplicationStatus(w http.ResponseWriter, r *http.Request) {
	participantID, err := getIDFromURL(r, "participantID")
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	currentUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		unauthorizedResponse(w, r, "authentication required")
		return
	}

	var input struct {
		Status models.ParticipantStatus `json:"status" validate:"required"`
	}
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}

	participant, err := h.participantService.UpdateApplicationStatus(r.Context(), participantID, input.Status, currentUserID)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, jsonResponse{"participant": participant}, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
