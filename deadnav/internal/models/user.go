package models

import "time"

// User represents an authenticated application user.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	TelegramID   *int64    `json:"telegram_id,omitempty"`
	VKID         *int64    `json:"vk_id,omitempty"`
	AuthProvider string    `json:"auth_provider"` // "local" | "telegram" | "vk"
	Notification bool      `json:notification`
	CreatedAt    time.Time `json:"created_at"`
}

// UserPreferences stores per-user scheduling configuration.
type UserPreferences struct {
	UserID         int64  `json:"user_id"`
	WorkStartHour  int    `json:"work_start_hour"`  // 0–23, default 9
	WorkEndHour    int    `json:"work_end_hour"`    // 0–23, default 18
	WorkDays       string `json:"work_days"`        // comma-separated abbreviations: "Mon,Tue,Wed,Thu,Fri"
	MinSlotMinutes int    `json:"min_slot_minutes"` // minimum schedulable slot in minutes, default 30
	Timezone       string `json:"timezone"`         // IANA timezone name, default "UTC"
}
