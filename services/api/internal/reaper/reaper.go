package reaper

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/logger"
)

// StartReaper starts the background process that cleans up expired environments
func StartReaper() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		logger.Log.Info("reaper service started",
			slog.Duration("interval", 5*time.Minute),
		)
		for range ticker.C {
			reapExpiredEnvironments()
		}
	}()
}

func reapExpiredEnvironments() {
	ctx := context.Background()
	log := logger.Log

	log.Debug("running environment reaper")

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, volume_name, created_at, ttl_seconds
		FROM environments
		WHERE created_at + (ttl_seconds || ' seconds')::interval < NOW()
	`)
	if err != nil {
		log.Error("reaper query failed",
			slog.String("error", err.Error()),
		)
		return
	}
	defer rows.Close()

	var reaped int
	var errors int
	for rows.Next() {
		var id uuid.UUID
		var volumeName string
		var createdAt time.Time
		var ttl int

		if err := rows.Scan(&id, &volumeName, &createdAt, &ttl); err != nil {
			log.Warn("failed to scan environment row",
				slog.String("error", err.Error()),
			)
			errors++
			continue
		}

		age := time.Since(createdAt)
		log.Info("reaping expired environment",
			slog.String("environment_id", id.String()),
			slog.String("volume_name", volumeName),
			slog.Duration("age", age),
			slog.Int("ttl_seconds", ttl),
		)

		// Remove volume
		if err := exec.Command("docker", "volume", "rm", "-f", volumeName).Run(); err != nil {
			log.Warn("failed to remove docker volume during reap",
				slog.String("volume_name", volumeName),
				slog.String("error", err.Error()),
			)
		}

		// Delete from DB
		if _, err := database.DB.ExecContext(ctx, "DELETE FROM environments WHERE id = $1", id); err != nil {
			log.Error("failed to delete environment during reap",
				slog.String("environment_id", id.String()),
				slog.String("error", err.Error()),
			)
			errors++
			continue
		}

		reaped++
	}

	if reaped > 0 || errors > 0 {
		log.Info("reaper cycle completed",
			slog.Int("reaped", reaped),
			slog.Int("errors", errors),
		)
	} else {
		log.Debug("reaper cycle completed - no expired environments")
	}
}

// ReconcileEnvironments reconciles the database with actual Docker volumes
func ReconcileEnvironments() error {
	ctx := context.Background()
	log := logger.Log

	log.Info("starting environment reconciliation")

	// Get all volumes from Docker
	cmd := exec.Command("docker", "volume", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		log.Error("failed to list docker volumes",
			slog.String("error", err.Error()),
		)
		return err
	}

	dockerVolumes := make(map[string]bool)
	for _, line := range bytes.Split(output, []byte("\n")) {
		if len(line) > 0 {
			dockerVolumes[string(line)] = true
		}
	}

	log.Debug("found docker volumes",
		slog.Int("count", len(dockerVolumes)),
	)

	// Get all environments from DB
	rows, err := database.DB.QueryContext(ctx, "SELECT id, volume_name FROM environments")
	if err != nil {
		log.Error("failed to query environments",
			slog.String("error", err.Error()),
		)
		return err
	}
	defer rows.Close()

	var deletedMissing int
	for rows.Next() {
		var id uuid.UUID
		var volumeName string
		if err := rows.Scan(&id, &volumeName); err != nil {
			log.Warn("failed to scan environment row",
				slog.String("error", err.Error()),
			)
			continue
		}

		// If volume doesn't exist in Docker, delete from DB
		if !dockerVolumes[volumeName] {
			log.Warn("volume missing for environment - deleting from database",
				slog.String("environment_id", id.String()),
				slog.String("volume_name", volumeName),
			)
			if _, err := database.DB.ExecContext(ctx, "DELETE FROM environments WHERE id = $1", id); err != nil {
				log.Error("failed to delete environment with missing volume",
					slog.String("environment_id", id.String()),
					slog.String("error", err.Error()),
				)
			} else {
				deletedMissing++
			}
		}
	}

	// Clean up orphaned TEE volumes (exist in Docker but not in DB)
	dbVolumes := make(map[string]bool)
	rows2, err := database.DB.QueryContext(ctx, "SELECT volume_name FROM environments")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var volumeName string
			rows2.Scan(&volumeName)
			dbVolumes[volumeName] = true
		}
	}

	var removedOrphans int
	for volumeName := range dockerVolumes {
		if strings.HasPrefix(volumeName, "tee-env-") && !dbVolumes[volumeName] {
			log.Warn("removing orphaned volume",
				slog.String("volume_name", volumeName),
			)
			if err := exec.Command("docker", "volume", "rm", "-f", volumeName).Run(); err != nil {
				log.Error("failed to remove orphaned volume",
					slog.String("volume_name", volumeName),
					slog.String("error", err.Error()),
				)
			} else {
				removedOrphans++
			}
		}
	}

	log.Info("reconciliation completed",
		slog.Int("deleted_missing_volumes", deletedMissing),
		slog.Int("removed_orphaned_volumes", removedOrphans),
	)

	return nil
}
