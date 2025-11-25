package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/models"
)

func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	log.Debug("list environments request received")

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, volume_name, main_module, created_at, last_executed_at,
		       execution_count, status, metadata, ttl_seconds
		FROM environments
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Error("failed to query environments",
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusInternalServerError, "query_failed", err.Error())
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
			log.Warn("failed to scan environment row",
				slog.String("error", err.Error()),
			)
			continue
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &env.Metadata)
		}
		envs = append(envs, env)
	}

	log.Info("environments listed",
		slog.Int("count", len(envs)),
	)

	writeJSON(w, http.StatusOK, envs)
}
