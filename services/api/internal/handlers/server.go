package handlers

import (
	"github.com/jsfour/assist-tee/internal/executor"
)

// Server holds the dependencies for HTTP handlers.
type Server struct {
	Executor executor.Executor
}

// NewServer creates a new Server with the given executor.
func NewServer(exec executor.Executor) *Server {
	return &Server{
		Executor: exec,
	}
}
