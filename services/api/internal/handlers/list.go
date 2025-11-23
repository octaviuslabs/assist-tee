package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/models"
)

func HandleList(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
		SELECT id, volume_name, main_module, created_at, last_executed_at,
		       execution_count, status, metadata, ttl_seconds
		FROM environments
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	envs := []models.Environment{}
	for rows.Next() {
		var env models.Environment
		var metadataJSON []byte
		err := rows.Scan(
			&env.ID, &env.VolumeName, &env.MainModule, &env.CreatedAt,
			&env.LastExecutedAt, &env.ExecutionCount, &env.Status,
			&metadataJSON, &env.TTLSeconds,
		)
		if err != nil {
			continue
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &env.Metadata)
		}
		envs = append(envs, env)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(envs)
}
