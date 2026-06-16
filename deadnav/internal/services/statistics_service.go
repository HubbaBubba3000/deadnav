package services

import (
	"database/sql"
	"deadnav/internal/models"
	"deadnav/internal/models/dto"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
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

// getCachedMonthlyStatistics retrieves monthly statistics from the cache.
// Returns sql.ErrNoRows if no statistics are found for the user.
func (s *StatisticsService) getCachedMonthlyStatistics(userID int64, year int, month int) (*dto.MonthlyStatistics, error) {
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	monthlyStats := &dto.MonthlyStatistics{
		Month:   monthStart,
		Heatmap: make([]dto.HeatmapDay, 0),
	}

	// Get cached statistics from database. The (user_id, month) primary key
	// guarantees at most one row, so a range scan is the cheapest lookup.
	if err := s.db.QueryRow(
		`SELECT total_tasks, completed_tasks, overdue_completed, moved_deadlines
		 FROM user_statistics
		 WHERE user_id = ? AND month = ?`,
		userID, monthStart).Scan(
		&monthlyStats.TotalTasks, &monthlyStats.CompletedTasks,
		&monthlyStats.OverdueCompleted, &monthlyStats.MovedDeadlines); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	// Read heatmap rows once into a map keyed by the canonical date string.
	rows, err := s.db.Query(
		`SELECT DATE_FORMAT(date, '%Y-%m-%d'), value
		 FROM heatmap_data
		 WHERE user_id = ? AND date >= ? AND date < ?
		 ORDER BY date`,
		userID, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	heatmapByDate := make(map[string]float64)
	for rows.Next() {
		var dateKey string
		var value float64
		if err := rows.Scan(&dateKey, &value); err != nil {
			rows.Close()
			return nil, err
		}
		heatmapByDate[dateKey] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	daysInMonth := monthEnd.AddDate(0, 0, -1).Day()
	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		key := date.Format("2006-01-02")
		value, ok := heatmapByDate[key]
		if !ok {
			value = 0
		}
		monthlyStats.Heatmap = append(monthlyStats.Heatmap, dto.HeatmapDay{
			Date:  date,
			Value: value,
		})
	}

	return monthlyStats, nil
}

// CreateMonthlyStatistics creates new monthly statistics for a user.
// If statistics already exist, they will be overwritten.
func (s *StatisticsService) CreateMonthlyStatistics(userID int64, stats *dto.MonthlyStatistics, year int, month int) error {
	return s.updateMonthlyStatistics(userID, stats, year, month)
}

// UpdateMonthlyStatistics updates existing monthly statistics for a user.
// If statistics don't exist, they will be created.
func (s *StatisticsService) UpdateMonthlyStatistics(userID int64, stats *dto.MonthlyStatistics, year int, month int) error {
	return s.updateMonthlyStatistics(userID, stats, year, month)
}

// updateMonthlyStatistics is a private helper method that performs the actual database operations.
func (s *StatisticsService) updateMonthlyStatistics(userID int64, stats *dto.MonthlyStatistics, year int, month int) error {
	if month < 1 || month > 12 {
		return fmt.Errorf("invalid month: %d", month)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("updateMonthlyStatistics: begin transaction: %w", err)
	}
	defer tx.Rollback()

	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	_, err = tx.Exec(
		`INSERT INTO user_statistics (user_id, month, total_tasks, completed_tasks, overdue_completed, moved_deadlines)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
			 total_tasks       = VALUES(total_tasks),
			 completed_tasks   = VALUES(completed_tasks),
			 overdue_completed = VALUES(overdue_completed),
			 moved_deadlines   = VALUES(moved_deadlines),
			 updated_at        = CURRENT_TIMESTAMP`,
		userID, monthStart, stats.TotalTasks, stats.CompletedTasks,
		stats.OverdueCompleted, stats.MovedDeadlines)
	if err != nil {
		return fmt.Errorf("updateMonthlyStatistics: upsert user_statistics: %w", err)
	}

	_, err = tx.Exec(
		`DELETE FROM heatmap_data
		 WHERE user_id = ? AND date >= ? AND date < ?`,
		userID, monthStart, monthEnd)
	if err != nil {
		return fmt.Errorf("updateMonthlyStatistics: delete heatmap_data: %w", err)
	}

	if len(stats.Heatmap) > 0 {
		args := make([]any, 0, len(stats.Heatmap)*3)
		placeholders := make([]byte, 0, len(stats.Heatmap)*8)
		for i, day := range stats.Heatmap {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, []byte("(?, ?, ?)")...)
			date := day.Date.UTC()
			dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
			args = append(args, userID, dayStart, day.Value)
		}

		_, err = tx.Exec(
			`INSERT INTO heatmap_data (user_id, date, value) VALUES `+string(placeholders),
			args...,
		)
		if err != nil {
			return fmt.Errorf("updateMonthlyStatistics: insert heatmap_data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("updateMonthlyStatistics: commit transaction: %w", err)
	}

	return nil
}

// DeleteMonthlyStatistics removes all monthly statistics for a user.
func (s *StatisticsService) DeleteMonthlyStatistics(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteMonthlyStatistics: begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM user_statistics WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("DeleteMonthlyStatistics: delete user_statistics: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM heatmap_data WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("DeleteMonthlyStatistics: delete heatmap_data: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteMonthlyStatistics: commit transaction: %w", err)
	}

	return nil
}

func (s *StatisticsService) GetMonthlyStatistics(
	userID int64,
	year, month int,
) (*dto.MonthlyStatistics, error) {

	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month: %d", month)
	}

	if year == 0 || month == 0 {
		now := time.Now().UTC()
		year, month = now.Year(), int(now.Month())
	}

	stats, err := s.getCachedMonthlyStatistics(userID, year, month)
	if err == nil {
		return stats, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("cache lookup: %w", err)
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	result := &dto.MonthlyStatistics{
		Month:   start,
		Heatmap: make([]dto.HeatmapDay, 0, 31),
	}

	// --- агрегированные метрики (один запрос вместо трёх) ---
	err = s.db.QueryRow(`
		SELECT
			COUNT(*) AS total,
			SUM(status = 'completed') AS completed,
			SUM(status = 'completed' AND updated_at > end_date) AS overdue
		FROM tasks
		WHERE user_id = ?
		  AND end_date >= ? AND end_date < ?
	`, userID, start, end).Scan(
		&result.TotalTasks,
		&result.CompletedTasks,
		&result.OverdueCompleted,
	)
	if err != nil {
		return nil, fmt.Errorf("GetMonthlyStatistics: aggregate stats: %w", err)
	}

	// --- moved deadlines ---
	// Используем колонку moved_deadline из таблицы tasks, которую UpdateTask
	// выставляет в TRUE при каждом изменении end_date. Таблица task_history
	// в схеме отсутствует, поэтому JOIN к ней не нужен.
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM tasks
		WHERE user_id = ?
		  AND status = 'completed'
		  AND moved_deadline = TRUE
		  AND updated_at >= ? AND updated_at < ?
	`, userID, start, end).Scan(&result.MovedDeadlines); err != nil {
		return nil, fmt.Errorf("GetMonthlyStatistics: moved deadlines: %w", err)
	}

	// --- heatmap (один запрос вместо 2×N) ---
	rows, err := s.db.Query(`
		SELECT
			DATE(end_date) AS day,
			COUNT(*) AS planned,
			SUM(status = 'completed') AS completed
		FROM tasks
		WHERE user_id = ?
		  AND end_date >= ? AND end_date < ?
		GROUP BY day
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("GetMonthlyStatistics: heatmap query: %w", err)
	}
	defer rows.Close()

	heatmap := make(map[string]dto.HeatmapDay)
	for rows.Next() {
		var day string
		var planned, completed int64
		if err := rows.Scan(&day, &planned, &completed); err != nil {
			return nil, fmt.Errorf("GetMonthlyStatistics: heatmap scan: %w", err)
		}
		value := 0.0
		if planned > 0 {
			value = float64(completed) / float64(planned)
			if value > 1 {
				value = 1
			}
		}
		date, _ := time.Parse("2006-01-02", day)
		heatmap[day] = dto.HeatmapDay{Date: date, Value: value}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetMonthlyStatistics: heatmap rows: %w", err)
	}

	// Заполняем все дни месяца, подставляя 0 для дней без задач.
	days := end.AddDate(0, 0, -1).Day()
	for d := 1; d <= days; d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
		key := date.Format("2006-01-02")
		if v, ok := heatmap[key]; ok {
			result.Heatmap = append(result.Heatmap, v)
		} else {
			result.Heatmap = append(result.Heatmap, dto.HeatmapDay{Date: date, Value: 0})
		}
	}

	return result, nil
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

	r, err := s.db.Query(
		`SELECT status, COUNT(*) FROM tasks WHERE user_id = ? GROUP BY status`, userID,
	)
	if err != nil {
		return err
	}
	defer r.Close()
	for r.Next() {
		var status string
		var count int64
		if err := r.Scan(&status, &count); err != nil {
			return err
		}
		stats.TasksByStatus[status] = count
	}
	if err := r.Err(); err != nil {
		return err
	}

	r2, err := s.db.Query(
		`SELECT priority, COUNT(*) FROM tasks WHERE user_id = ? GROUP BY priority`, userID,
	)
	if err != nil {
		return err
	}
	defer r2.Close()
	for r2.Next() {
		var priority int
		var count int64
		if err := r2.Scan(&priority, &count); err != nil {
			return err
		}
		stats.TasksByPriority[priority] = count
	}
	return nil
}

func (s *StatisticsService) computeDurationStats(stats *models.Statistics, userID int64) error {
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

	r, err := s.db.Query(
		`SELECT TIMESTAMPDIFF(HOUR, start_date, end_date)
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND end_date > start_date
		 ORDER BY 1`, userID,
	)
	if err != nil {
		return err
	}
	defer r.Close()

	var durations []float64
	for r.Next() {
		var d float64
		if err := r.Scan(&d); err != nil {
			return err
		}
		durations = append(durations, d)
	}
	if err := r.Err(); err != nil {
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

	r2, err := s.db.Query(
		`SELECT priority, AVG(TIMESTAMPDIFF(HOUR, start_date, end_date))
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND end_date > start_date
		 GROUP BY priority`, userID,
	)
	if err != nil {
		return err
	}
	defer r2.Close()
	for r2.Next() {
		var priority int
		var avg float64
		if err := r2.Scan(&priority, &avg); err != nil {
			return err
		}
		stats.AvgDurationByPriority[priority] = avg
	}
	return nil
}

func (s *StatisticsService) computeTimeBasedStats(stats *models.Statistics, userID int64) error {
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ? AND end_date < NOW()
		   AND status NOT IN ('completed','cancelled')`, userID,
	).Scan(&stats.OverdueTasks); err != nil {
		return err
	}

	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ?
		   AND end_date BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 7 DAY)
		   AND status NOT IN ('completed','cancelled')`, userID,
	).Scan(&stats.UpcomingDeadlines); err != nil {
		return err
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(AVG(TIMESTAMPDIFF(HOUR, end_date, updated_at)), 0)
		 FROM tasks
		 WHERE user_id = ? AND status = 'completed' AND updated_at > end_date`, userID,
	).Scan(&stats.AvgDelayHours); err != nil {
		return err
	}

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
	var last30Days int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks
		 WHERE user_id = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)`, userID,
	).Scan(&last30Days); err != nil {
		return err
	}
	stats.AvgTasksPerDay = math.Round(float64(last30Days)/30*100) / 100

	r, err := s.db.Query(
		`SELECT DAYNAME(created_at) AS day_name, COUNT(*)
		 FROM tasks
		 WHERE user_id = ?
		 GROUP BY DAYOFWEEK(created_at), DAYNAME(created_at)
		 ORDER BY DAYOFWEEK(created_at)`, userID,
	)
	if err != nil {
		return err
	}
	defer r.Close()

	var peakDay string
	var peakCount int64
	for r.Next() {
		var day string
		var count int64
		if err := r.Scan(&day, &count); err != nil {
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
