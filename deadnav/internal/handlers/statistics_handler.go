package handlers

import (
	"net/http"
	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

type StatisticsHandler struct {
	statsService *services.StatisticsService
}

func NewStatisticsHandler(statsService *services.StatisticsService) *StatisticsHandler {
	return &StatisticsHandler{statsService: statsService}
}

func (h *StatisticsHandler) GetStatistics(c *gin.Context) {
	stats, err := h.statsService.GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
