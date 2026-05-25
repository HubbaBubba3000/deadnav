package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"deadnav/internal/config"
	"deadnav/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	DB     *sql.DB
	Config *config.Config
}

func NewUserService(db *sql.DB, cfg *config.Config) *UserService {
	return &UserService{
		DB:     db,
		Config: cfg,
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

// Register creates a new user with password authentication
func (s *UserService) Register(req RegisterRequest) (*AuthResponse, error) {
	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email and password are required")
	}

	if len(req.Password) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	// Check if username or email already exists
	var existingCount int
	err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM users WHERE username = ? OR email = ?",
		req.Username, req.Email,
	).Scan(&existingCount)
	if err != nil {
		return nil, err
	}
	if existingCount > 0 {
		return nil, errors.New("username or email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user
	now := time.Now()
	result, err := s.DB.Exec(
		"INSERT INTO users (username, email, password_hash, auth_provider, created_at) VALUES (?, ?, ?, 'local', ?)",
		req.Username, req.Email, string(hashedPassword), now,
	)
	if err != nil {
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	user := models.User{
		ID:           userID,
		Username:     req.Username,
		Email:        req.Email,
		AuthProvider: "local",
		CreatedAt:    now,
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

// Login authenticates a user with username/email and password
func (s *UserService) Login(req LoginRequest) (*AuthResponse, error) {
	if req.UsernameOrEmail == "" || req.Password == "" {
		return nil, errors.New("username/email and password are required")
	}

	var user models.User
	var passwordHash sql.NullString

	err := s.DB.QueryRow(
		`SELECT id, username, email, COALESCE(password_hash, ''), auth_provider, created_at
		 FROM users WHERE username = ? OR email = ?`,
		req.UsernameOrEmail, req.UsernameOrEmail,
	).Scan(&user.ID, &user.Username, &user.Email, &passwordHash, &user.AuthProvider, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid credentials")
		}
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
		`SELECT id, username, email, COALESCE(password_hash, ''), auth_provider, telegram_id, created_at
		 FROM users WHERE id = ?`,
		userID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AuthProvider, &telegramID, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("GetUserByID: user %d not found: %w", userID, sql.ErrNoRows)
		}
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

// isDuplicateEntry reports whether err is a MySQL duplicate-entry error (1062).
func isDuplicateEntry(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1062") && strings.Contains(msg, "Duplicate entry")
}
