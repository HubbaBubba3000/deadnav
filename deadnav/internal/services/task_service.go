package services

import (
	"database/sql"
	"deadnav/internal/models"
	"time"
)

type TaskService struct {
	db *sql.DB
}

func NewTaskService(db *sql.DB) *TaskService {
	return &TaskService{db: db}
}

func (s *TaskService) CreateTask(task *models.Task) error {
	query := `INSERT INTO tasks (title, description, status, priority, start_date, end_date, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	
	result, err := s.db.Exec(query, task.Title, task.Description, task.Status, task.Priority, 
		task.StartDate, task.EndDate, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return err
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	task.ID = id
	
	return nil
}

func (s *TaskService) GetAllTasks() ([]models.Task, error) {
	query := `SELECT id, title, description, status, priority, start_date, end_date, created_at, updated_at 
			  FROM tasks ORDER BY created_at DESC`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority,
			&task.StartDate, &task.EndDate, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	
	return tasks, nil
}

func (s *TaskService) GetTaskByID(id int64) (*models.Task, error) {
	query := `SELECT id, title, description, status, priority, start_date, end_date, created_at, updated_at 
			  FROM tasks WHERE id = ?`
	
	var task models.Task
	err := s.db.QueryRow(query, id).Scan(&task.ID, &task.Title, &task.Description, &task.Status, 
		&task.Priority, &task.StartDate, &task.EndDate, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	
	return &task, nil
}

func (s *TaskService) UpdateTask(id int64, task *models.Task) error {
	query := `UPDATE tasks SET title=?, description=?, status=?, priority=?, start_date=?, end_date=?, updated_at=? 
			  WHERE id=?`
	
	task.UpdatedAt = time.Now()
	
	_, err := s.db.Exec(query, task.Title, task.Description, task.Status, task.Priority,
		task.StartDate, task.EndDate, task.UpdatedAt, id)
	
	return err
}

func (s *TaskService) DeleteTask(id int64) error {
	query := `DELETE FROM tasks WHERE id=?`
	_, err := s.db.Exec(query, id)
	return err
}
