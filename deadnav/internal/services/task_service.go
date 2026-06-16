package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"deadnav/internal/models"
)

// TaskService handles CRUD operations for tasks.
type TaskService struct {
	db *sql.DB
}

// NewTaskService creates a TaskService backed by the provided database connection.
func NewTaskService(db *sql.DB) *TaskService {
	return &TaskService{db: db}
}

// CreateTask inserts a new task and populates task.ID and the timestamp fields.
func (s *TaskService) CreateTask(task *models.Task) error {
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	result, err := s.db.Exec(
		`INSERT INTO tasks
		     (user_id, title, description, status, priority, duration_minutes,
		      start_date, end_date, complexity, urgency, importance, estimated_minutes,
		      created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.UserID, task.Title, task.Description,
		task.Status, task.Priority, task.DurationMinutes,
		task.StartDate, task.EndDate,
		task.Complexity, task.Urgency, task.Importance, task.EstimatedMinutes,
		task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("CreateTask: insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("CreateTask: last insert id: %w", err)
	}
	task.ID = id
	return nil
}

// GetAllTasks returns all tasks that belong to userID, newest first.
func (s *TaskService) GetAllTasks(userID int64) ([]models.Task, error) {
	return s.GetTasks(models.TaskFilter{UserID: userID})
}

// GetTasks returns tasks matching the given filter, newest first.
func (s *TaskService) GetTasks(filter models.TaskFilter) ([]models.Task, error) {
	query := `SELECT id, user_id, title, description, status, priority, duration_minutes,
	        start_date, end_date, complexity, urgency, importance, estimated_minutes,
	        created_at, updated_at
	 FROM tasks
	 WHERE user_id = ?`
	args := []any{filter.UserID}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Priority != nil {
		query += " AND priority = ?"
		args = append(args, *filter.Priority)
	}
	if !filter.StartDateFrom.IsZero() {
		query += " AND start_date >= ?"
		args = append(args, filter.StartDateFrom)
	}
	if !filter.StartDateTo.IsZero() {
		query += " AND start_date <= ?"
		args = append(args, filter.StartDateTo)
	}
	if !filter.EndDateFrom.IsZero() {
		query += " AND end_date >= ?"
		args = append(args, filter.EndDateFrom)
	}
	if !filter.EndDateTo.IsZero() {
		query += " AND end_date <= ?"
		args = append(args, filter.EndDateTo)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetTasks: query: %w", err)
	}
	defer rows.Close()

	tasks, err := scanTasks(rows)
	if err != nil {
		return nil, fmt.Errorf("GetTasks: %w", err)
	}
	return tasks, nil
}

// GetTaskByID returns the task with the given id if it belongs to userID.
// Returns a wrapped sql.ErrNoRows when not found.
func (s *TaskService) GetTaskByID(id int64, userID int64) (*models.Task, error) {
	var t models.Task
	err := s.db.QueryRow(
		`SELECT id, user_id, title, description, status, priority, duration_minutes,
		        start_date, end_date, complexity, urgency, importance, estimated_minutes,
		        created_at, updated_at
		 FROM tasks
		 WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(
		&t.ID, &t.UserID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.DurationMinutes,
		&t.StartDate, &t.EndDate, &t.Complexity, &t.Urgency, &t.Importance, &t.EstimatedMinutes,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("GetTaskByID: task %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("GetTaskByID: scan: %w", err)
	}
	return &t, nil
}

// UpdateTask applies the non-nil fields of patch to the row with id that
// belongs to userID. A wrapped sql.ErrNoRows is returned when the task is
// not found; a wrapped ErrEmptyUpdate is returned when patch has no fields
// set.
func (s *TaskService) UpdateTask(id int64, userID int64, patch *models.TaskUpdate) error {
	if patch == nil || !patch.HasAny() {
		return fmt.Errorf("UpdateTask: no fields to update: %w", ErrEmptyUpdate)
	}

	// If the deadline is being moved, we need its current value to decide
	// whether moved_deadline should flip on (in addition to fetching the
	// existing estimation inputs when those weren't sent).
	var (
		curEndDate    time.Time
		curComplexity int
		curUrgency    int
		curImportance int
	)
	err := s.db.QueryRow(
		`SELECT end_date, complexity, urgency, importance
		 FROM tasks
		 WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(&curEndDate, &curComplexity, &curUrgency, &curImportance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("UpdateTask: task %d not found: %w", id, sql.ErrNoRows)
		}
		return fmt.Errorf("UpdateTask: scan current: %w", err)
	}

	now := time.Now()

	// Build the dynamic UPDATE: only the columns explicitly set on the
	// patch are written. updated_at is always refreshed. moved_deadline is
	// set when the caller actually supplied a new end_date that differs
	// from the stored one.
	sets := []string{"updated_at = ?"}
	args := []any{now}

	if patch.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *patch.Title)
	}
	if patch.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *patch.Description)
	}
	if patch.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *patch.Status)
	}
	if patch.Priority != nil {
		sets = append(sets, "priority = ?")
		args = append(args, *patch.Priority)
	}
	if patch.DurationMinutes != nil {
		sets = append(sets, "duration_minutes = ?")
		args = append(args, *patch.DurationMinutes)
	}
	if patch.StartDate != nil {
		sets = append(sets, "start_date = ?")
		args = append(args, *patch.StartDate)
	}
	if patch.EndDate != nil {
		sets = append(sets, "end_date = ?")
		args = append(args, *patch.EndDate)
		moved := !patch.EndDate.Equal(curEndDate)
		sets = append(sets, "moved_deadline = ?")
		args = append(args, moved)
	}
	if patch.EstimatedMinutes != nil {
		sets = append(sets, "estimated_minutes = ?")
		args = append(args, *patch.EstimatedMinutes)
	}

	// If the estimation inputs change but the caller didn't pass an
	// explicit estimated_minutes, derive it server-side from the resulting
	// (patched || existing) values to keep the column consistent.
	if patch.EstimatedMinutes == nil && patch.AffectsEstimation() {
		complexity := curComplexity
		urgency := curUrgency
		importance := curImportance
		if patch.Complexity != nil {
			complexity = *patch.Complexity
		}
		if patch.Urgency != nil {
			urgency = *patch.Urgency
		}
		if patch.Importance != nil {
			importance = *patch.Importance
		}
		sets = append(sets, "estimated_minutes = ?")
		args = append(args, models.CalculateEstimatedTime(complexity, urgency, importance))
	}

	query := "UPDATE tasks SET "
	for i, col := range sets {
		if i > 0 {
			query += ", "
		}
		query += col
	}
	query += " WHERE id = ? AND user_id = ?"
	args = append(args, id, userID)

	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("UpdateTask: exec: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateTask: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("UpdateTask: task %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

// DeleteTask removes the task with id that belongs to userID.
// Returns a wrapped sql.ErrNoRows when the task is not found.
func (s *TaskService) DeleteTask(id int64, userID int64) error {
	res, err := s.db.Exec(
		`DELETE FROM tasks WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("DeleteTask: exec: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteTask: rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("DeleteTask: task %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

// ErrEmptyUpdate is returned by UpdateTask when the supplied patch contains
// no fields to apply.
var ErrEmptyUpdate = errors.New("empty update")

// scanTasks reads all rows from a task query result into a slice.
func scanTasks(rows *sql.Rows) ([]models.Task, error) {
	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.DurationMinutes,
			&t.StartDate, &t.EndDate, &t.Complexity, &t.Urgency, &t.Importance, &t.EstimatedMinutes,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanTasks: scan row: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanTasks: iterate rows: %w", err)
	}
	return tasks, nil
}
