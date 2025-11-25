package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Context key for request ID
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	LogLevelEnv  string     = "LOG_LEVEL"
)

var (
	// Default logger instance
	Log *slog.Logger
)

// Config holds logger configuration
type Config struct {
	Level      slog.Level
	JSONFormat bool
	AddSource  bool
}

// Init initializes the global logger with the given configuration
func Init(cfg *Config) {
	if cfg == nil {
		cfg = &Config{
			Level:      slog.LevelInfo,
			JSONFormat: true,
			AddSource:  false,
		}
	}

	// Check environment variable for log level override
	if levelStr := os.Getenv(LogLevelEnv); levelStr != "" {
		switch levelStr {
		case "debug", "DEBUG":
			cfg.Level = slog.LevelDebug
		case "info", "INFO":
			cfg.Level = slog.LevelInfo
		case "warn", "WARN", "warning", "WARNING":
			cfg.Level = slog.LevelWarn
		case "error", "ERROR":
			cfg.Level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.JSONFormat {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)
}

// WithRequestID returns a logger with request ID attached
func WithRequestID(ctx context.Context, requestID string) *slog.Logger {
	return Log.With(slog.String("request_id", requestID))
}

// FromContext returns a logger from context, or the default logger
func FromContext(ctx context.Context) *slog.Logger {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return WithRequestID(ctx, requestID)
	}
	return Log
}

// WithContext adds request ID to context
func WithContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID retrieves request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// Standard logging helpers with context support

func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}

// LogOperation logs the start of an operation and returns a function to log its completion
func LogOperation(ctx context.Context, operation string, attrs ...any) func(err error) {
	start := time.Now()
	logger := FromContext(ctx)

	allAttrs := append([]any{slog.String("operation", operation)}, attrs...)
	logger.Debug("operation started", allAttrs...)

	return func(err error) {
		duration := time.Since(start)
		allAttrs = append(allAttrs,
			slog.Duration("duration", duration),
			slog.Int64("duration_ms", duration.Milliseconds()),
		)

		if err != nil {
			allAttrs = append(allAttrs, slog.String("error", err.Error()))
			logger.Error("operation failed", allAttrs...)
		} else {
			logger.Info("operation completed", allAttrs...)
		}
	}
}

// LogExecutionResult logs execution results with structured fields
func LogExecutionResult(ctx context.Context, envID, execID string, exitCode int, durationMs int64, err error) {
	logger := FromContext(ctx)
	attrs := []any{
		slog.String("environment_id", envID),
		slog.String("execution_id", execID),
		slog.Int("exit_code", exitCode),
		slog.Int64("duration_ms", durationMs),
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("execution failed", attrs...)
	} else if exitCode != 0 {
		logger.Warn("execution completed with non-zero exit", attrs...)
	} else {
		logger.Info("execution completed", attrs...)
	}
}
