package middleware

import (
	"net/http"
	"strconv"
	"time"

	"deadnav/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"go.uber.org/zap"
)

// NewRateLimiter builds a gin.HandlerFunc that throttles requests using an
// in-memory token bucket. The rate string follows the ulule/limiter format
// "limit-period" (e.g. "5-M" = 5 requests per minute, "100-H" = 100/hour).
//
// Period suffixes: S (second), M (minute), H (hour), D (day).
//
// If rate is empty the function returns nil so the caller can detect the
// disabled case and skip applying the middleware entirely.
func NewRateLimiter(rate string) (gin.HandlerFunc, error) {
	if rate == "" {
		return nil, nil
	}

	parsed, err := limiter.NewRateFromFormatted(rate)
	if err != nil {
		return nil, err
	}

	store := memory.NewStore()
	instance := limiter.New(store, parsed)

	return ginlimiter.NewMiddleware(instance,
		ginlimiter.WithKeyGetter(func(c *gin.Context) string {
			return c.ClientIP()
		}),
		ginlimiter.WithLimitReachedHandler(func(c *gin.Context) {
			// Retry-After is the number of seconds the client should wait
			// before retrying. Round the period up to the next whole
			// second so sub-second periods still emit a useful hint.
			retryAfter := int64(parsed.Period / time.Second)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))

			logger.GetLogger().Warn("rate limit exceeded",
				zap.String("client_ip", c.ClientIP()),
				zap.String("path", c.Request.URL.Path),
				zap.Int64("limit", parsed.Limit),
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please try again later",
			})
		}),
	), nil
}
