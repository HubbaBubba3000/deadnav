package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Telegram TelegramConfig
}

type ServerConfig struct {
	Port           string
	AllowedOrigins string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type AuthConfig struct {
	JWTSecret     string
	JWTExpiration int // in hours
}

type TelegramConfig struct {
	BotToken string
}

func Load() (*Config, error) {
	// Try to load .env file, but don't fail if it doesn't exist
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning loading .env: %v\n", err)
		}
	}

	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" || jwtSecret == "your-secret-key-change-in-production" {
		if os.Getenv("GIN_MODE") == "release" {
			log.Fatal("FATAL: JWT_SECRET must be set in production environment")
		}
		log.Println("WARNING: Using default JWT_SECRET. Set JWT_SECRET environment variable for production.")
		jwtSecret = "your-secret-key-change-in-production"
	}

	return &Config{
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			AllowedOrigins: getEnv("ALLOWED_ORIGINS", ""),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "task_scheduler"),
		},
		Auth: AuthConfig{
			JWTSecret:     jwtSecret,
			JWTExpiration: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		},
		Telegram: TelegramConfig{
			BotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
