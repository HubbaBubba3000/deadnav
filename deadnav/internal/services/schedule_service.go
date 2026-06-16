package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadnav/internal/models"
	"deadnav/pkg/logger"

	"go.uber.org/zap"
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
	db  *sql.DB
	log *zap.Logger
}

// NewScheduleService creates a ScheduleService backed by the provided database connection.
func NewScheduleService(db *sql.DB) *ScheduleService {
	return &ScheduleService{
		db:  db,
		log: logger.GetLogger(),
	}
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

	if err := validateSchedulingWindow(task); err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	if err := s.autoScheduleTaskTx(tx, task, userID); err != nil {
		return nil, fmt.Errorf("AutoScheduleTask: %w", err)
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

// RescheduleTaskWithCascade tries to place the requested task into schedule by
// moving conflicting tasks one-by-one to their next available slots.
func (s *ScheduleService) RescheduleTaskWithCascade(task *models.Task, userID int64) (*models.Schedule, error) {
	ctx := context.Background()

	if err := validateSchedulingWindow(task); err != nil {
		return nil, fmt.Errorf("RescheduleTaskWithCascade: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("RescheduleTaskWithCascade: begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.rescheduleTaskWithCascadeTx(tx, task, userID); err != nil {
		return nil, fmt.Errorf("RescheduleTaskWithCascade: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("RescheduleTaskWithCascade: commit: %w", err)
	}

	sc, err := s.GetTaskSchedule(task.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("RescheduleTaskWithCascade: fetch saved schedule: %w", err)
	}
	return sc, nil
}

func (s *ScheduleService) autoScheduleTaskTx(tx *sql.Tx, task *models.Task, userID int64) error {
	prefs, err := s.getUserPreferencesTx(tx, userID)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}

	duration := calculateDuration(task)
	now := time.Now()
	existing, err := s.getUserScheduleRangeTx(tx, userID, now, task.EndDate)
	if err != nil {
		return fmt.Errorf("fetch existing schedules: %w", err)
	}
	existing = excludeTaskSchedule(existing, task.ID)

	slot, blockedTaskIDs, err := findFreeSlot(task, prefs, existing, duration)
	if err != nil {
		return withBlockedTaskIDs(err, blockedTaskIDs)
	}

	if err := s.upsertScheduleTx(tx, task, userID, slot); err != nil {
		return err
	}
	return nil
}

// maxCascadeDepth limits how many levels of cascaded rescheduling are
// attempted. This prevents runaway work when the blocked-task graph is deep
// or cyclic.
const maxCascadeDepth = 10

func (s *ScheduleService) rescheduleTaskWithCascadeTx(tx *sql.Tx, task *models.Task, userID int64) error {
	// visitedIDs tracks every task ID that has already been processed during
	// this cascade pass. It serves two purposes:
	//   1. Cycle guard — if task A was displaced by moving task B, and moving
	//      B would displace A again, we skip A the second time instead of
	//      looping forever.
	//   2. Duplicate work guard — blockedTaskIDs from findFreeSlot may contain
	//      the same ID more than once across iterations.
	visitedIDs := map[int64]struct{}{task.ID: {}}
	return s.rescheduleWithCascadeInternal(tx, task, userID, visitedIDs, 0)
}

func (s *ScheduleService) rescheduleWithCascadeInternal(
	tx *sql.Tx,
	task *models.Task,
	userID int64,
	visitedIDs map[int64]struct{},
	depth int,
) error {
	if depth > maxCascadeDepth {
		return fmt.Errorf("превышена максимальная глубина каскадного перепланирования (%d уровней)", maxCascadeDepth)
	}

	prefs, err := s.getUserPreferencesTx(tx, userID)
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}

	duration := calculateDuration(task)
	now := time.Now()
	existing, err := s.getUserScheduleRangeTx(tx, userID, now, task.EndDate)
	if err != nil {
		return fmt.Errorf("fetch existing schedules: %w", err)
	}
	existing = excludeTaskSchedule(existing, task.ID)

	slot, blockedTaskIDs, err := findFreeSlot(task, prefs, existing, duration)
	if err == nil {
		return s.upsertScheduleTx(tx, task, userID, slot)
	}
	if len(blockedTaskIDs) == 0 {
		// No blocking tasks identified — nothing to cascade, surface the error.
		return withBlockedTaskIDs(err, blockedTaskIDs)
	}

	blockedTasks, err := s.getTasksByIDsTx(tx, userID, blockedTaskIDs)
	if err != nil {
		return fmt.Errorf("load blocked tasks: %w", err)
	}

	// Track IDs that we could not move so we can report them to the caller.
	var unresolvedIDs []int64

	for _, blockedTask := range blockedTasks {
		// Skip terminal states — completed/cancelled tasks cannot be moved.
		if blockedTask.Status == "completed" || blockedTask.Status == "cancelled" {
			continue
		}

		// Cycle / duplicate guard: skip tasks we have already processed in
		// this cascade chain (including the root task itself).
		if _, seen := visitedIDs[blockedTask.ID]; seen {
			unresolvedIDs = append(unresolvedIDs, blockedTask.ID)
			continue
		}
		visitedIDs[blockedTask.ID] = struct{}{}

		// Attempt to recursively reschedule the blocker. On failure we record
		// it as unresolved and continue with the remaining blockers instead of
		// aborting immediately — moving other blockers may still free enough
		// space for the root task.
		if err := s.rescheduleWithCascadeInternal(tx, blockedTask, userID, visitedIDs, depth+1); err != nil {
			unresolvedIDs = append(unresolvedIDs, blockedTask.ID)
		}
	}

	// Re-read the schedule after all cascade moves and try to place the root
	// task. Even a partial cascade (some blockers moved, some not) may have
	// freed a valid slot.
	existing, err = s.getUserScheduleRangeTx(tx, userID, now, task.EndDate)
	if err != nil {
		return fmt.Errorf("reload schedules after cascade: %w", err)
	}
	existing = excludeTaskSchedule(existing, task.ID)

	slot, remainingBlockedIDs, err := findFreeSlot(task, prefs, existing, duration)
	if err != nil {
		// Merge unresolved IDs from both sources so the caller gets a
		// complete picture of what is still blocking the root task.
		allBlocked := mergeIDs(unresolvedIDs, remainingBlockedIDs)
		return withBlockedTaskIDs(err, allBlocked)
	}

	return s.upsertScheduleTx(tx, task, userID, slot)
}

// mergeIDs returns a deduplicated union of two int64 slices.
func mergeIDs(a, b []int64) []int64 {
	seen := make(map[int64]struct{}, len(a)+len(b))
	result := make([]int64, 0, len(a)+len(b))
	for _, id := range append(a, b...) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

func (s *ScheduleService) upsertScheduleTx(tx *sql.Tx, task *models.Task, userID int64, slot *models.ScheduleSlot) error {
	_, err := tx.Exec(
		`INSERT INTO schedules (task_id, title, status, user_id, start_time, end_time)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     start_time = VALUES(start_time),
		     end_time   = VALUES(end_time)`,
		task.ID, task.Title, task.Status, userID, slot.StartTime.UTC(), slot.EndTime.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert schedule: %w", err)
	}
	return nil
}

func (s *ScheduleService) getTasksByIDsTx(q dbQuerier, userID int64, taskIDs []int64) ([]*models.Task, error) {
	var tasks []*models.Task
	for _, id := range taskIDs {
		var t models.Task
		err := q.QueryRow(
			`SELECT id, user_id, title, description, status, priority, duration_minutes,
			        start_date, end_date, complexity, urgency, importance, estimated_minutes,
			        created_at, updated_at
			 FROM tasks
			 WHERE id = ? AND user_id = ?`,
			id, userID,
		).Scan(
			&t.ID, &t.UserID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.DurationMinutes,
			&t.StartDate, &t.EndDate, &t.Complexity, &t.Urgency, &t.Importance, &t.EstimatedMinutes,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, fmt.Errorf("getTasksByIDsTx: task %d: %w", id, err)
		}
		tasks = append(tasks, &t)
	}
	return tasks, nil
}

// RemoveSchedule deletes the schedule entry for the given task and user.
func (s *ScheduleService) RemoveSchedule(taskID int64, userID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM schedules WHERE task_id = ? AND user_id = ?`,
		taskID, userID,
	)
	if err != nil {
		s.log.Error("RemoveSchedule",
			zap.Error(err),
		)
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
	query := `SELECT id, task_id, title, status, user_id, start_time, end_time, created_at
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
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Order != "" {
		query += " ORDER BY " + filter.Order
	} else {
		query += " ORDER BY start_time"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		s.log.Error("GetSchedule: query",
			zap.Error(err),
		)
		return nil, fmt.Errorf("GetSchedules: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		s.log.Error("GetSchedule",
			zap.Error(err),
		)
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
		`SELECT id, task_id, title, status, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE user_id = ?
		   AND start_time < ?
		   AND end_time   > ?
		 ORDER BY start_time`,
		userID, to, from,
	)
	if err != nil {
		s.log.Error("GetUserScheduleRange: query:",
			zap.Error(err),
		)
		return nil, fmt.Errorf("GetUserScheduleRange: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		s.log.Error("GetUserScheduleRange:",
			zap.Error(err),
		)
		return nil, fmt.Errorf("GetUserScheduleRange: %w", err)
	}
	return schedules, nil
}

// GetTaskSchedule returns the schedule for a specific task owned by userID.
// Returns a wrapped sql.ErrNoRows when no schedule exists.
func (s *ScheduleService) GetTaskSchedule(taskID int64, userID int64) (*models.Schedule, error) {
	var sc models.Schedule
	err := s.db.QueryRow(
		`SELECT id, task_id, title, status, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE task_id = ? AND user_id = ?`,
		taskID, userID,
	).Scan(&sc.ID, &sc.TaskID, &sc.Title, &sc.Status, &sc.UserID, &sc.StartTime, &sc.EndTime, &sc.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("GetTaskSchedule: no schedule for task %d: %w", taskID, sql.ErrNoRows)
		}
		s.log.Error("GetTaskSchedule: scan:",
			zap.Error(err),
		)
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
		s.log.Error("getUserPreferences: scan",
			zap.Error(err),
		)
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
		`SELECT id, task_id, title, status, user_id, start_time, end_time, created_at
		 FROM schedules
		 WHERE user_id = ?
		   AND start_time < ?
		   AND end_time   > ?
		   AND status NOT IN ('completed', 'cancelled')
		 ORDER BY start_time`,
		userID, to, from,
	)
	if err != nil {
		s.log.Error("getUserScheduleRangeTx: query",
			zap.Error(err),
		)
		return nil, fmt.Errorf("getUserScheduleRangeTx: query: %w", err)
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		s.log.Error("getUserScheduleRangeTx",
			zap.Error(err),
		)
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
// When no slot can be found before the deadline, it returns an error that
// includes the IDs of tasks that are blocking the schedule.
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
) (*models.ScheduleSlot, []int64, error) {
	// blockedTaskIDs accumulates task IDs that prevent scheduling due to conflicts
	var blockedTaskIDs []int64
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
				return nil, blockedTaskIDs, fmt.Errorf("нет доступного слота до дедлайна %s", deadline.Format(time.RFC3339))
			}

			if overlap := findOverlap(slotStart, slotEnd, existing); overlap != nil {
				// Check if this conflicting task is already in our blocked list
				alreadyBlocked := false
				for _, id := range blockedTaskIDs {
					if id == overlap.TaskID {
						alreadyBlocked = true
						break
					}
				}
				// If not already in the list, add it
				if !alreadyBlocked {
					blockedTaskIDs = append(blockedTaskIDs, overlap.TaskID)
				}
				// Push start to just after the conflicting schedule ends.
				slotStart = overlap.EndTime.In(loc)
				continue
			}

			// Found a clear window — return it.
			return &models.ScheduleSlot{
				StartTime: slotStart.UTC(),
				EndTime:   slotEnd.UTC(),
			}, blockedTaskIDs, nil
		}

		// No slot found today; try the next calendar day.
		current = day.AddDate(0, 0, 1)
	}

	return nil, blockedTaskIDs, fmt.Errorf("нет доступного слота до дедлайна %s", deadline.Format(time.RFC3339))
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
			&sc.ID, &sc.TaskID, &sc.Title, &sc.Status, &sc.UserID,
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

func validateSchedulingWindow(task *models.Task) error {
	if task.EndDate.Before(task.StartDate) {
		return fmt.Errorf("дедлайн не может быть раньше даты начала")
	}

	duration := calculateDuration(task)
	if duration <= 0 {
		return fmt.Errorf("длительность задачи должна быть больше 0")
	}

	if task.StartDate.Add(duration).After(task.EndDate) {
		return fmt.Errorf("окно задачи меньше требуемой длительности")
	}

	return nil
}

func excludeTaskSchedule(schedules []models.Schedule, taskID int64) []models.Schedule {
	if taskID == 0 {
		return schedules
	}

	filtered := schedules[:0]
	for _, sc := range schedules {
		if sc.TaskID != taskID {
			filtered = append(filtered, sc)
		}
	}
	return filtered
}

func withBlockedTaskIDs(err error, blockedTaskIDs []int64) error {
	if err == nil || len(blockedTaskIDs) == 0 {
		return err
	}

	var idStrs []string
	for _, id := range blockedTaskIDs {
		idStrs = append(idStrs, fmt.Sprintf("%d", id))
	}

	return fmt.Errorf("%s blocked_tasks=[%s]", err.Error(), strings.Join(idStrs, ", "))
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
