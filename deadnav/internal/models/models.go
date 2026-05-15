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
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	TelegramID   *int64     `json:"telegram_id,omitempty"`
	AuthProvider string     `json:"auth_provider"` // "local" or "telegram"
	CreatedAt    time.Time  `json:"created_at"`
}

type Schedule struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type Statistics struct {
	// Basic counts
	TotalTasks       int64            `json:"total_tasks"`
	CompletedTasks   int64            `json:"completed_tasks"`
	PendingTasks     int64            `json:"pending_tasks"`
	InProgressTasks  int64            `json:"in_progress_tasks"`
	CancelledTasks   int64            `json:"cancelled_tasks"`
	AvgDuration      float64          `json:"avg_duration_hours"`
	TasksByStatus    map[string]int64 `json:"tasks_by_status"`
	TasksByPriority  map[int]int64    `json:"tasks_by_priority"`

	// Time-based analytics
	OverdueTasks        int64   `json:"overdue_tasks"`        // tasks with end_date < NOW() and not completed/cancelled
	UpcomingDeadlines   int64   `json:"upcoming_deadlines"`   // tasks ending within 7 days
	AvgDelayHours       float64 `json:"avg_delay_hours"`      // average delay for overdue completed tasks
	OnTimeCompletionRate float64 `json:"on_time_completion_rate"` // percentage of tasks completed before deadline

	// Duration analytics
	MedianDuration     float64 `json:"median_duration_hours"`
	MinDuration        float64 `json:"min_duration_hours"`
	MaxDuration        float64 `json:"max_duration_hours"`
	AvgDurationByPriority map[int]float64 `json:"avg_duration_by_priority"`

	// Trend analytics
	TasksCreatedThisWeek   int64 `json:"tasks_created_this_week"`
	TasksCompletedThisWeek int64 `json:"tasks_completed_this_week"`
	TasksCreatedLastWeek   int64 `json:"tasks_created_last_week"`
	TasksCompletedLastWeek int64 `json:"tasks_completed_last_week"`
	CompletionTrend       string `json:"completion_trend"` // "improving", "declining", "stable"

	// Priority analytics
	HighPriorityTasks    int64   `json:"high_priority_tasks"`    // priority 4-5
	LowPriorityTasks     int64   `json:"low_priority_tasks"`     // priority 1-2
	HighPriorityCompletionRate float64 `json:"high_priority_completion_rate"`
	LowPriorityCompletionRate  float64 `json:"low_priority_completion_rate"`

	// Workload analytics
	AvgTasksPerDay   float64 `json:"avg_tasks_per_day"`   // last 30 days
	PeakDay          string  `json:"peak_day"`            // day of week with most tasks created
	TasksByDayOfWeek map[string]int64 `json:"tasks_by_day_of_week"`

	// User analytics (if users exist)
	TotalUsers         int64   `json:"total_users"`
	ActiveUsers        int64   `json:"active_users"` // users who created/updated tasks recently
	NewUsersThisMonth  int64   `json:"new_users_this_month"`

	// Performance score
	ProductivityScore float64 `json:"productivity_score"` // 0-100 composite score
}
