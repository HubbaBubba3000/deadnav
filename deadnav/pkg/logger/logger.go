package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init() error {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	
	var err error
	Logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

func GetLogger() *zap.Logger {
	if Logger == nil {
		return zap.NewNop()
	}
	return Logger
}
