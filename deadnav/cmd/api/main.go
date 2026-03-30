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

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskService)
	statsHandler := handlers.NewStatisticsHandler(statsService)

	// Setup Gin router
	r := gin.Default()

	// Apply middleware
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Task routes
	taskGroup := r.Group("/api/v1/tasks")
	{
		taskGroup.POST("", taskHandler.CreateTask)
		taskGroup.GET("", taskHandler.GetAllTasks)
		taskGroup.GET("/:id", taskHandler.GetTask)
		taskGroup.PUT("/:id", taskHandler.UpdateTask)
		taskGroup.DELETE("/:id", taskHandler.DeleteTask)
	}

	// Statistics routes
	statsGroup := r.Group("/api/v1/statistics")
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
