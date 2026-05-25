package models

import "time"

// Task represents a unit of work with a time window and deadline.
//
// StartDate is the earliest the task can be scheduled (window open).
// EndDate is the deadline by which the task must be completed.
// DurationMinutes is the estimated work time in minutes, used by the scheduler.
// When DurationMinutes is 0, the scheduler derives the duration from EndDate − StartDate.
// EstimatedMinutes is an additional estimation based on complexity/urgency/importance.
type Task struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Title            string    `json:"title" binding:"required,min=1,max=255"`
	Description      string    `json:"description" binding:"max=2000"`
	Status           string    `json:"status" binding:"required,oneof=pending in_progress completed cancelled"` // pending | in_progress | completed | cancelled
	Priority         int       `json:"priority" binding:"required,min=1,max=5"`                                 // 1 (low) – 5 (high)
	DurationMinutes  int       `json:"duration_minutes" binding:"min=0,max=480"`                                // 0 = auto-calculated from dates
	StartDate        time.Time `json:"start_date" binding:"required,ltefield=EndDate"`                          // earliest start / window open
	EndDate          time.Time `json:"end_date" binding:"required"`                                             // deadline
	Complexity       int       `json:"complexity" binding:"required,min=1,max=5"`                               // 1 (low) – 5 (high)
	Urgency          int       `json:"urgency" binding:"required,min=1,max=5"`                                  // 1 (low) – 5 (high)
	Importance       int       `json:"importance" binding:"required,min=1,max=5"`                               // 1 (low) – 5 (high)
	EstimatedMinutes int       `json:"estimated_minutes"`                                                       // estimated time in minutes (derived from complexity/urgency/importance)
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
