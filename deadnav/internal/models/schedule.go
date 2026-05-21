package models

import "time"

// Schedule represents a concrete time block in the user's calendar for a task.
// There is at most one Schedule per Task (enforced by a unique key on task_id).
type Schedule struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
}

// ScheduleSlot is a free (unoccupied) time interval returned by the
// free-slot query. It carries no database identity.
type ScheduleSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}
