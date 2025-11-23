package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/executor"
)

func HandleDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	envID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid environment ID", http.StatusBadRequest)
		return
	}

	if err := executor.DeleteEnvironment(r.Context(), envID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
