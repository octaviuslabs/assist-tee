package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/models"
)

func HandleExecute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	envID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid environment ID", http.StatusBadRequest)
		return
	}

	var req models.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := executor.ExecuteInEnvironment(r.Context(), envID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
