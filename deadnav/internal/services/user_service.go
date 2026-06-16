package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadnav/internal/config"
	"deadnav/internal/models"
	"deadnav/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	DB     *sql.DB
	Config *config.Config
	log    *zap.Logger
}

func NewUserService(db *sql.DB, cfg *config.Config) *UserService {
	return &UserService{
		DB:     db,
		Config: cfg,
		log:    logger.GetLogger(),
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"username_or_email"`
	Password        string `json:"password"`
}

type TelegramAuthRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	AuthDate   int64  `json:"auth_date"`
	Hash       string `json:"hash"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// Register creates a new user with password authentication.
// Email is optional; when provided it must be unique.
func (s *UserService) Register(req RegisterRequest) (*AuthResponse, error) {
	// Validate input
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("требуются имя пользователя и пароль")
	}

	email := strings.TrimSpace(req.Email)

	if len(req.Password) < 6 {
		return nil, errors.New("пароль должен содержать не менее 6 символов")
	}

	// Check if username already exists (always) and, if email was provided,
	// that it is not already in use.
	var existingCount int
	var err error
	if email != "" {
		err = s.DB.QueryRow(
			"SELECT COUNT(*) FROM users WHERE username = ? OR email = ?",
			req.Username, email,
		).Scan(&existingCount)
	} else {
		err = s.DB.QueryRow(
			"SELECT COUNT(*) FROM users WHERE username = ?",
			req.Username,
		).Scan(&existingCount)
	}
	if err != nil {
		s.log.Error("register: db query failed",
			zap.Error(err),
		)
		return nil, err
	}
	if existingCount > 0 {
		return nil, errors.New("имя пользователя или электронная почта уже существуют")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error("hash password failed",
			zap.Error(err),
		)
		return nil, err
	}

	// Create user. Use sql.NullString so an empty email is stored as NULL,
	// which keeps the optional/unique semantics consistent.
	now := time.Now()
	var emailArg interface{}
	if email == "" {
		emailArg = nil
	} else {
		emailArg = email
	}
	result, err := s.DB.Exec(
		"INSERT INTO users (username, email, password_hash, auth_provider, created_at) VALUES (?, ?, ?, 'local', ?)",
		req.Username, emailArg, string(hashedPassword), now,
	)
	if err != nil {
		s.log.Error("Register: db query failed",
			zap.Error(err),
		)
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		s.log.Error("Register: lastindexid failed",
			zap.Error(err),
		)
		return nil, err
	}

	user := models.User{
		ID:           userID,
		Username:     req.Username,
		Email:        email,
		AuthProvider: "local",
		CreatedAt:    now,
	}

	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		s.log.Error("Register: jwt genereate failed",
			zap.Error(err),
		)
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Login authenticates a user with username/email and password
func (s *UserService) Login(req LoginRequest) (*AuthResponse, error) {
	if req.UsernameOrEmail == "" || req.Password == "" {
		return nil, errors.New("требуются имя пользователя/электронная почта и пароль")
	}

	var user models.User
	var passwordHash sql.NullString

	err := s.DB.QueryRow(
		`SELECT id, username, COALESCE(email, ''), COALESCE(password_hash, ''), auth_provider, created_at
		 FROM users WHERE  username = ? OR email = ?`,
		req.UsernameOrEmail, req.UsernameOrEmail,
	).Scan(&user.ID, &user.Username, &user.Email, &passwordHash, &user.AuthProvider, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid credentials")
		}
		s.log.Error("login: db query failed",
			zap.Error(err),
		)
		return nil, err
	}

	// Check if user uses password authentication
	if user.AuthProvider != "local" {
		return nil, errors.New("this account uses Telegram login, please use Telegram auth")
	}

	// Verify password
	if !passwordHash.Valid {
		return nil, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// LoginWithTelegram authenticates or creates a user via Telegram
func (s *UserService) LoginWithTelegram(req TelegramAuthRequest) (*AuthResponse, error) {
	if req.TelegramID == 0 || req.Username == "" {
		return nil, errors.New("telegram_id and username are required")
	}

	var user models.User
	var telegramID sql.NullInt64

	err := s.DB.QueryRow(
		`SELECT id, username, email, COALESCE(password_hash, ''), auth_provider, telegram_id, created_at
		 FROM users WHERE telegram_id = ?`,
		req.TelegramID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AuthProvider, &telegramID, &user.CreatedAt)

	if err == sql.ErrNoRows {
		// User doesn't exist, create new user
		now := time.Now()
		result, err := s.DB.Exec(
			"INSERT INTO users (telegram_id, username, email, auth_provider, created_at) VALUES (?, ?, ?, 'telegram', ?)",
			req.TelegramID, req.Username, "", now,
		)
		if err != nil {
			// Check if username conflict
			if isDuplicateEntry(err) {
				// Try with telegram_id as username suffix
				uniqueUsername := fmt.Sprintf("%s_%d", req.Username, req.TelegramID%10000)
				result, err = s.DB.Exec(
					"INSERT INTO users (telegram_id, username, email, auth_provider, created_at) VALUES (?, ?, ?, 'telegram', ?)",
					req.TelegramID, uniqueUsername, "", now,
				)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}

		userID, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}

		user = models.User{
			ID:           userID,
			Username:     req.Username,
			Email:        "",
			TelegramID:   &req.TelegramID,
			AuthProvider: "telegram",
			CreatedAt:    now,
		}
	} else if err != nil {
		return nil, err
	} else {
		// User exists, update telegram_id if not set
		if !telegramID.Valid {
			_, _ = s.DB.Exec("UPDATE users SET telegram_id = ? WHERE id = ?", req.TelegramID, user.ID)
			user.TelegramID = &req.TelegramID
		}
	}

	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(userID int64) (*models.User, error) {
	var user models.User
	var telegramID sql.NullInt64

	err := s.DB.QueryRow(
		`SELECT id, username, email, COALESCE(password_hash, ''), auth_provider, telegram_id, created_at, notification
		 FROM users WHERE id = ?`,
		userID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AuthProvider, &telegramID, &user.CreatedAt, &user.Notification)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("GetUserByID: user %d not found: %w", userID, sql.ErrNoRows)
		}
		s.log.Error("GetUserByID: db query failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if telegramID.Valid {
		user.TelegramID = &telegramID.Int64
	}

	return &user, nil
}

// generateToken creates a JWT token for the user
func (s *UserService) generateToken(user models.User) (string, error) {
	expirationHours := s.Config.Auth.JWTExpiration
	if expirationHours == 0 {
		expirationHours = 24
	}

	claims := jwt.MapClaims{
		"user_id":       user.ID,
		"username":      user.Username,
		"auth_provider": user.AuthProvider,
		"exp":           time.Now().Add(time.Duration(expirationHours) * time.Hour).Unix(),
		"iat":           time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.Config.Auth.JWTSecret))
}

// ValidateToken validates a JWT token and returns the user ID
func (s *UserService) ValidateToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.Config.Auth.JWTSecret), nil
	})

	if err != nil {
		s.log.Error("validatetoken: ",
			zap.Error(err),
		)
		return 0, err
	}

	if !token.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("invalid user_id in token")
	}

	return int64(userID), nil
}

// VKAuthRequest is the payload from the VK ID callback handler after a
// successful authorisation. The client receives a `code` from VK, exchanges
// it for an access token, fetches the user profile and finally posts the
// minimal profile info needed to identify the user in our database.
type VKAuthRequest struct {
	VKID      int64  `json:"vk_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar"`
}

// LoginWithVK authenticates or creates a user via VK ID.
//
// Behaviour mirrors LoginWithTelegram:
//   - if a user with the given VK ID exists, their vk_id field is refreshed
//     (if necessary) and the existing record is returned;
//   - otherwise a new user is inserted with auth_provider = "vk" and a
//     generated username (with collision fallback).
func (s *UserService) LoginWithVK(req VKAuthRequest) (*AuthResponse, error) {
	if req.VKID == 0 {
		return nil, errors.New("vk_id is required")
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		// Fall back to a stable, unique handle based on the VK ID.
		username = fmt.Sprintf("vk_%d", req.VKID)
	}

	var user models.User
	var vkID sql.NullInt64

	err := s.DB.QueryRow(
		`SELECT id, username, email, COALESCE(password_hash, ''), auth_provider, vk_id, created_at
		 FROM users WHERE vk_id = ?`,
		req.VKID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AuthProvider, &vkID, &user.CreatedAt)

	if err == sql.ErrNoRows {
		// New VK user — create an account.
		now := time.Now()
		email := strings.TrimSpace(req.Email)

		insert := func(name string) (int64, error) {
			res, err := s.DB.Exec(
				"INSERT INTO users (vk_id, username, email, auth_provider, created_at) VALUES (?, ?, ?, 'vk', ?)",
				req.VKID, name, email, now,
			)
			if err != nil {
				return 0, err
			}
			return res.LastInsertId()
		}

		userID, err := insert(username)
		if err != nil {
			if isDuplicateEntry(err) {
				// Username conflict — append a suffix derived from VK ID.
				suffix := req.VKID % 100000
				uniqueUsername := fmt.Sprintf("%s_%d", username, suffix)
				userID, err = insert(uniqueUsername)
				if err != nil {
					return nil, err
				}
				username = uniqueUsername
			} else {
				return nil, err
			}
		}

		user = models.User{
			ID:           userID,
			Username:     username,
			Email:        email,
			VKID:         &req.VKID,
			AuthProvider: "vk",
			CreatedAt:    now,
		}
	} else if err != nil {
		return nil, err
	} else {
		// Existing user — keep vk_id in sync and reload the model.
		if !vkID.Valid {
			_, _ = s.DB.Exec("UPDATE users SET vk_id = ? WHERE id = ?", req.VKID, user.ID)
		}
		user.VKID = &req.VKID
	}

	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

func (s *UserService) UpdateNotification(userid int64, enabled bool) error {

	res, err := s.DB.Exec("UPDATE users SET notification = ? WHERE id = ?", enabled, userid)
	if err != nil {
		return fmt.Errorf("updatenotification: query: %w", err)
	}
	id, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("updatenotification: rowaffected: %w", err)
	}
	if id == 0 {
		return fmt.Errorf("updatenotification: user not found")
	}
	return nil
}

// isDuplicateEntry reports whether err is a MySQL duplicate-entry error (1062).
func isDuplicateEntry(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1062") && strings.Contains(msg, "Duplicate entry")
}
