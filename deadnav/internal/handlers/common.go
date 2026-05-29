package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"deadnav/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// errorResponse is the canonical error envelope returned by all handlers.
type errorResponse struct {
	Error string `json:"error"`
}

// messageResponse is a simple success message envelope.
type messageResponse struct {
	Message string `json:"message"`
}

// mustUserID extracts the user_id set by the JWT middleware.
// It panics if the value is missing, which would indicate a route that
// accidentally bypassed authentication middleware.
func mustUserID(c *gin.Context) int64 {
	v, exists := c.Get("user_id")
	if !exists {
		panic("mustUserID: user_id not found in context — is auth middleware configured?")
	}
	id, ok := v.(int64)
	if !ok || id == 0 {
		panic("mustUserID: user_id in context has invalid type or zero value")
	}
	return id
}

// parseID reads the :id URL parameter and returns it as int64.
func parseID(c *gin.Context) (int64, error) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: must be a positive integer", raw)
	}
	if id < 1 {
		return 0, fmt.Errorf("invalid id %d: must be positive", id)
	}
	return id, nil
}

// internalError logs the real error and returns a generic 500 response to the
// client, preventing internal details (database errors, stack traces, etc.)
// from leaking outward.
func internalError(c *gin.Context, msg string, err error) {
	c.Error(err)
	logger.GetLogger().Error(msg, zap.Error(err))
	c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
}
