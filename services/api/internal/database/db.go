package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Connect() error {
	host := getEnv("DB_HOST", "postgres")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "tee")
	password := getEnv("DB_PASSWORD", "tee")
	dbname := getEnv("DB_NAME", "tee")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection with retries
	for i := 0; i < 30; i++ {
		err = DB.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	return nil
}

func InitSchema() error {
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
	return err
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
