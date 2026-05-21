package models

import "time"

// Task represents a unit of work with a time window and deadline.
//
// StartDate is the earliest the task can be scheduled (window open).
// EndDate is the deadline by which the task must be completed.
// DurationMinutes is the estimated work time in minutes.
// When DurationMinutes is 0, the scheduler derives the duration from EndDate − StartDate.
type Task struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`           // pending | in_progress | completed | cancelled
	Priority        int       `json:"priority"`         // 1 (low) – 5 (high)
	DurationMinutes int       `json:"duration_minutes"` // 0 = auto-calculated from dates
	StartDate       time.Time `json:"start_date"`       // earliest start / window open
	EndDate         time.Time `json:"end_date"`         // deadline
	Complexity      int       `json:"complexity"`       // 1 (low) – 5 (high)
	Urgency         int       `json:"urgency"`          // 1 (low) – 5 (high)
	Importance      int       `json:"importance"`       // 1 (low) – 5 (high)
	EstimatedTime   int       `json:"estimated_time"`   // estimated time in minutes
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
