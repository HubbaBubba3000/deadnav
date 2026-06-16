package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"deadnav/internal/models"
	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// ScheduleHandler handles HTTP requests for calendar scheduling.
type ScheduleHandler struct {
	scheduleService *services.ScheduleService
	taskService     *services.TaskService
}

// NewScheduleHandler creates a ScheduleHandler with the given services.
func NewScheduleHandler(scheduleService *services.ScheduleService, taskService *services.TaskService) *ScheduleHandler {
	return &ScheduleHandler{
		scheduleService: scheduleService,
		taskService:     taskService,
	}
}

// parseScheduleFilter extracts ScheduleFilter from query parameters.
func parseScheduleFilter(c *gin.Context, userID int64) (models.ScheduleFilter, error) {
	filter := models.ScheduleFilter{UserID: userID}

	if from := c.Query("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return models.ScheduleFilter{}, err
		}
		filter.From = t
	}
	if to := c.Query("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return models.ScheduleFilter{}, err
		}
		filter.To = t
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if order := c.Query("order"); order != "" {
		filter.Order = order
	}

	return filter, nil
}

// GetSchedule godoc
// @Summary Get user schedule
// @Description Return all scheduled time blocks for the authenticated user.
// @Description Supports optional filtering via query parameters.
// @Tags schedule
// @Produce json
// @Security BearerAuth
// @Param   from query string false "Filter schedules with start_time >= this time (RFC3339)"
// @Param   to   query string false "Filter schedules with end_time <= this time (RFC3339)"
// @Success 200 {array}  models.Schedule
// @Failure 401 {object} errorResponse
// @Router /api/v1/schedule [get]
func (h *ScheduleHandler) GetSchedule(c *gin.Context) {
	userID := mustUserID(c)

	filter, err := parseScheduleFilter(c, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid filter: " + err.Error()})
		return
	}

	schedules, err := h.scheduleService.GetSchedules(filter)
	if err != nil {
		internalError(c, "GetSchedule: query", err)
		return
	}

	if schedules == nil {
		schedules = []models.Schedule{}
	}
	c.JSON(http.StatusOK, schedules)
}

// GetTaskSchedule godoc
// @Summary Get schedule for a task
// @Description Return the scheduled time block for a specific task.
// @Tags schedule
// @Produce json
// @Security BearerAuth
// @Param   id path int true "Task ID"
// @Success 200 {object} models.Schedule
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/schedule/task/{id} [get]
func (h *ScheduleHandler) GetTaskSchedule(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	schedule, err := h.scheduleService.GetTaskSchedule(id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "расписание для этой задачи не найдено"})
			return
		}
		internalError(c, "GetTaskSchedule: fetch", err)
		return
	}

	c.JSON(http.StatusOK, schedule)
}

// RescheduleTask godoc
// @Summary Reschedule a task
// @Description Re-run the auto-scheduler for a specific task, replacing any existing slot.
// @Tags schedule
// @Produce json
// @Security BearerAuth
// @Param   id path int true "Task ID"
// @Success 200 {object} rescheduleTaskResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 422 {object} rescheduleTaskResponse
// @Router /api/v1/schedule/task/{id}/reschedule [post]
func (h *ScheduleHandler) RescheduleTask(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	task, err := h.taskService.GetTaskByID(id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "задача не найдена"})
			return
		}
		internalError(c, "RescheduleTask: fetch", err)
		return
	}

	schedule, err := h.scheduleService.RescheduleTaskWithCascade(task, userID)
	scheduleWarning := buildScheduleWarning(err, userID, h.taskService)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, rescheduleTaskResponse{ScheduleWarning: scheduleWarning})
		return
	}

	c.JSON(http.StatusOK, rescheduleTaskResponse{
		Schedule:        schedule,
		ScheduleWarning: scheduleWarning,
	})
}

// UnscheduleTask godoc
// @Summary Remove a task from the schedule
// @Description Delete the calendar entry for a specific task without deleting the task itself.
// @Tags schedule
// @Produce json
// @Security BearerAuth
// @Param   id path int true "Task ID"
// @Success 200 {object} messageResponse
// @Failure 400 {object} errorResponse
// @Router /api/v1/schedule/task/{id} [delete]
func (h *ScheduleHandler) UnscheduleTask(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.scheduleService.RemoveSchedule(id, userID); err != nil {
		internalError(c, "UnscheduleTask: remove", err)
		return
	}

	c.JSON(http.StatusOK, messageResponse{Message: "расписание удалено успешно"})
}

// GetFreeSlots godoc
// @Summary Get free time slots
// @Description Return all free working-hours slots of the requested duration
// within the given time range.
// @Tags schedule
// @Produce json
// @Security BearerAuth
// @Param   from     query string true  "Start of range (RFC3339)"  example("2024-01-15T00:00:00Z")
// @Param   to       query string true  "End of range (RFC3339)"    example("2024-01-22T00:00:00Z")
// @Param   duration query int    false "Minimum slot duration in minutes (default 60)"
// @Success 200 {array}  models.ScheduleSlot
// @Failure 400 {object} errorResponse
// @Router /api/v1/schedule/free-slots [get]
type rescheduleTaskResponse struct {
	Schedule        *models.Schedule `json:"schedule,omitempty"`
	ScheduleWarning *scheduleWarning `json:"schedule_warning,omitempty"`
}

func (h *ScheduleHandler) GetFreeSlots(c *gin.Context) {
	userID := mustUserID(c)

	fromStr := c.Query("from")
	toStr := c.Query("to")
	durationStr := c.DefaultQuery("duration", "60")

	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "параметры 'from' и 'to' обязательны (формат RFC3339)"})
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "неверная дата 'from': " + err.Error()})
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "неверная дата 'to': " + err.Error()})
		return
	}
	duration, err := strconv.Atoi(durationStr)
	if err != nil || duration < 1 {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "'duration' должен быть положительным целым числом (минуты)"})
		return
	}

	slots, err := h.scheduleService.GetFreeSlots(userID, from, to, duration)
	if err != nil {
		internalError(c, "GetFreeSlots: search", err)
		return
	}

	if slots == nil {
		slots = []models.ScheduleSlot{}
	}
	c.JSON(http.StatusOK, slots)
}
