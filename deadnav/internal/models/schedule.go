package models

import "time"

// Schedule represents a concrete time block in the user's calendar for a task.
// There is at most one Schedule per Task (enforced by a unique key on task_id).
type Schedule struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
}

// ScheduleFilter holds optional filter criteria for querying schedules.
type ScheduleFilter struct {
	UserID int64
	From   time.Time
	To     time.Time
	Status string
	Order  string
}

// HasFilters returns true if at least one filter criterion is set.
func (f ScheduleFilter) HasFilters() bool {
	return !f.From.IsZero() || !f.To.IsZero() || f.Status != "" || f.Order != ""
}

// ScheduleSlot is a free (unoccupied) time interval returned by the
// free-slot query. It carries no database identity.
type ScheduleSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}
