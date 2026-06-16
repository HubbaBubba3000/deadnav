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
	MovedDeadline    bool      `json:"moved_deadline"`
}

// TaskUpdate is a partial update payload for a task. Only fields whose
// pointer is non-nil are applied; nil fields are left unchanged. This
// matches the PATCH semantics used by the service layer.
type TaskUpdate struct {
	Title            *string    `json:"title,omitempty"            binding:"omitempty,min=1,max=255"`
	Description      *string    `json:"description,omitempty"      binding:"omitempty,max=2000"`
	Status           *string    `json:"status,omitempty"           binding:"omitempty,oneof=pending in_progress completed cancelled"`
	Priority         *int       `json:"priority,omitempty"         binding:"omitempty,min=1,max=5"`
	DurationMinutes  *int       `json:"duration_minutes,omitempty" binding:"omitempty,min=0,max=480"`
	StartDate        *time.Time `json:"start_date,omitempty"        binding:"omitempty"`
	EndDate          *time.Time `json:"end_date,omitempty"          binding:"omitempty"`
	Complexity       *int       `json:"complexity,omitempty"       binding:"omitempty,min=1,max=5"`
	Urgency          *int       `json:"urgency,omitempty"          binding:"omitempty,min=1,max=5"`
	Importance       *int       `json:"importance,omitempty"       binding:"omitempty,min=1,max=5"`
	EstimatedMinutes *int       `json:"estimated_minutes,omitempty"`
}

// HasAny reports whether at least one field is set on the patch.
func (u *TaskUpdate) HasAny() bool {
	return u.Title != nil || u.Description != nil || u.Status != nil ||
		u.Priority != nil || u.DurationMinutes != nil ||
		u.StartDate != nil || u.EndDate != nil ||
		u.Complexity != nil || u.Urgency != nil || u.Importance != nil ||
		u.EstimatedMinutes != nil
}

// AffectsEstimation reports whether the patch changes the inputs that
// drive the derived estimated_minutes value.
func (u *TaskUpdate) AffectsEstimation() bool {
	return u.Complexity != nil || u.Urgency != nil || u.Importance != nil
}

// TaskFilter holds optional filter criteria for querying tasks.
type TaskFilter struct {
	UserID        int64
	Status        string
	Priority      *int
	StartDateFrom time.Time
	StartDateTo   time.Time
	EndDateFrom   time.Time
	EndDateTo     time.Time
}

// HasFilters returns true if at least one filter criterion is set.
func (f TaskFilter) HasFilters() bool {
	return f.Status != "" || f.Priority != nil ||
		!f.StartDateFrom.IsZero() || !f.StartDateTo.IsZero() ||
		!f.EndDateFrom.IsZero() || !f.EndDateTo.IsZero()
}

func CalculateEstimatedTime(complexity, urgency, importance int) int {
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
