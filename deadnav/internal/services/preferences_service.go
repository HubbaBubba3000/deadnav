package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadnav/internal/models"
)

// PreferencesService manages per-user scheduling preferences.
type PreferencesService struct {
	db *sql.DB
}

// NewPreferencesService creates a PreferencesService backed by the provided
// database connection.
func NewPreferencesService(db *sql.DB) *PreferencesService {
	return &PreferencesService{db: db}
}

// GetPreferences returns the preferences for userID.
// When no row exists, the default values are returned.
func (s *PreferencesService) GetPreferences(userID int64) (*models.UserPreferences, error) {
	prefs := defaultPreferences(userID)

	err := s.db.QueryRow(
		`SELECT work_start_hour, work_end_hour, work_days, min_slot_minutes, timezone
		 FROM user_preferences WHERE user_id = ?`,
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
			return prefs, nil
		}
		return nil, fmt.Errorf("GetPreferences: scan: %w", err)
	}
	return prefs, nil
}

// UpsertPreferences creates or replaces the preferences for prefs.UserID.
func (s *PreferencesService) UpsertPreferences(prefs *models.UserPreferences) error {
	if err := validatePreferences(prefs); err != nil {
		return fmt.Errorf("UpsertPreferences: %w", err)
	}

	_, err := s.db.Exec(
		`INSERT INTO user_preferences
		     (user_id, work_start_hour, work_end_hour, work_days, min_slot_minutes, timezone)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     work_start_hour  = VALUES(work_start_hour),
		     work_end_hour    = VALUES(work_end_hour),
		     work_days        = VALUES(work_days),
		     min_slot_minutes = VALUES(min_slot_minutes),
		     timezone         = VALUES(timezone)`,
		prefs.UserID,
		prefs.WorkStartHour,
		prefs.WorkEndHour,
		prefs.WorkDays,
		prefs.MinSlotMinutes,
		prefs.Timezone,
	)
	if err != nil {
		return fmt.Errorf("UpsertPreferences: exec: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultPreferences(userID int64) *models.UserPreferences {
	return &models.UserPreferences{
		UserID:         userID,
		WorkStartHour:  9,
		WorkEndHour:    18,
		WorkDays:       "Mon,Tue,Wed,Thu,Fri",
		MinSlotMinutes: 30,
		Timezone:       "UTC",
	}
}

// validWorkDayAbbrevs is the accepted set of day abbreviations.
var validWorkDayAbbrevs = map[string]struct{}{
	"Sun": {}, "Mon": {}, "Tue": {}, "Wed": {},
	"Thu": {}, "Fri": {}, "Sat": {},
}

func validatePreferences(p *models.UserPreferences) error {
	if p.WorkStartHour < 0 || p.WorkStartHour > 23 {
		return errors.New("work_start_hour должен находиться в диапазоне от 0 до 23")
	}
	if p.WorkEndHour < 0 || p.WorkEndHour > 23 {
		return errors.New("work_end_hour должен находиться в диапазоне от 0 до 23")
	}
	if p.WorkStartHour >= p.WorkEndHour {
		return errors.New("work_start_hour должен быть меньше work_end_hour")
	}
	if p.MinSlotMinutes < 5 || p.MinSlotMinutes > 480 {
		return errors.New("min_slot_minutes должен быть в диапазоне от 5 до 480")
	}

	parts := strings.Split(p.WorkDays, ",")
	if len(parts) == 0 {
		return errors.New("work_days не должен быть пустым")
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if _, ok := validWorkDayAbbrevs[part]; !ok {
			return fmt.Errorf("неизвестное сокращение дня недели: %q", part)
		}
	}

	if p.Timezone != "" {
		if _, err := time.LoadLocation(p.Timezone); err != nil {
			return fmt.Errorf("недопустимый часовой пояс %q: %w", p.Timezone, err)
		}
	}
	return nil
}
