package handlers

import (
	"github.com/Dosada05/tournament-system/services"
	"net/http"
)

type DashboardHandler struct {
	dashboardService services.DashboardService
}

func NewDashboardHandler(s services.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: s}
}

func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.dashboardService.GetStats(r.Context())
	if err != nil {
		serverErrorResponse(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stats, nil)
}
