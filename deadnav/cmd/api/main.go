package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deadnav/internal/config"
	"deadnav/internal/database"
	"deadnav/internal/handlers"
	"deadnav/internal/services"
	"deadnav/pkg/logger"
	"deadnav/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	if err := logger.Init(); err != nil {
		log.Fatalf("failed to initialise logger: %v", err)
	}
	log := logger.GetLogger()
	defer log.Sync()

	// ── Configuration ─────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to load configuration: %v", err))
	}

	// ── Database (retry until MySQL is ready) ─────────────────────────────────
	var db *sql.DB
	for attempt := 1; attempt <= 10; attempt++ {
		db, err = database.NewMySQLConnection(cfg.Database)
		if err == nil {
			break
		}
		log.Warn(fmt.Sprintf("database not ready (attempt %d/30): %v", attempt, err))
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to connect to database after 30 attempts: %v", err))
	}
	defer db.Close()
	log.Info("database connection established")

	// ── Services ──────────────────────────────────────────────────────────────
	userService := services.NewUserService(db, cfg)
	taskService := services.NewTaskService(db)
	scheduleService := services.NewScheduleService(db)
	statsService := services.NewStatisticsService(db)
	preferencesService := services.NewPreferencesService(db)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(userService)
	taskHandler := handlers.NewTaskHandler(taskService, scheduleService)
	scheduleHandler := handlers.NewScheduleHandler(scheduleService, taskService)
	statsHandler := handlers.NewStatisticsHandler(statsService)
	preferencesHandler := handlers.NewPreferencesHandler(preferencesService)

	// ── Router ────────────────────────────────────────────────────────────────
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20) // 8 MB
		c.Next()
	})
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ── Auth routes (public) ──────────────────────────────────────────────────
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/telegram", authHandler.LoginWithTelegram)
		auth.GET("/me", middleware.JWTAuth(userService), authHandler.GetMe)
	}

	// ── Protected routes ──────────────────────────────────────────────────────
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWTAuth(userService))
	{
		// Tasks
		tasks := protected.Group("/tasks")
		{
			tasks.POST("", taskHandler.CreateTask)
			tasks.GET("", taskHandler.GetAllTasks)
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.PUT("/:id", taskHandler.UpdateTask)
			tasks.DELETE("/:id", taskHandler.DeleteTask)
		}

		// Calendar / schedule
		schedule := protected.Group("/schedule")
		{
			schedule.GET("", scheduleHandler.GetSchedule)
			schedule.GET("/free-slots", scheduleHandler.GetFreeSlots)
			schedule.GET("/task/:id", scheduleHandler.GetTaskSchedule)
			schedule.POST("/task/:id/reschedule", scheduleHandler.RescheduleTask)
			schedule.DELETE("/task/:id", scheduleHandler.UnscheduleTask)
		}

		// Statistics
		protected.GET("/statistics", statsHandler.GetStatistics)

		// User preferences
		prefs := protected.Group("/preferences")
		{
			prefs.GET("", preferencesHandler.GetPreferences)
			prefs.PUT("", preferencesHandler.UpdatePreferences)
		}
	}

	// ── Graceful server ──────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in a goroutine.
	go func() {
		log.Info(fmt.Sprintf("starting server on %s", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(fmt.Sprintf("server error: %v", err))
		}
	}()

	// Wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Give outstanding requests 10 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(fmt.Sprintf("server forced to shutdown: %v", err))
	}

	log.Info("server exited gracefully")
}
