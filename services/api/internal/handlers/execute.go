package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/models"
)

const maxExecuteBodySize = 1 << 20 // 1 MB for execute

func (s *Server) HandleExecute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	vars := mux.Vars(r)
	envID, err := uuid.Parse(vars["id"])
	if err != nil {
		log.Warn("invalid environment ID",
			slog.String("id", vars["id"]),
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_id", "Invalid environment ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxExecuteBodySize)

	var req models.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			log.Warn("request body too large",
				slog.String("environment_id", envID.String()),
				slog.Int64("limit", maxExecuteBodySize),
			)
			writeErrorWithCode(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body exceeds 1 MB limit")
			return
		}
		log.Warn("failed to decode execute request",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Log request details
	timeoutMs := 5000
	memoryMb := 128
	if req.Limits != nil {
		if req.Limits.TimeoutMs > 0 {
			timeoutMs = req.Limits.TimeoutMs
		}
		if req.Limits.MemoryMb > 0 {
			memoryMb = req.Limits.MemoryMb
		}
	}

	log.Info("execute request received",
		slog.String("environment_id", envID.String()),
		slog.Int("timeout_ms", timeoutMs),
		slog.Int("memory_mb", memoryMb),
	)

	done := logger.LogOperation(ctx, "execute_in_environment",
		slog.String("environment_id", envID.String()),
	)

	resp, err := s.Executor.ExecuteInEnvironment(ctx, envID, &req)
	done(err)

	if err != nil {
		log.Error("execution failed",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusInternalServerError, "execution_failed", err.Error())
		return
	}

	// Log execution result
	logger.LogExecutionResult(ctx, envID.String(), resp.ID.String(), resp.ExitCode, resp.DurationMs, nil)

	writeJSON(w, http.StatusOK, resp)
}
