package models

// Statistics aggregates analytical data for a single user's tasks.
// User-level fields (TotalUsers, ActiveUsers, NewUsersThisMonth) are
// populated only when queried without a specific user context.
type Statistics struct {
	// ── Basic counts ──────────────────────────────────────────────────────────
	TotalTasks      int64            `json:"total_tasks"`
	CompletedTasks  int64            `json:"completed_tasks"`
	PendingTasks    int64            `json:"pending_tasks"`
	InProgressTasks int64            `json:"in_progress_tasks"`
	CancelledTasks  int64            `json:"cancelled_tasks"`
	TasksByStatus   map[string]int64 `json:"tasks_by_status"`
	TasksByPriority map[int]int64    `json:"tasks_by_priority"`

	// ── Time-based analytics ──────────────────────────────────────────────────
	OverdueTasks         int64   `json:"overdue_tasks"`           // not done and past deadline
	UpcomingDeadlines    int64   `json:"upcoming_deadlines"`      // ending within 7 days
	AvgDelayHours        float64 `json:"avg_delay_hours"`         // avg overrun for late completions
	OnTimeCompletionRate float64 `json:"on_time_completion_rate"` // % completed before deadline

	// ── Duration analytics ────────────────────────────────────────────────────
	AvgDuration           float64         `json:"avg_duration_hours"`
	MedianDuration        float64         `json:"median_duration_hours"`
	MinDuration           float64         `json:"min_duration_hours"`
	MaxDuration           float64         `json:"max_duration_hours"`
	AvgDurationByPriority map[int]float64 `json:"avg_duration_by_priority"`

	// ── Trend analytics ───────────────────────────────────────────────────────
	TasksCreatedThisWeek   int64  `json:"tasks_created_this_week"`
	TasksCompletedThisWeek int64  `json:"tasks_completed_this_week"`
	TasksCreatedLastWeek   int64  `json:"tasks_created_last_week"`
	TasksCompletedLastWeek int64  `json:"tasks_completed_last_week"`
	CompletionTrend        string `json:"completion_trend"` // "improving" | "declining" | "stable"

	// ── Priority analytics ────────────────────────────────────────────────────
	HighPriorityTasks          int64   `json:"high_priority_tasks"` // priority 4–5
	LowPriorityTasks           int64   `json:"low_priority_tasks"`  // priority 1–2
	HighPriorityCompletionRate float64 `json:"high_priority_completion_rate"`
	LowPriorityCompletionRate  float64 `json:"low_priority_completion_rate"`

	// ── Workload analytics ────────────────────────────────────────────────────
	AvgTasksPerDay   float64          `json:"avg_tasks_per_day"` // last 30 days
	PeakDay          string           `json:"peak_day"`          // weekday with most tasks created
	TasksByDayOfWeek map[string]int64 `json:"tasks_by_day_of_week"`

	// ── User analytics (global, not per-user) ─────────────────────────────────
	TotalUsers        int64 `json:"total_users"`
	ActiveUsers       int64 `json:"active_users"` // registered in last 30 days
	NewUsersThisMonth int64 `json:"new_users_this_month"`

	// ── Composite score ───────────────────────────────────────────────────────
	ProductivityScore float64 `json:"productivity_score"` // 0–100
}
