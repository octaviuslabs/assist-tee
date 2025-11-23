package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/models"
)

func HandleSetup(w http.ResponseWriter, r *http.Request) {
	var req models.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.MainModule == "" {
		http.Error(w, "mainModule is required", http.StatusBadRequest)
		return
	}
	if len(req.Modules) == 0 {
		http.Error(w, "modules cannot be empty", http.StatusBadRequest)
		return
	}
	if _, exists := req.Modules[req.MainModule]; !exists {
		http.Error(w, "mainModule must exist in modules map", http.StatusBadRequest)
		return
	}

	env, err := executor.SetupEnvironment(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(env)
}
