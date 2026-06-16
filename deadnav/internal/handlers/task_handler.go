package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"deadnav/internal/models"
	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// parseTaskFilter extracts TaskFilter from query parameters.
func parseTaskFilter(c *gin.Context, userID int64) (models.TaskFilter, error) {
	var priority *int
	if pStr := c.Query("priority"); pStr != "" {
		p, err := strconv.Atoi(pStr)
		if err != nil {
			return models.TaskFilter{}, err
		}
		priority = &p
	}

	filter := models.TaskFilter{
		UserID:   userID,
		Status:   c.Query("status"),
		Priority: priority,
	}

	if from := c.Query("start_date_from"); from != "" {
		t, err := time.Parse(time.RFC3339Nano, from)
		if err != nil {
			return models.TaskFilter{}, err
		}
		filter.StartDateFrom = t
	}
	if to := c.Query("start_date_to"); to != "" {
		t, err := time.Parse(time.RFC3339Nano, to)
		if err != nil {
			return models.TaskFilter{}, err
		}
		filter.StartDateTo = t
	}
	if from := c.Query("end_date_from"); from != "" {
		t, err := time.Parse(time.RFC3339Nano, from)
		if err != nil {
			return models.TaskFilter{}, err
		}
		filter.EndDateFrom = t
	}
	if to := c.Query("end_date_to"); to != "" {
		t, err := time.Parse(time.RFC3339Nano, to)
		if err != nil {
			return models.TaskFilter{}, err
		}
		filter.EndDateTo = t
	}

	return filter, nil
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
		c.JSON(http.StatusBadRequest, errorResponse{Error: "неверный формат запроса"})
		return
	}
	task.UserID = userID

	// Calculate estimated time if not provided.
	if task.EstimatedMinutes == 0 {
		task.EstimatedMinutes = models.CalculateEstimatedTime(task.Complexity, task.Urgency, task.Importance)
	}

	if err := h.taskService.CreateTask(&task); err != nil {
		internalError(c, "CreateTask: insert", err)
		return
	}

	// Auto-schedule is required — if it fails the task must not be persisted.
	// We compensate by deleting the just-created task so the caller sees a
	// clean error and no orphan row remains in the DB.
	schedule, schedErr := h.scheduleService.AutoScheduleTask(&task, userID)
	if schedErr != nil {
		if delErr := h.taskService.DeleteTask(task.ID, userID); delErr != nil {
			internalError(c, "CreateTask: rollback after schedule failure", delErr)
			return
		}
		c.JSON(http.StatusUnprocessableEntity, errorResponse{
			Error: "не удалось разместить задачу в расписании: " + schedErr.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, createTaskResponse{
		Task:     task,
		Schedule: schedule,
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
			c.JSON(http.StatusNotFound, errorResponse{Error: "задача не найдена"})
			return
		}
		internalError(c, "GetTask: fetch", err)
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetAllTasks godoc
// @Summary List tasks
// @Description Return all tasks that belong to the authenticated user. Supports optional filtering via query parameters.
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param   status         query string false "Filter by status (pending, in_progress, completed, cancelled)"
// @Param   priority       query int    false "Filter by priority (1-5)"
// @Param   start_date_from query string false "Filter tasks with start_date >= this datetime (ISO 8601, e.g. 2026-06-06T19:00:00Z)"
// @Param   start_date_to   query string false "Filter tasks with start_date <= this datetime (ISO 8601, e.g. 2026-06-06T19:00:00Z)"
// @Param   end_date_from   query string false "Filter tasks with end_date >= this datetime (ISO 8601, e.g. 2026-06-06T19:00:00Z)"
// @Param   end_date_to     query string false "Filter tasks with end_date <= this datetime (ISO 8601, e.g. 2026-06-06T19:00:00Z)"
// @Success 200 {array}  models.Task
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/tasks [get]
func (h *TaskHandler) GetAllTasks(c *gin.Context) {
	userID := mustUserID(c)

	filter, err := parseTaskFilter(c, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid filter: " + err.Error()})
		return
	}

	tasks, err := h.taskService.GetTasks(filter)
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
// @Description Update mutable fields of a task. Only the fields that are
// explicitly present in the request body are modified; omitted fields are
// left unchanged. If dates change the existing schedule is automatically
// recalculated.
// @Tags tasks
// @Accept  json
// @Produce json
// @Security BearerAuth
// @Param   id    path int             true "Task ID"
// @Param   patch body models.TaskUpdate true "Partial task payload"
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

	var patch models.TaskUpdate
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "неверный формат запроса"})
		return
	}

	if !patch.HasAny() {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "не передано ни одного поля для обновления"})
		return
	}

	if err := h.taskService.UpdateTask(id, userID, &patch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, errorResponse{Error: "задача не найдена"})
			return
		}
		if errors.Is(err, services.ErrEmptyUpdate) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "не передано ни одного поля для обновления"})
			return
		}
		internalError(c, "UpdateTask: exec", err)
		return
	}

	// Re-fetch the updated task so we can reschedule with accurate data.
	var updated *models.Task
	var schedule *models.Schedule
	var schedWarning *scheduleWarning
	if updated, err = h.taskService.GetTaskByID(id, userID); err != nil {
		schedWarning = &scheduleWarning{
			Code:    "schedule_error",
			Message: "не удалось получить обновленную задачу: " + err.Error(),
		}
	} else {
		updated.ID = id
		schedule, err = h.scheduleService.AutoScheduleTask(updated, userID)
		schedWarning = buildScheduleWarning(err, userID, h.taskService)
	}

	c.JSON(http.StatusOK, updateTaskResponse{
		Message:         "задача обновлена успешно",
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
			c.JSON(http.StatusNotFound, errorResponse{Error: "задача не найдена"})
			return
		}
		internalError(c, "DeleteTask: exec", err)
		return
	}

	c.JSON(http.StatusOK, messageResponse{Message: "задача удалена успешно"})
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type createTaskResponse struct {
	Task            models.Task      `json:"task"`
	Schedule        *models.Schedule `json:"schedule,omitempty"`
	ScheduleWarning *scheduleWarning `json:"schedule_warning,omitempty"`
}

type updateTaskResponse struct {
	Message         string           `json:"message"`
	Schedule        *models.Schedule `json:"schedule,omitempty"`
	ScheduleWarning *scheduleWarning `json:"schedule_warning,omitempty"`
}

// blockedTaskInfo describes a task that blocks scheduling of another task.
type blockedTaskInfo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// scheduleWarning is returned when auto-scheduling fails. The human-readable
// message lives in Message; structured data (e.g. tasks that need to be
// moved) lives in Data so clients don't have to parse strings.
type scheduleWarning struct {
	Message string         `json:"message"`
	Code    string         `json:"code,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func buildScheduleWarning(scheduleErr error, userID int64, taskService *services.TaskService) *scheduleWarning {
	if scheduleErr == nil {
		return nil
	}

	if strings.Contains(scheduleErr.Error(), "blocked_tasks=") {
		start := strings.Index(scheduleErr.Error(), "[")
		end := strings.Index(scheduleErr.Error(), "]")
		if start != -1 && end != -1 && end > start {
			idsStr := scheduleErr.Error()[start+1 : end]
			idStrs := strings.Split(idsStr, ",")
			var blocked []blockedTaskInfo
			for _, idStr := range idStrs {
				idStr = strings.TrimSpace(idStr)
				if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
					if blockingTask, err := taskService.GetTaskByID(id, userID); err == nil {
						blocked = append(blocked, blockedTaskInfo{
							ID:    blockingTask.ID,
							Title: blockingTask.Title,
							Date:  blockingTask.EndDate.Format("2006-01-02"),
						})
					}
				}
			}
			if len(blocked) > 0 {
				return &scheduleWarning{
					Code:    "blocked_tasks",
					Message: "Невозможно разместить задачу в расписании до дедлайна. Пожалуйста, переместите или перенесите следующие задачи.",
					Data: map[string]any{
						"blocked_tasks": blocked,
					},
				}
			}
		}
		return &scheduleWarning{
			Code:    "blocked_tasks",
			Message: "Невозможно разместить задачу в расписании до дедлайна. Пожалуйста, освободите время, переместив другие задачи.",
		}
	}

	return &scheduleWarning{
		Code:    "schedule_error",
		Message: scheduleErr.Error(),
	}
}
