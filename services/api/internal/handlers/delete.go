package handlers

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/logger"
)

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
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

	log.Info("delete request received",
		slog.String("environment_id", envID.String()),
	)

	done := logger.LogOperation(ctx, "delete_environment",
		slog.String("environment_id", envID.String()),
	)

	err = s.Executor.DeleteEnvironment(ctx, envID)
	done(err)

	if err != nil {
		log.Error("environment deletion failed",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		writeErrorWithCode(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}

	log.Info("environment deleted",
		slog.String("environment_id", envID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
