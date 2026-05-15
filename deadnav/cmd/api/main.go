package main

import (
	"fmt"
	"log"
	"deadnav/internal/config"
	"deadnav/internal/database"
	"deadnav/internal/handlers"
	"deadnav/internal/services"
	"deadnav/pkg/logger"
	"deadnav/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize logger
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.NewMySQLConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	logger.GetLogger().Info("Database connection established")

	// Initialize services
	taskService := services.NewTaskService(db)
	statsService := services.NewStatisticsService(db)
	userService := services.NewUserService(db, cfg)

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskService)
	statsHandler := handlers.NewStatisticsHandler(statsService)
	authHandler := handlers.NewAuthHandler(userService)

	// Setup Gin router
	r := gin.Default()

	// Apply middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth routes (public)
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/telegram", authHandler.LoginWithTelegram)
		authGroup.GET("/me", middleware.JWTAuth(userService), authHandler.GetMe)
	}

	// Task routes (protected)
	taskGroup := r.Group("/api/v1/tasks")
	taskGroup.Use(middleware.JWTAuth(userService))
	{
		taskGroup.POST("", taskHandler.CreateTask)
		taskGroup.GET("", taskHandler.GetAllTasks)
		taskGroup.GET("/:id", taskHandler.GetTask)
		taskGroup.PUT("/:id", taskHandler.UpdateTask)
		taskGroup.DELETE("/:id", taskHandler.DeleteTask)
	}

	// Statistics routes (protected)
	statsGroup := r.Group("/api/v1/statistics")
	statsGroup.Use(middleware.JWTAuth(userService))
	{
		statsGroup.GET("", statsHandler.GetStatistics)
	}

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	logger.GetLogger().Info(fmt.Sprintf("Starting server on %s", addr))

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
