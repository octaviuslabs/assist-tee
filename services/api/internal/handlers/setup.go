package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/models"
)

func (s *Server) HandleSetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req models.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("failed to decode setup request",
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Log request details
	depCount := 0
	if req.Dependencies != nil {
		depCount = len(req.Dependencies.NPM) + len(req.Dependencies.Deno)
	}
	log.Info("setup request received",
		slog.String("main_module", req.MainModule),
		slog.Int("module_count", len(req.Modules)),
		slog.Int("dependency_count", depCount),
		slog.Int("ttl_seconds", req.TTLSeconds),
	)

	// Validate request
	if req.MainModule == "" {
		log.Warn("validation failed: mainModule is required")
		writeErrorWithCode(w, http.StatusBadRequest, "validation_error", "mainModule is required")
		return
	}
	if len(req.Modules) == 0 {
		log.Warn("validation failed: modules cannot be empty")
		writeErrorWithCode(w, http.StatusBadRequest, "validation_error", "modules cannot be empty")
		return
	}
	if _, exists := req.Modules[req.MainModule]; !exists {
		log.Warn("validation failed: mainModule must exist in modules map",
			slog.String("main_module", req.MainModule),
		)
		writeErrorWithCode(w, http.StatusBadRequest, "validation_error", "mainModule must exist in modules map")
		return
	}

	done := logger.LogOperation(ctx, "setup_environment",
		slog.String("main_module", req.MainModule),
		slog.Int("module_count", len(req.Modules)),
	)

	env, err := s.Executor.SetupEnvironment(ctx, &req)
	done(err)

	if err != nil {
		log.Error("environment setup failed",
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusInternalServerError, "setup_failed", err.Error())
		return
	}

	log.Info("environment created",
		slog.String("environment_id", env.ID.String()),
		slog.String("volume_name", env.VolumeName),
		slog.String("status", env.Status),
	)

	writeJSON(w, http.StatusOK, env)
}
