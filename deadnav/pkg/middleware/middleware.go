package middleware

import (
	"strings"
	"time"

	"deadnav/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CORS returns a CORS middleware that validates allowed origins.
// If allowedOrigins is empty, no origin is allowed (secure default for API).
// If allowedOrigins is "*", all origins are allowed (use with caution).
func CORS(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Check if origin is allowed
		allowed := false
		if allowedOrigins == "*" {
			allowed = true
		} else if allowedOrigins != "" && origin != "" {
			for _, allowedOrigin := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(allowedOrigin) == origin {
					allowed = true
					break
				}
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		log := logger.GetLogger()
		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		}

		if statusCode >= 400 {
			if errMsg := c.Errors.String(); errMsg != "" {
				fields = append(fields, zap.String("error", errMsg))
			}
			log.Error("request", fields...)
		} else {
			log.Info("request", fields...)
		}
	}
}

func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
