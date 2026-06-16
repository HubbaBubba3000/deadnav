package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deadnav/internal/models/dto"
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
// @Param month query string false "Month and year in format MM.YYYY, e.g., 12.2022. If not provided, returns full statistics"
// @Success 200 {object} models.Statistics "Full statistics"
// @Success 200 {object} dto.MonthlyStatistics "Monthly statistics"
// @Failure 400 {object} errorResponse "Invalid month format"
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse "No statistics found for the specified month"
// @Failure 500 {object} errorResponse
// @Router /api/v1/statistics [get]
func (h *StatisticsHandler) GetStatistics(c *gin.Context) {
	userID := mustUserID(c)

	// Check if month parameter is provided
	monthParam := c.Query("month")
	if monthParam == "" {
		// No month parameter, return full statistics
		stats, err := h.statsService.GetStatistics(userID)
		if err != nil {
			internalError(c, "GetStatistics: fetch", err)
			return
		}

		c.JSON(http.StatusOK, stats)
		return
	}

	// Parse month parameter (format: YYYY.MM)
	parts := strings.Split(monthParam, ".")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат месяца. Используйте ГГГГ.ММ, например, 2022.12"})
		return
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный год. Год должен быть числом"})
		return
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный месяц. Месяц должен быть числом от 1 до 12"})
		return
	}

	// Validate month range
	if month < 1 || month > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Месяц должен быть числом от 1 до 12"})
		return
	}

	// Create date from year and month to validate
	// For the first day of the requested month
	date := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if date.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "нельзя получить статистику за будущие даты"})
		return
	}

	// Get monthly statistics
	monthlyStats, err := h.statsService.GetMonthlyStatistics(userID, year, month)
	if err != nil {
		// Check if it's a "no rows" error
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"warning": "статистика не доступна для указанного месяца"})
			return
		}
		internalError(c, "GetMonthlyStatistics: fetch", err)
		return
	}

	// Check if the returned statistics are empty/zero
	if monthlyStats.TotalTasks == 0 && monthlyStats.CompletedTasks == 0 && len(monthlyStats.Heatmap) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"warning": "статистика не доступна для указанного месяца"})
		return
	}

	c.JSON(http.StatusOK, monthlyStats)
}

// CreateStatistics godoc
// @Summary Create monthly statistics
// @Description Create monthly statistics for a user
// @Tags statistics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param month query string true "Month and year in format YYYY.MM, e.g., 2022.12"
// @Param stats body dto.MonthlyStatistics true "Monthly statistics payload (month field in body is ignored)"
// @Success 201 {object} response "Statistics created successfully"
// @Failure 400 {object} errorResponse "Invalid month query parameter or request body"
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/v1/statistics [post]
func (h *StatisticsHandler) CreateStatistics(c *gin.Context) {
	userID := mustUserID(c)

	year, month, ok := parseMonthQuery(c)
	if !ok {
		return
	}

	var stats dto.MonthlyStatistics
	if err := c.ShouldBindJSON(&stats); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "недопустимое тело запроса"})
		return
	}
	stats.Month = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	if err := h.statsService.CreateMonthlyStatistics(userID, &stats, year, month); err != nil {
		internalError(c, "CreateMonthlyStatistics", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "статистика создана успешно"})
}

// UpdateStatistics godoc
// @Summary Update monthly statistics
// @Description Update monthly statistics for a user
// @Tags statistics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param month query string true "Month and year in format YYYY.MM, e.g., 2022.12"
// @Param stats body dto.MonthlyStatistics true "Monthly statistics payload (month field in body is ignored)"
// @Success 200 {object} response "Statistics updated successfully"
// @Failure 400 {object} errorResponse "Invalid month query parameter or request body"
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/v1/statistics [put]
func (h *StatisticsHandler) UpdateStatistics(c *gin.Context) {
	userID := mustUserID(c)

	year, month, ok := parseMonthQuery(c)
	if !ok {
		return
	}

	var stats dto.MonthlyStatistics
	if err := c.ShouldBindJSON(&stats); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "недопустимое тело запроса"})
		return
	}
	stats.Month = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	if err := h.statsService.UpdateMonthlyStatistics(userID, &stats, year, month); err != nil {
		internalError(c, "UpdateMonthlyStatistics", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "статистика обновлена успешно"})
}

// DeleteStatistics godoc
// @Summary Delete monthly statistics
// @Description Delete all monthly statistics for a user
// @Tags statistics
// @Security BearerAuth
// @Success 200 {object} response "Statistics deleted successfully"
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/v1/statistics [delete]
func (h *StatisticsHandler) DeleteStatistics(c *gin.Context) {
	userID := mustUserID(c) // panic when uid = 0

	if err := h.statsService.DeleteMonthlyStatistics(userID); err != nil {
		internalError(c, "DeleteMonthlyStatistics", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "статистика удалена успешно"})
}

// extractYearMonth pulls a (year, month) pair out of the optional Month field
// of a monthly statistics payload. It returns ok=false when the field is the
// zero value so the handler can produce a meaningful 400 error.
func extractYearMonth(month time.Time) (int, int, bool) {
	if month.IsZero() {
		return 0, 0, false
	}
	return month.Year(), int(month.Month()), true
}

// parseMonthQuery extracts and validates the `month` query parameter in
// YYYY.MM format used by POST/PUT /api/v1/statistics. It writes a 400
// response and returns ok=false when the parameter is missing or malformed.
func parseMonthQuery(c *gin.Context) (int, int, bool) {
	monthParam := strings.TrimSpace(c.Query("month"))
	if monthParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "параметр month обязателен. Используйте формат ГГГГ.ММ, например, 2022.12"})
		return 0, 0, false
	}

	parts := strings.Split(monthParam, ".")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат месяца. Используйте ГГГГ.ММ, например, 2022.12"})
		return 0, 0, false
	}

	year, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный год. Год должен быть числом"})
		return 0, 0, false
	}

	month, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || month < 1 || month > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный месяц. Месяц должен быть числом от 1 до 12"})
		return 0, 0, false
	}

	return year, month, true
}
