package logger

import (
	"go.uber.org/zap"
)

var zapLog *zap.Logger

// SetConfig sets the logger configuration
func SetConfig(config zap.Config) error {
	var err error
	enccoderConfig := zap.NewProductionEncoderConfig()
	enccoderConfig.StacktraceKey = "" // to hide stacktrace info
	config.EncoderConfig = enccoderConfig
	zapLog, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return err
	}
	return nil
}

func Info(message string, fields ...zap.Field) {
	zapLog.Info(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	zapLog.Debug(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	zapLog.Warn(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	zapLog.Error(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	zapLog.Fatal(message, fields...)
}
