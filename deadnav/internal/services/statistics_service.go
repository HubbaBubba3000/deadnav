package services

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"deadnav/internal/models"
)

// StatisticsService computes analytical data for a user's tasks.
type StatisticsService struct {
	db *sql.DB
}

// NewStatisticsService creates a StatisticsService backed by the provided
// database connection.
func NewStatisticsService(db *sql.DB) *StatisticsService {
	return &StatisticsService{db: db}
}

// GetStatistics returns a full statistics report scoped to userID.
func (s *StatisticsService) GetStatistics(userID int64) (*models.Statistics, error) {
	stats := &models.Statistics{
		TasksByStatus:         make(map[string]int64),
		TasksByPriority:       make(map[int]int64),
		TasksByDayOfWeek:      make(map[string]int64),
		AvgDurationByPriority: make(map[int]float64),
	}

	if err := s.computeBasicCounts(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: basic counts: %w", err)
	}
	if err := s.computeDurationStats(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: duration stats: %w", err)
	}
	if err := s.computeTimeBasedStats(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: time-based stats: %w", err)
	}
	if err := s.computeTrendStats(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: trend stats: %w", err)
	}
	if err := s.computePriorityStats(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: priority stats: %w", err)
	}
	if err := s.computeWorkloadStats(stats, userID); err != nil {
		return nil, fmt.Errorf("GetStatistics: workload stats: %w", err)
	}
	if err := s.computeUserStats(stats); err != nil {
		return nil, fmt.Errorf("GetStatistics: user stats: %w", err)
	}

	stats.ProductivityScore = calculateProductivityScore(stats)
	return stats, nil
}

// ---------------------------------------------------------------------------
// Private computation helpers
// ---------------------------------------------------------------------------

func (s *StatisticsService) computeBasicCounts(stats *models.Statistics, userID int64) error {
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE user_id = ?`, userID,
	).Scan(&stats.TotalTasks); err != nil {
		return err
	}

	statusCounts := []struct {
		field  *int64
		status string
	}{
		{&stats.CompletedTasks, "completed"},
		{&stats.PendingTasks, "pending"},
		{&stats.InProgressTasks, "in_progress"},
		{&stats.CancelledTasks, "cancelled"},
	}
	for _, sc := range statusCounts {
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND status = ?`, userID, sc.status,
		).Scan(sc.field); err != nil {
			return err
		}
	}

	// Tasks grouped by status
	rows, err := s.db.Query(
		`SELECT status, COUNT(*) FROM tasks WHERE user_id = ? GROUP BY status`, userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return err
		}
		stats.TasksByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Tasks grouped by priority
	rows2, err := s.db.Query(
		`SELECT priority, COUNT(*) FROM tasks WHERE user_id = ? GROUP BY priority`, userID,
	)
	if err != nil {
		return err
	}
	defer rows2.Close()
	for rows2.Next() {
		var priority int
		var count int64
		if err := rows2.Scan(&priority, &count); err != nil {
			return err
		}
		stats.TasksByPriority[priority] = count
	}
	return rows2.Err()
}

func (s *StatisticsService) computeDurationStats(stats *models.Statistics, userID int64) error {
	// Average duration (completed tasks only).
	if err := s.db.QueryRow(
		`SELECT COALESCE(AVG(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		 FROM tasks WHERE user_id = ? AND status = 'completed'`, userID,
	).Scan(&stats.AvgDuration); err != nil {
		return err
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(MIN(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		 FROM tasks WHERE user_id = ? AND status = 'completed' AND end_date > start_date`, userID,
	).Scan(&stats.MinDuration); err != nil {
		return err
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(TIMESTAMPDIFF(HOUR, start_date, end_date)), 0)
		 FROM tasks WHERE user_id = ? AND status = 'completed'`, userID,
	).Scan(&stats.MaxDuration); err != nil {
		return err
	}

	// Median (requires fetching all durations into Go).
	rows, err := s.db.Query(
		`SELECT TIMESTAMPDIFF(HOUR, start_date, end_date)
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND end_date > start_date
		 ORDER BY 1`, userID,
	)
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
	if err := rows.Err(); err != nil {
		return err
	}

	if n := len(durations); n > 0 {
		sort.Float64s(durations)
		if n%2 == 0 {
			stats.MedianDuration = (durations[n/2-1] + durations[n/2]) / 2
		} else {
			stats.MedianDuration = durations[n/2]
		}
	}

	// Average duration by priority.
	rows2, err := s.db.Query(
		`SELECT priority, AVG(TIMESTAMPDIFF(HOUR, start_date, end_date))
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND end_date > start_date
		 GROUP BY priority`, userID,
	)
	if err != nil {
		return err
	}
	defer rows2.Close()
	for rows2.Next() {
		var priority int
		var avg float64
		if err := rows2.Scan(&priority, &avg); err != nil {
			return err
		}
		stats.AvgDurationByPriority[priority] = avg
	}
	return rows2.Err()
}

func (s *StatisticsService) computeTimeBasedStats(stats *models.Statistics, userID int64) error {
	// Overdue: past deadline, not completed/cancelled.
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ? AND end_date < NOW()
		   AND status NOT IN ('completed','cancelled')`, userID,
	).Scan(&stats.OverdueTasks); err != nil {
		return err
	}

	// Upcoming deadlines (next 7 days).
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ?
		   AND end_date BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 7 DAY)
		   AND status NOT IN ('completed','cancelled')`, userID,
	).Scan(&stats.UpcomingDeadlines); err != nil {
		return err
	}

	// Average delay for tasks that were completed after their deadline.
	if err := s.db.QueryRow(
		`SELECT COALESCE(AVG(TIMESTAMPDIFF(HOUR, end_date, updated_at)), 0)
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND updated_at > end_date`, userID,
	).Scan(&stats.AvgDelayHours); err != nil {
		return err
	}

	// On-time completion rate: completed on or before end_date.
	// "On time" is approximated as updated_at <= end_date.
	var onTime, total int64
	if err := s.db.QueryRow(
		`SELECT
		     COALESCE(SUM(CASE WHEN updated_at <= end_date THEN 1 ELSE 0 END), 0),
		     COUNT(*)
		 FROM tasks WHERE user_id = ? AND status = 'completed'`, userID,
	).Scan(&onTime, &total); err != nil {
		return err
	}
	if total > 0 {
		stats.OnTimeCompletionRate = math.Round(float64(onTime)/float64(total)*100*100) / 100
	}
	return nil
}

func (s *StatisticsService) computeTrendStats(stats *models.Statistics, userID int64) error {
	now := time.Now()
	// Start of current week (Sunday = day 0).
	startOfWeek := now.Truncate(24*time.Hour).AddDate(0, 0, -int(now.Weekday()))
	startOfLastWeek := startOfWeek.AddDate(0, 0, -7)

	queries := []struct {
		dest  *int64
		query string
		args  []interface{}
	}{
		{&stats.TasksCreatedThisWeek,
			`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND created_at >= ?`,
			[]interface{}{userID, startOfWeek}},
		{&stats.TasksCompletedThisWeek,
			`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND status = 'completed' AND updated_at >= ?`,
			[]interface{}{userID, startOfWeek}},
		{&stats.TasksCreatedLastWeek,
			`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND created_at >= ? AND created_at < ?`,
			[]interface{}{userID, startOfLastWeek, startOfWeek}},
		{&stats.TasksCompletedLastWeek,
			`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND status = 'completed' AND updated_at >= ? AND updated_at < ?`,
			[]interface{}{userID, startOfLastWeek, startOfWeek}},
	}
	for _, q := range queries {
		if err := s.db.QueryRow(q.query, q.args...).Scan(q.dest); err != nil {
			return err
		}
	}

	switch {
	case stats.TasksCompletedThisWeek > stats.TasksCompletedLastWeek:
		stats.CompletionTrend = "improving"
	case stats.TasksCompletedThisWeek < stats.TasksCompletedLastWeek:
		stats.CompletionTrend = "declining"
	default:
		stats.CompletionTrend = "stable"
	}
	return nil
}

func (s *StatisticsService) computePriorityStats(stats *models.Statistics, userID int64) error {
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND priority >= 4`, userID,
	).Scan(&stats.HighPriorityTasks); err != nil {
		return err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE user_id = ? AND priority <= 2`, userID,
	).Scan(&stats.LowPriorityTasks); err != nil {
		return err
	}

	// High-priority completion rate.
	var highTotal, highCompleted int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0)
		 FROM tasks WHERE user_id = ? AND priority >= 4`, userID,
	).Scan(&highTotal, &highCompleted); err != nil {
		return err
	}
	if highTotal > 0 {
		stats.HighPriorityCompletionRate = math.Round(float64(highCompleted)/float64(highTotal)*100*100) / 100
	}

	// Low-priority completion rate.
	var lowTotal, lowCompleted int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0)
		 FROM tasks WHERE user_id = ? AND priority <= 2`, userID,
	).Scan(&lowTotal, &lowCompleted); err != nil {
		return err
	}
	if lowTotal > 0 {
		stats.LowPriorityCompletionRate = math.Round(float64(lowCompleted)/float64(lowTotal)*100*100) / 100
	}
	return nil
}

func (s *StatisticsService) computeWorkloadStats(stats *models.Statistics, userID int64) error {
	// Average tasks per day over the last 30 days.
	var last30Days int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)`, userID,
	).Scan(&last30Days); err != nil {
		return err
	}
	stats.AvgTasksPerDay = math.Round(float64(last30Days)/30*100) / 100

	// Tasks by day of week.
	rows, err := s.db.Query(
		`SELECT DAYNAME(created_at) AS day_name, COUNT(*)
		 FROM tasks
		 WHERE user_id = ?
		 GROUP BY DAYOFWEEK(created_at), DAYNAME(created_at)
		 ORDER BY DAYOFWEEK(created_at)`, userID,
	)
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
	return rows.Err()
}

func (s *StatisticsService) computeUserStats(stats *models.Statistics) error {
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)`,
	).Scan(&stats.ActiveUsers); err != nil {
		return err
	}
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users
		 WHERE created_at >= DATE_FORMAT(NOW(), '%Y-%m-01')`,
	).Scan(&stats.NewUsersThisMonth); err != nil {
		return err
	}
	return nil
}

// calculateProductivityScore returns a 0–100 composite productivity score.
func calculateProductivityScore(stats *models.Statistics) float64 {
	if stats.TotalTasks == 0 {
		return 0
	}

	var score float64

	// Completion rate → 0–40 pts
	score += float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 40

	// On-time rate → 0–25 pts
	score += (stats.OnTimeCompletionRate / 100) * 25

	// Low overdue rate → 0–15 pts
	overdueRate := 1 - float64(stats.OverdueTasks)/float64(stats.TotalTasks)
	score += overdueRate * 15

	// Trend bonus → 0–10 pts
	switch stats.CompletionTrend {
	case "improving":
		score += 10
	case "stable":
		score += 5
	}

	// High-priority completion → 0–10 pts
	if stats.HighPriorityTasks > 0 {
		score += (stats.HighPriorityCompletionRate / 100) * 10
	}

	return math.Round(score*100) / 100
}
