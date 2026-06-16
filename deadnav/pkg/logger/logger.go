package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger *zap.Logger
	once   sync.Once
)

// Init builds the global sugared logger. It is safe to call concurrently —
// the actual initialisation happens at most once.
func Init() error {
	var initErr error
	once.Do(func() {
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		Logger, initErr = config.Build()
	})
	return initErr
}

// GetLogger returns the global logger, or a no-op logger if Init hasn't been
// called yet. Safe for concurrent use.
func GetLogger() *zap.Logger {
	if Logger == nil {
		return zap.NewNop()
	}
	return Logger
}
