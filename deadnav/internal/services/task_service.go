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
	rows, err := s.db.Query(
		`SELECT id, user_id, title, description, status, priority, duration_minutes,
		        start_date, end_date, complexity, urgency, importance, estimated_minutes,
		        created_at, updated_at
		 FROM tasks
		 WHERE user_id = ?
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAllTasks: query: %w", err)
	}
	defer rows.Close()

	tasks, err := scanTasks(rows)
	if err != nil {
		return nil, fmt.Errorf("GetAllTasks: %w", err)
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

// UpdateTask applies the mutable fields of task to the row with id that belongs
// to userID. Returns a wrapped sql.ErrNoRows when the task is not found.
func (s *TaskService) UpdateTask(id int64, userID int64, task *models.Task) error {
	task.UpdatedAt = time.Now()

	res, err := s.db.Exec(
		`UPDATE tasks
		 SET title            = ?,
		     description      = ?,
		     status           = ?,
		     priority         = ?,
		     duration_minutes = ?,
		     start_date       = ?,
		     end_date         = ?,
		     complexity       = ?,
		     urgency          = ?,
		     importance       = ?,
		     estimated_minutes= ?,
		     updated_at       = ?
		 WHERE id = ? AND user_id = ?`,
		task.Title, task.Description,
		task.Status, task.Priority, task.DurationMinutes,
		task.StartDate, task.EndDate,
		task.Complexity, task.Urgency, task.Importance, task.EstimatedMinutes,
		task.UpdatedAt,
		id, userID,
	)
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
