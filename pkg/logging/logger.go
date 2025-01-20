package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var zapLog *zap.Logger

// SetConfig sets the logger configuration
func SetConfig(config zap.Config) error {
	var err error
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.StacktraceKey = "" // to hide stacktrace info
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // format timestamp to ISO8601
	config.EncoderConfig = encoderConfig
	config.Encoding = "json" // set encoding to JSON
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
