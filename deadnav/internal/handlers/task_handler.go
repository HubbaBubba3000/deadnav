package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"deadnav/internal/models"
	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// calculateEstimatedTime computes the estimated time in minutes based on
// complexity, urgency, and importance. All arithmetic is kept in the integer
// domain — no unnecessary float conversions.
func calculateEstimatedTime(complexity, urgency, importance int) int {
	estimatedMinutes := (complexity + urgency + importance) * 15

	minTime := 30
	maxTime := 480

	if estimatedMinutes < minTime {
		estimatedMinutes = minTime
	}
	if estimatedMinutes > maxTime {
		estimatedMinutes = maxTime
	}
	return estimatedMinutes
}

// TaskHandler handles HTTP requests for task management.
type TaskHandler struct {
	taskService     *services.TaskService
	scheduleService *services.ScheduleService
}

// NewTaskHandler creates a TaskHandler with the given services.
func NewTaskHandler(taskService *services.TaskService, scheduleService *services.ScheduleService) *TaskHandler {
	return &TaskHandler{
		taskService:     taskService,
		scheduleService: scheduleService,
	}
}

// CreateTask godoc
// @Summary Create a task
// @Description Create a new task for the authenticated user. The scheduler automatically
// finds the first available working-hours slot and attaches a schedule entry.
// @Tags tasks
// @Accept  json
// @Produce json
// @Security BearerAuth
// @Param   task body models.Task true "Task payload"
// @Success 201 {object} createTaskResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userID := mustUserID(c)

	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	task.UserID = userID

	// Calculate estimated time if not provided.
	if task.EstimatedMinutes == 0 {
		task.EstimatedMinutes = calculateEstimatedTime(task.Complexity, task.Urgency, task.Importance)
	}

	if err := h.taskService.CreateTask(&task); err != nil {
		internalError(c, "CreateTask: insert", err)
		return
	}

	// Auto-schedule is best-effort — failure does not roll back task creation.
	schedule, schedErr := h.scheduleService.AutoScheduleTask(&task, userID)

	var scheduleWarning string
	if schedErr != nil {
		scheduleWarning = schedErr.Error()
	}

	c.JSON(http.StatusCreated, createTaskResponse{
		Task:            task,
		Schedule:        schedule,
		ScheduleWarning: scheduleWarning,
	})
}

// GetTask godoc
// @Summary Get a task
// @Description Get a single task by ID (must belong to the authenticated user).
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param   id path int true "Task ID"
// @Success 200 {object} models.Task
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id} [get]
func (h *TaskHandler) GetTask(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	task, err := h.taskService.GetTaskByID(id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "task not found"})
			return
		}
		internalError(c, "GetTask: fetch", err)
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetAllTasks godoc
// @Summary List tasks
// @Description Return all tasks that belong to the authenticated user.
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Success 200 {array}  models.Task
// @Failure 401 {object} errorResponse
// @Router /api/v1/tasks [get]
func (h *TaskHandler) GetAllTasks(c *gin.Context) {
	userID := mustUserID(c)

	tasks, err := h.taskService.GetAllTasks(userID)
	if err != nil {
		internalError(c, "GetAllTasks: query", err)
		return
	}

	if tasks == nil {
		tasks = []models.Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

// UpdateTask godoc
// @Summary Update a task
// @Description Update mutable fields of a task. If dates change the existing
// schedule is automatically recalculated.
// @Tags tasks
// @Accept  json
// @Produce json
// @Security BearerAuth
// @Param   id   path int        true "Task ID"
// @Param   task body models.Task true "Updated task payload"
// @Success 200 {object} updateTaskResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id} [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Recalculate estimated time if complexity/urgency/importance were changed.
	task.EstimatedMinutes = calculateEstimatedTime(task.Complexity, task.Urgency, task.Importance)

	if err := h.taskService.UpdateTask(id, userID, &task); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "task not found"})
			return
		}
		internalError(c, "UpdateTask: exec", err)
		return
	}

	// Re-fetch the updated task so we can reschedule with accurate data.
	updated, _ := h.taskService.GetTaskByID(id, userID)
	var schedule *models.Schedule
	var schedWarning string
	if updated != nil {
		updated.ID = id
		schedule, err = h.scheduleService.AutoScheduleTask(updated, userID)
		if err != nil {
			schedWarning = err.Error()
		}
	}

	c.JSON(http.StatusOK, updateTaskResponse{
		Message:         "task updated successfully",
		Schedule:        schedule,
		ScheduleWarning: schedWarning,
	})
}

// DeleteTask godoc
// @Summary Delete a task
// @Description Delete a task and its associated schedule entry.
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param   id path int true "Task ID"
// @Success 200 {object} messageResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id} [delete]
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	userID := mustUserID(c)
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Remove schedule first (cascades on FK, but be explicit for clarity).
	_ = h.scheduleService.RemoveSchedule(id, userID)

	if err := h.taskService.DeleteTask(id, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "task not found"})
			return
		}
		internalError(c, "DeleteTask: exec", err)
		return
	}

	c.JSON(http.StatusOK, messageResponse{Message: "task deleted successfully"})
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type createTaskResponse struct {
	Task            models.Task      `json:"task"`
	Schedule        *models.Schedule `json:"schedule,omitempty"`
	ScheduleWarning string           `json:"schedule_warning,omitempty"`
}

type updateTaskResponse struct {
	Message         string           `json:"message"`
	Schedule        *models.Schedule `json:"schedule,omitempty"`
	ScheduleWarning string           `json:"schedule_warning,omitempty"`
}
