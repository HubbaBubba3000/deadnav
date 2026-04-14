package middleware

import (
	"net/http"
	"strings"

	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// JWTAuth returns a middleware that validates the JWT token
func JWTAuth(userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format, use 'Bearer <token>'"})
			c.Abort()
			return
		}

		// Validate token
		userID, err := userService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", userID)
		c.Next()
	}
}

// OptionalAuth returns a middleware that optionally validates the JWT token
// If token is present and valid, sets user_id in context, otherwise continues
func OptionalAuth(userService *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.Next()
			return
		}

		userID, err := userService.ValidateToken(tokenString)
		if err == nil {
			c.Set("user_id", userID)
		}

		c.Next()
	}
}
