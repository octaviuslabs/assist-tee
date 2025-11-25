package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/jsfour/assist-tee/internal/logger"
)

var DB *sql.DB

func Connect() error {
	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "tee")
	password := getEnv("DB_PASSWORD", "tee")
	dbname := getEnv("DB_NAME", "tee")

	logger.Log.Info("connecting to database",
		slog.String("host", host),
		slog.String("port", port),
		slog.String("user", user),
		slog.String("database", dbname),
	)

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		logger.Log.Error("failed to open database connection",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Test connection with retries
	logger.Log.Debug("testing database connection with retries",
		slog.Int("max_retries", 30),
	)

	var lastErr error
	for i := 0; i < 30; i++ {
		err = DB.Ping()
		if err == nil {
			logger.Log.Info("database connection established",
				slog.Int("attempts", i+1),
			)
			break
		}
		lastErr = err
		logger.Log.Debug("database ping failed, retrying",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()),
		)
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil && err != nil {
		logger.Log.Error("failed to connect to database after retries",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Log connection pool stats
	stats := DB.Stats()
	logger.Log.Debug("database connection pool configured",
		slog.Int("max_open_connections", stats.MaxOpenConnections),
		slog.Int("open_connections", stats.OpenConnections),
		slog.Int("in_use", stats.InUse),
		slog.Int("idle", stats.Idle),
	)

	return nil
}

func InitSchema() error {
	logger.Log.Info("initializing database schema")

	schema := `
	CREATE TABLE IF NOT EXISTS environments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		volume_name VARCHAR(255) NOT NULL UNIQUE,
		main_module VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_executed_at TIMESTAMP,
		execution_count INTEGER NOT NULL DEFAULT 0,
		status VARCHAR(50) NOT NULL DEFAULT 'ready',
		metadata JSONB,
		ttl_seconds INTEGER DEFAULT 3600
	);

	CREATE INDEX IF NOT EXISTS idx_environments_created_at ON environments(created_at);
	CREATE INDEX IF NOT EXISTS idx_environments_last_executed_at ON environments(last_executed_at);
	CREATE INDEX IF NOT EXISTS idx_environments_status ON environments(status);

	CREATE TABLE IF NOT EXISTS executions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
		started_at TIMESTAMP NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMP,
		exit_code INTEGER,
		stdout TEXT,
		stderr TEXT,
		duration_ms INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_executions_environment_id ON executions(environment_id);
	CREATE INDEX IF NOT EXISTS idx_executions_started_at ON executions(started_at);
	`

	_, err := DB.Exec(schema)
	if err != nil {
		logger.Log.Error("failed to initialize database schema",
			slog.String("error", err.Error()),
		)
		return err
	}

	logger.Log.Info("database schema initialized")
	return nil
}

// LogStats logs current database connection pool statistics
func LogStats() {
	if DB == nil {
		return
	}
	stats := DB.Stats()
	logger.Log.Info("database connection pool stats",
		slog.Int("max_open_connections", stats.MaxOpenConnections),
		slog.Int("open_connections", stats.OpenConnections),
		slog.Int("in_use", stats.InUse),
		slog.Int("idle", stats.Idle),
		slog.Int64("wait_count", stats.WaitCount),
		slog.Duration("wait_duration", stats.WaitDuration),
		slog.Int64("max_idle_closed", stats.MaxIdleClosed),
		slog.Int64("max_lifetime_closed", stats.MaxLifetimeClosed),
	)
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
