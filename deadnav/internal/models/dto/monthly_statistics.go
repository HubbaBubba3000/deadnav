package dto

import (
	"time"
)

// MonthlyStatistics represents a monthly report for a user's tasks
// Includes completed on-time tasks, overdue completed tasks, tasks with moved deadlines,
// and a heatmap of daily task completion
type MonthlyStatistics struct {
	// Month stores the first day of the month this report refers to.
	// Only the year and month components are meaningful.
	Month time.Time `json:"month"`
	// Basic counts for the month
	TotalTasks       int64 `json:"total_tasks"`
	CompletedTasks   int64 `json:"completed_tasks"`
	OverdueCompleted int64 `json:"overdue_completed"`
	MovedDeadlines   int64 `json:"moved_deadlines"`

	// Heatmap data for the month
	Heatmap []HeatmapDay `json:"heatmap"`
}

// HeatmapDay represents a single day in the heatmap
// Color values range from 0 (red - no tasks completed) to 1 (green - all tasks completed)
// Intermediate values represent gradient colors
type HeatmapDay struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"` // 0-1, where 0=red, 1=green
}
