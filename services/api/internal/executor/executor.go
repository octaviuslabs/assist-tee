package executor

import (
	"context"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/models"
)

// Executor defines the interface for environment management and code execution.
// This allows for mocking in tests without requiring Docker.
type Executor interface {
	// SetupEnvironment creates a new execution environment with the given modules and dependencies.
	SetupEnvironment(ctx context.Context, req *models.SetupRequest) (*models.Environment, error)

	// ExecuteInEnvironment runs code in an existing environment and returns the result.
	ExecuteInEnvironment(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error)

	// DeleteEnvironment removes an environment and cleans up its resources.
	DeleteEnvironment(ctx context.Context, envID uuid.UUID) error
}

// DockerExecutor implements Executor using Docker containers.
type DockerExecutor struct{}

// NewDockerExecutor creates a new DockerExecutor instance.
func NewDockerExecutor() *DockerExecutor {
	return &DockerExecutor{}
}

// Verify DockerExecutor implements Executor interface
var _ Executor = (*DockerExecutor)(nil)
