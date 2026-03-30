package models

import "time"

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Schedule struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type Statistics struct {
	TotalTasks     int64              `json:"total_tasks"`
	CompletedTasks int64              `json:"completed_tasks"`
	PendingTasks   int64              `json:"pending_tasks"`
	AvgDuration    float64            `json:"avg_duration"`
	TasksByStatus  map[string]int64   `json:"tasks_by_status"`
	TasksByPriority map[int]int64     `json:"tasks_by_priority"`
}
