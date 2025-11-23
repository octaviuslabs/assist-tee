package reaper

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/database"
)

// StartReaper starts the background process that cleans up expired environments
func StartReaper() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			reapExpiredEnvironments()
		}
	}()
}

func reapExpiredEnvironments() {
	log.Println("Running environment reaper...")

	rows, err := database.DB.Query(`
		SELECT id, volume_name, created_at, ttl_seconds
		FROM environments
		WHERE created_at + (ttl_seconds || ' seconds')::interval < NOW()
	`)
	if err != nil {
		log.Printf("Reaper error: %v\n", err)
		return
	}
	defer rows.Close()

	var reaped int
	for rows.Next() {
		var id uuid.UUID
		var volumeName string
		var createdAt time.Time
		var ttl int

		rows.Scan(&id, &volumeName, &createdAt, &ttl)

		log.Printf("Reaping environment %s (age: %v, ttl: %ds)\n",
			id, time.Since(createdAt), ttl)

		// Remove volume
		exec.Command("docker", "volume", "rm", "-f", volumeName).Run()

		// Delete from DB
		database.DB.Exec("DELETE FROM environments WHERE id = $1", id)
		reaped++
	}

	if reaped > 0 {
		log.Printf("Reaped %d environments\n", reaped)
	}
}

// ReconcileEnvironments reconciles the database with actual Docker volumes
func ReconcileEnvironments() error {
	log.Println("Reconciling environments...")

	// Get all volumes from Docker
	cmd := exec.Command("docker", "volume", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	dockerVolumes := make(map[string]bool)
	for _, line := range bytes.Split(output, []byte("\n")) {
		if len(line) > 0 {
			dockerVolumes[string(line)] = true
		}
	}

	// Get all environments from DB
	rows, err := database.DB.Query("SELECT id, volume_name FROM environments")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var volumeName string
		rows.Scan(&id, &volumeName)

		// If volume doesn't exist in Docker, delete from DB
		if !dockerVolumes[volumeName] {
			log.Printf("Volume %s missing for environment %s, deleting from DB\n", volumeName, id)
			database.DB.Exec("DELETE FROM environments WHERE id = $1", id)
		}
	}

	// Clean up orphaned TEE volumes (exist in Docker but not in DB)
	dbVolumes := make(map[string]bool)
	rows2, err := database.DB.Query("SELECT volume_name FROM environments")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var volumeName string
			rows2.Scan(&volumeName)
			dbVolumes[volumeName] = true
		}
	}

	for volumeName := range dockerVolumes {
		if strings.HasPrefix(volumeName, "tee-env-") && !dbVolumes[volumeName] {
			log.Printf("Removing orphaned volume: %s\n", volumeName)
			exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
		}
	}

	log.Println("Reconciliation complete")
	return nil
}
