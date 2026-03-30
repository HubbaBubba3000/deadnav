package services

import (
	"database/sql"
	"deadnav/internal/models"
)

type StatisticsService struct {
	db *sql.DB
}

func NewStatisticsService(db *sql.DB) *StatisticsService {
	return &StatisticsService{db: db}
}

func (s *StatisticsService) GetStatistics() (*models.Statistics, error) {
	stats := &models.Statistics{
		TasksByStatus:   make(map[string]int64),
		TasksByPriority: make(map[int]int64),
	}

	// Total tasks
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&stats.TotalTasks)
	if err != nil {
		return nil, err
	}

	// Completed tasks
	err = s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'completed'`).Scan(&stats.CompletedTasks)
	if err != nil {
		return nil, err
	}

	// Pending tasks
	err = s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status IN ('pending', 'in_progress')`).Scan(&stats.PendingTasks)
	if err != nil {
		return nil, err
	}

	// Average duration
	err = s.db.QueryRow(`SELECT AVG(TIMESTAMPDIFF(HOUR, start_date, end_date)) FROM tasks WHERE status = 'completed'`).Scan(&stats.AvgDuration)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Tasks by status
	rows, err := s.db.Query(`SELECT status, COUNT(*) as count FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.TasksByStatus[status] = count
	}

	// Tasks by priority
	rows, err = s.db.Query(`SELECT priority, COUNT(*) as count FROM tasks GROUP BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var priority int
		var count int64
		if err := rows.Scan(&priority, &count); err != nil {
			return nil, err
		}
		stats.TasksByPriority[priority] = count
	}

	return stats, nil
}
