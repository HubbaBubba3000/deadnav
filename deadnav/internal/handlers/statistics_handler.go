package handlers

import (
	"net/http"

	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// StatisticsHandler handles HTTP requests for user statistics.
type StatisticsHandler struct {
	statsService *services.StatisticsService
}

// NewStatisticsHandler creates a StatisticsHandler with the given service.
func NewStatisticsHandler(statsService *services.StatisticsService) *StatisticsHandler {
	return &StatisticsHandler{statsService: statsService}
}

// GetStatistics godoc
// @Summary Get statistics
// @Description Return a comprehensive statistics report for the authenticated user's tasks.
// @Tags statistics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.Statistics
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/v1/statistics [get]
func (h *StatisticsHandler) GetStatistics(c *gin.Context) {
	userID := mustUserID(c)

	stats, err := h.statsService.GetStatistics(userID)
	if err != nil {
		internalError(c, "GetStatistics: fetch", err)
		return
	}

	c.JSON(http.StatusOK, stats)
}
