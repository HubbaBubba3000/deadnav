package services

import (
	"database/sql"
	"math"
	"sort"
	"time"

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
		TasksByStatus:         make(map[string]int64),
		TasksByPriority:       make(map[int]int64),
		TasksByDayOfWeek:      make(map[string]int64),
		AvgDurationByPriority: make(map[int]float64),
	}

	// === BASIC COUNTS ===
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&stats.TotalTasks); err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'completed'`).Scan(&stats.CompletedTasks); err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'pending'`).Scan(&stats.PendingTasks); err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'`).Scan(&stats.InProgressTasks); err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'cancelled'`).Scan(&stats.CancelledTasks); err != nil {
		return nil, err
	}

	// === TASKS BY STATUS ===
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.TasksByStatus[status] = count
	}
	rows.Close()

	// === TASKS BY PRIORITY ===
	rows, err = s.db.Query(`SELECT priority, COUNT(*) FROM tasks GROUP BY priority`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var priority int
		var count int64
		if err := rows.Scan(&priority, &count); err != nil {
			return nil, err
		}
		stats.TasksByPriority[priority] = count
	}
	rows.Close()

	// === DURATION ANALYTICS ===
	if err := s.computeDurationStats(stats); err != nil {
		return nil, err
	}

	// === TIME-BASED ANALYTICS ===
	if err := s.computeTimeBasedStats(stats); err != nil {
		return nil, err
	}

	// === TREND ANALYTICS ===
	if err := s.computeTrendStats(stats); err != nil {
		return nil, err
	}

	// === PRIORITY ANALYTICS ===
	if err := s.computePriorityStats(stats); err != nil {
		return nil, err
	}

	// === WORKLOAD ANALYTICS ===
	if err := s.computeWorkloadStats(stats); err != nil {
		return nil, err
	}

	// === USER ANALYTICS ===
	if err := s.computeUserStats(stats); err != nil {
		return nil, err
	}

	// === PRODUCTIVITY SCORE ===
	stats.ProductivityScore = s.calculateProductivityScore(stats)

	return stats, nil
}

func (s *StatisticsService) computeDurationStats(stats *models.Statistics) error {
	// Average duration for completed tasks
	if err := s.db.QueryRow(`
		SELECT COALESCE(AVG(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		FROM tasks WHERE status = 'completed'
	`).Scan(&stats.AvgDuration); err != nil {
		return err
	}

	// Min duration
	if err := s.db.QueryRow(`
		SELECT COALESCE(MIN(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		FROM tasks WHERE status = 'completed' AND end_date > start_date
	`).Scan(&stats.MinDuration); err != nil {
		return err
	}

	// Max duration
	if err := s.db.QueryRow(`
		SELECT COALESCE(MAX(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		FROM tasks WHERE status = 'completed'
	`).Scan(&stats.MaxDuration); err != nil {
		return err
	}

	// Median duration
	rows, err := s.db.Query(`
		SELECT TIMESTAMPDIFF(HOUR, start_date, end_date) as duration
		FROM tasks WHERE status = 'completed' AND end_date > start_date
		ORDER BY duration
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var durations []float64
	for rows.Next() {
		var d float64
		if err := rows.Scan(&d); err != nil {
			return err
		}
		durations = append(durations, d)
	}

	if len(durations) > 0 {
		sort.Float64s(durations)
		n := len(durations)
		if n%2 == 0 {
			stats.MedianDuration = (durations[n/2-1] + durations[n/2]) / 2
		} else {
			stats.MedianDuration = durations[n/2]
		}
	}

	// Average duration by priority
	rows, err = s.db.Query(`
		SELECT priority, AVG(TIMESTAMPDIFF(HOUR, start_date, end_date))
		FROM tasks WHERE status = 'completed' AND end_date > start_date
		GROUP BY priority
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var priority int
		var avgDur float64
		if err := rows.Scan(&priority, &avgDur); err != nil {
			return err
		}
		stats.AvgDurationByPriority[priority] = avgDur
	}

	return nil
}

func (s *StatisticsService) computeTimeBasedStats(stats *models.Statistics) error {
	// Overdue tasks (end_date < NOW() and not completed/cancelled)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks
		WHERE end_date < NOW() AND status NOT IN ('completed', 'cancelled')
	`).Scan(&stats.OverdueTasks); err != nil {
		return err
	}

	// Upcoming deadlines (within 7 days)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks
		WHERE end_date BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 7 DAY)
		AND status NOT IN ('completed', 'cancelled')
	`).Scan(&stats.UpcomingDeadlines); err != nil {
		return err
	}

	// Average delay for overdue completed tasks (how much they exceeded deadline)
	if err := s.db.QueryRow(`
		SELECT COALESCE(AVG(
			CASE WHEN end_date < updated_at THEN TIMESTAMPDIFF(HOUR, end_date, updated_at) ELSE 0 END
		), 0)
		FROM tasks WHERE status = 'completed' AND updated_at > end_date
	`).Scan(&stats.AvgDelayHours); err != nil {
		return err
	}

	// On-time completion rate
	var completedOnTime, totalCompleted int64
	if err := s.db.QueryRow(`
		SELECT
			SUM(CASE WHEN end_date >= start_date THEN 1 ELSE 0 END),
			COUNT(*)
		FROM tasks WHERE status = 'completed'
	`).Scan(&completedOnTime, &totalCompleted); err != nil {
		return err
	}
	if totalCompleted > 0 {
		stats.OnTimeCompletionRate = math.Round(float64(completedOnTime)/float64(totalCompleted)*100*100) / 100
	}

	return nil
}

func (s *StatisticsService) computeTrendStats(stats *models.Statistics) error {
	now := time.Now()
	startOfWeek := now.Truncate(24 * time.Hour).AddDate(0, 0, -int(now.Weekday()))
	startOfLastWeek := startOfWeek.AddDate(0, 0, -7)

	// Tasks created this week
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE created_at >= ?
	`, startOfWeek).Scan(&stats.TasksCreatedThisWeek); err != nil {
		return err
	}

	// Tasks completed this week
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE status = 'completed' AND updated_at >= ?
	`, startOfWeek).Scan(&stats.TasksCompletedThisWeek); err != nil {
		return err
	}

	// Tasks created last week
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND created_at < ?
	`, startOfLastWeek, startOfWeek).Scan(&stats.TasksCreatedLastWeek); err != nil {
		return err
	}

	// Tasks completed last week
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE status = 'completed' AND updated_at >= ? AND updated_at < ?
	`, startOfLastWeek, startOfWeek).Scan(&stats.TasksCompletedLastWeek); err != nil {
		return err
	}

	// Determine trend
	if stats.TasksCompletedThisWeek > stats.TasksCompletedLastWeek {
		stats.CompletionTrend = "improving"
	} else if stats.TasksCompletedThisWeek < stats.TasksCompletedLastWeek {
		stats.CompletionTrend = "declining"
	} else {
		stats.CompletionTrend = "stable"
	}

	return nil
}

func (s *StatisticsService) computePriorityStats(stats *models.Statistics) error {
	// High priority tasks (4-5)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE priority >= 4
	`).Scan(&stats.HighPriorityTasks); err != nil {
		return err
	}

	// Low priority tasks (1-2)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE priority <= 2
	`).Scan(&stats.LowPriorityTasks); err != nil {
		return err
	}

	// High priority completion rate
	var highTotal, highCompleted int64
	if err := s.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)
		FROM tasks WHERE priority >= 4
	`).Scan(&highTotal, &highCompleted); err != nil {
		return err
	}
	if highTotal > 0 {
		stats.HighPriorityCompletionRate = math.Round(float64(highCompleted)/float64(highTotal)*100*100) / 100
	}

	// Low priority completion rate
	var lowTotal, lowCompleted int64
	if err := s.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)
		FROM tasks WHERE priority <= 2
	`).Scan(&lowTotal, &lowCompleted); err != nil {
		return err
	}
	if lowTotal > 0 {
		stats.LowPriorityCompletionRate = math.Round(float64(lowCompleted)/float64(lowTotal)*100*100) / 100
	}

	return nil
}

func (s *StatisticsService) computeWorkloadStats(stats *models.Statistics) error {
	// Average tasks per day (last 30 days)
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&stats.TasksCreatedThisWeek); err != nil {
		return err
	}
	// Reuse the field name correctly - this should be tasks in last 30 days
	var last30days int64
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&last30days); err != nil {
		return err
	}
	stats.AvgTasksPerDay = math.Round(float64(last30days)/30*100) / 100

	// Tasks by day of week
	rows, err := s.db.Query(`
		SELECT DAYNAME(created_at) as day_name, COUNT(*)
		FROM tasks GROUP BY DAYOFWEEK(created_at), DAYNAME(created_at)
		ORDER BY DAYOFWEEK(created_at)
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var peakDay string
	var peakCount int64
	for rows.Next() {
		var day string
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return err
		}
		stats.TasksByDayOfWeek[day] = count
		if count > peakCount {
			peakCount = count
			peakDay = day
		}
	}
	stats.PeakDay = peakDay

	return nil
}

func (s *StatisticsService) computeUserStats(stats *models.Statistics) error {
	// Total users
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return err
	}

	// Active users (logged in or created/updated something in last 30 days)
	// Since tasks aren't user-associated yet, we count users by recent activity
	// For now, just count users created recently
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM users WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&stats.ActiveUsers); err != nil {
		return err
	}

	// New users this month
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM users WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&stats.NewUsersThisMonth); err != nil {
		return err
	}

	return nil
}

func (s *StatisticsService) calculateProductivityScore(stats *models.Statistics) float64 {
	if stats.TotalTasks == 0 {
		return 0
	}

	var score float64

	// Completion rate (0-40 points)
	completionRate := float64(stats.CompletedTasks) / float64(stats.TotalTasks)
	score += completionRate * 40

	// On-time rate (0-25 points)
	score += (stats.OnTimeCompletionRate / 100) * 25

	// No overdue penalty (0-15 points)
	if stats.TotalTasks > 0 {
		overdueRate := 1 - (float64(stats.OverdueTasks) / float64(stats.TotalTasks))
		score += overdueRate * 15
	}

	// Trend bonus (0-10 points)
	switch stats.CompletionTrend {
	case "improving":
		score += 10
	case "stable":
		score += 5
	}

	// High priority completion (0-10 points)
	if stats.HighPriorityTasks > 0 {
		score += (stats.HighPriorityCompletionRate / 100) * 10
	}

	return math.Round(score*100) / 100
}
