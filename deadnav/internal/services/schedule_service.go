package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadnav/internal/models"
)

// Scheduling constants.
const (
	defaultWorkStartHour  = 9
	defaultWorkEndHour    = 18
	defaultWorkDays       = "Mon,Tue,Wed,Thu,Fri"
	defaultMinSlotMinutes = 30
	defaultTimezone       = "UTC"

	minDurationMinutes = 30  // floor for auto-calculated duration
	maxDurationMinutes = 480 // ceiling for auto-calculated duration (8 hours)
)

// ScheduleService handles all scheduling operations.
type ScheduleService struct {
	db *sql.DB
}

// NewScheduleService creates a ScheduleService backed by the provided database connection.
func NewScheduleService(db *sql.DB) *ScheduleService {
	return &ScheduleService{db: db}
}

// ---------------------------------------------------------------------------
// Public methods
// ---------------------------------------------------------------------------

// AutoScheduleTask finds the first available time slot that satisfies the
// task constraints and the user's working-hour preferences, persists it, and
// returns the resulting Schedule.
//
// The operation is executed inside a serializable transaction to prevent
// double-booking race conditions: between reading existing schedules and
// inserting the new slot, another concurrent request might claim the same
// time window.
func (s *ScheduleService) AutoScheduleTask(task *models.Task, userID int64) (*models.Schedule, error) {
	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	prefs, err := s.getUserPreferencesTx(tx, userID)
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: load preferences: %w", err)
	}

	duration := calculateDuration(task)

	// Collect schedules that could conflict (now → deadline) within the
	// transaction so we hold a consistent view of existing bookings.
	now := time.Now()
	existing, err := s.getUserScheduleRangeTx(tx, userID, now, task.EndDate)
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: fetch existing schedules: %w", err)
	}

	slot, err := findFreeSlot(task, prefs, existing, duration)
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: %w", err)
	}

	// Upsert within the transaction: ON DUPLICATE KEY prevents two rows for
	// the same task, and SERIALIZABLE ensures no other session can insert a
	// conflicting time window between our SELECT and INSERT.
	_, err = tx.Exec(
		`INSERT INTO schedules (task_id, user_id, start_time, end_time)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     start_time = VALUES(start_time),
		     end_time   = VALUES(end_time)`,
		task.ID, userID, slot.StartTime.UTC(), slot.EndTime.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: upsert schedule: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: commit: %w", err)
	}

	// Fetch the canonical row (handles both INSERT and UPDATE paths uniformly).
	sc, err := s.GetTaskSchedule(task.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: fetch saved schedule: %w", err)
	}
	return sc, nil
}

// RemoveSchedule deletes the schedule entry for the given task and user.
func (s *ScheduleService) RemoveSchedule(taskID int64, userID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM schedules WHERE task_id = ? AND user_id = ?`,
		taskID, userID,
	)
	if err != nil {
		return fmt.Errorf("RemoveSchedule: %w", err)
	}
	return nil
}

// GetUserSchedule returns all schedules for the user, ordered by start time.
func (s *ScheduleService) GetUserSchedule(userID int64) ([]models.Schedule, error) {
	return s.GetSchedules(models.ScheduleFilter{UserID: userID})
}

// GetSchedules returns schedules matching the given filter, ordered by start time.
func (s *ScheduleService) GetSchedules(filter models.ScheduleFilter) ([]models.Schedule, error) {
	query := `SELECT id, task_id, user_id, start_time, end_time, created_at
	 FROM schedules
	 WHERE user_id = ?`
	args := []any{filter.UserID}

	if !filter.From.IsZero() {
		query += " AND start_time >= ?"
		args = append(args, filter.From)
	}
	if !filter.To.IsZero() {
		query += " AND end_time <= ?"
		args = append(args, filter.To)
	}

	query += " ORDER BY start_time"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetSchedules: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		return nil, fmt.Errorf("GetSchedules: %w", err)
	}
	return schedules, nil
}

// GetUserScheduleRange returns schedules for the user whose time window
// overlaps [from, to), ordered by start time.
//
// Overlap condition: start_time < to AND end_time > from.
func (s *ScheduleService) GetUserScheduleRange(userID int64, from, to time.Time) ([]models.Schedule, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE user_id = ?
		   AND start_time < ?
		   AND end_time   > ?
		 ORDER BY start_time`,
		userID, to, from,
	)
	if err != nil {
		return nil, fmt.Errorf("GetUserScheduleRange: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		return nil, fmt.Errorf("GetUserScheduleRange: %w", err)
	}
	return schedules, nil
}

// GetTaskSchedule returns the schedule for a specific task owned by userID.
// Returns a wrapped sql.ErrNoRows when no schedule exists.
func (s *ScheduleService) GetTaskSchedule(taskID int64, userID int64) (*models.Schedule, error) {
	var sc models.Schedule
	err := s.db.QueryRow(
		`SELECT id, task_id, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE task_id = ? AND user_id = ?`,
		taskID, userID,
	).Scan(&sc.ID, &sc.TaskID, &sc.UserID, &sc.StartTime, &sc.EndTime, &sc.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("GetTaskSchedule: no schedule for task %d: %w", taskID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("GetTaskSchedule: scan: %w", err)
	}
	return &sc, nil
}

// GetFreeSlots returns every contiguous free slot of at least durationMinutes
// within [from, to] that falls inside the user's configured working hours.
func (s *ScheduleService) GetFreeSlots(userID int64, from, to time.Time, durationMinutes int) ([]models.ScheduleSlot, error) {
	prefs, err := s.getUserPreferences(userID)
	if err != nil {
		return nil, fmt.Errorf("GetFreeSlots: load preferences: %w", err)
	}

	existing, err := s.GetUserScheduleRange(userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetFreeSlots: fetch existing schedules: %w", err)
	}

	loc := loadLocation(prefs.Timezone)
	workDays := parseWorkDays(prefs.WorkDays)
	duration := time.Duration(durationMinutes) * time.Minute

	var slots []models.ScheduleSlot

	current := from.In(loc)
	rangeEnd := to.In(loc)

	for !current.After(rangeEnd) {
		day := dayMidnight(current, loc)

		if workDays[day.Weekday()] {
			dayStart := time.Date(day.Year(), day.Month(), day.Day(), prefs.WorkStartHour, 0, 0, 0, loc)
			dayEnd := time.Date(day.Year(), day.Month(), day.Day(), prefs.WorkEndHour, 0, 0, 0, loc)

			slotStart := maxTime(current, dayStart)

			for {
				slotEnd := slotStart.Add(duration)

				// Must fit within the working day and the requested range.
				if slotEnd.After(dayEnd) || slotEnd.After(rangeEnd) {
					break
				}

				if overlap := findOverlap(slotStart, slotEnd, existing); overlap != nil {
					// Jump past the conflicting schedule and try again.
					slotStart = overlap.EndTime.In(loc)
					continue
				}

				slots = append(slots, models.ScheduleSlot{
					StartTime: slotStart.UTC(),
					EndTime:   slotEnd.UTC(),
				})
				slotStart = slotEnd
			}
		}

		// Advance to midnight of the next calendar day (DST-safe).
		current = day.AddDate(0, 0, 1)
	}

	return slots, nil
}

// ---------------------------------------------------------------------------
// Private methods
// ---------------------------------------------------------------------------

// getUserPreferences loads preferences from user_preferences.
// When no row exists the factory defaults are returned so callers always
// receive a fully-populated struct.
func (s *ScheduleService) getUserPreferences(userID int64) (*models.UserPreferences, error) {
	return s.getUserPreferencesTx(s.db, userID)
}

// getUserPreferencesTx is the transaction-aware variant used inside
// AutoScheduleTask — accepts either *sql.DB or *sql.Tx via the Querier interface.
func (s *ScheduleService) getUserPreferencesTx(q dbQuerier, userID int64) (*models.UserPreferences, error) {
	prefs := &models.UserPreferences{
		UserID:         userID,
		WorkStartHour:  defaultWorkStartHour,
		WorkEndHour:    defaultWorkEndHour,
		WorkDays:       defaultWorkDays,
		MinSlotMinutes: defaultMinSlotMinutes,
		Timezone:       defaultTimezone,
	}

	err := q.QueryRow(
		`SELECT work_start_hour, work_end_hour, work_days, min_slot_minutes, timezone
		 FROM user_preferences
		 WHERE user_id = ?`,
		userID,
	).Scan(
		&prefs.WorkStartHour,
		&prefs.WorkEndHour,
		&prefs.WorkDays,
		&prefs.MinSlotMinutes,
		&prefs.Timezone,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No preferences row — defaults are already populated.
			return prefs, nil
		}
		return nil, fmt.Errorf("getUserPreferences: scan: %w", err)
	}
	return prefs, nil
}

// dbQuerier abstracts *sql.DB and *sql.Tx so that query helpers can be
// reused inside and outside transactions.
type dbQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

// getUserScheduleRangeTx returns schedules for the user whose time window
// overlaps [from, to) using the given querier (db or tx).
func (s *ScheduleService) getUserScheduleRangeTx(q dbQuerier, userID int64, from, to time.Time) ([]models.Schedule, error) {
	rows, err := q.Query(
		`SELECT id, task_id, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE user_id = ?
		   AND start_time < ?
		   AND end_time   > ?
		 ORDER BY start_time`,
		userID, to, from,
	)
	if err != nil {
		return nil, fmt.Errorf("getUserScheduleRangeTx: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		return nil, fmt.Errorf("getUserScheduleRangeTx: %w", err)
	}
	return schedules, nil
}

// ---------------------------------------------------------------------------
// Scheduling algorithm
// ---------------------------------------------------------------------------

// findFreeSlot iterates through working days between max(now, task.StartDate)
// and task.EndDate and returns the first slot of the required duration that
// does not conflict with any existing schedule.
//
// Algorithm per working day:
//  1. Compute dayStart and dayEnd from user preferences.
//  2. slotStart = max(current position in time, dayStart).
//  3. While slotStart + duration ≤ dayEnd:
//     a. If slotEnd > deadline → break (no later slot in any day will fit).
//     b. Check for overlap with existing schedules.
//     c. No overlap → return slot.
//     d. Overlap → advance slotStart to end of the conflicting schedule.
//  4. Advance to next day.
func findFreeSlot(
	task *models.Task,
	prefs *models.UserPreferences,
	existing []models.Schedule,
	duration time.Duration,
) (*models.ScheduleSlot, error) {
	loc := loadLocation(prefs.Timezone)
	workDays := parseWorkDays(prefs.WorkDays)

	deadline := task.EndDate
	// Earliest possible start is the later of right now and the task's window open.
	current := maxTime(time.Now(), task.StartDate).In(loc)
	end := deadline.In(loc)

	for !current.After(end) {
		day := dayMidnight(current, loc)

		if !workDays[day.Weekday()] {
			current = day.AddDate(0, 0, 1)
			continue
		}

		dayStart := time.Date(day.Year(), day.Month(), day.Day(), prefs.WorkStartHour, 0, 0, 0, loc)
		dayEnd := time.Date(day.Year(), day.Month(), day.Day(), prefs.WorkEndHour, 0, 0, 0, loc)

		slotStart := maxTime(current, dayStart)

		for {
			slotEnd := slotStart.Add(duration)

			// Slot must fit inside the working day window.
			if slotEnd.After(dayEnd) {
				break
			}

			// Slot must not reach past the task deadline.
			if slotEnd.After(deadline) {
				return nil, fmt.Errorf("findFreeSlot: no available slot before deadline %s", deadline.Format(time.RFC3339))
			}

			if overlap := findOverlap(slotStart, slotEnd, existing); overlap != nil {
				// Push start to just after the conflicting schedule ends.
				slotStart = overlap.EndTime.In(loc)
				continue
			}

			// Found a clear window — return it.
			return &models.ScheduleSlot{
				StartTime: slotStart.UTC(),
				EndTime:   slotEnd.UTC(),
			}, nil
		}

		// No slot found today; try the next calendar day.
		current = day.AddDate(0, 0, 1)
	}

	return nil, fmt.Errorf("findFreeSlot: no available slot before deadline %s", deadline.Format(time.RFC3339))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scanSchedules scans all rows from a schedule query into a slice.
func scanSchedules(rows *sql.Rows) ([]models.Schedule, error) {
	var schedules []models.Schedule
	for rows.Next() {
		var sc models.Schedule
		if err := rows.Scan(
			&sc.ID, &sc.TaskID, &sc.UserID,
			&sc.StartTime, &sc.EndTime, &sc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanSchedules: scan row: %w", err)
		}
		schedules = append(schedules, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanSchedules: iterate rows: %w", err)
	}
	return schedules, nil
}

// calculateDuration returns the scheduling duration for a task.
//
//   - If task.DurationMinutes > 0, that value is used as-is.
//   - Otherwise the duration is derived from EndDate − StartDate, clamped to
//     [minDurationMinutes, maxDurationMinutes].
func calculateDuration(task *models.Task) time.Duration {
	if task.DurationMinutes > 0 {
		return time.Duration(task.DurationMinutes) * time.Minute
	}

	minutes := int(task.EndDate.Sub(task.StartDate).Minutes())
	if minutes < minDurationMinutes {
		minutes = minDurationMinutes
	}
	if minutes > maxDurationMinutes {
		minutes = maxDurationMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// findOverlap returns the first schedule in the slice whose window overlaps
// [slotStart, slotEnd).
//
// Two half-open intervals [a, b) and [c, d) overlap when a < d AND c < b.
func findOverlap(slotStart, slotEnd time.Time, schedules []models.Schedule) *models.Schedule {
	for i := range schedules {
		sc := &schedules[i]
		if slotStart.Before(sc.EndTime) && sc.StartTime.Before(slotEnd) {
			return sc
		}
	}
	return nil
}

// parseWorkDays converts a comma-separated abbreviation string such as
// "Mon,Tue,Wed,Thu,Fri" into a set of time.Weekday values.
func parseWorkDays(workDays string) map[time.Weekday]bool {
	abbrevToWeekday := map[string]time.Weekday{
		"Sun": time.Sunday,
		"Mon": time.Monday,
		"Tue": time.Tuesday,
		"Wed": time.Wednesday,
		"Thu": time.Thursday,
		"Fri": time.Friday,
		"Sat": time.Saturday,
	}
	days := make(map[time.Weekday]bool, 7)
	for _, part := range strings.Split(workDays, ",") {
		if wd, ok := abbrevToWeekday[strings.TrimSpace(part)]; ok {
			days[wd] = true
		}
	}
	return days
}

// loadLocation parses an IANA timezone name, falling back to UTC on error.
func loadLocation(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// maxTime returns the later of two time values.
func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// dayMidnight returns the midnight (00:00:00) of the day containing t in loc.
// Using time.Date with explicit fields is DST-safe and avoids 24 h ≠ 1 day
// edge cases around clock changes.
func dayMidnight(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
