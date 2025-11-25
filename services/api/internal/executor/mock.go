package executor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/models"
)

// MockExecutor implements Executor interface for testing.
type MockExecutor struct {
	// SetupFunc is called when SetupEnvironment is invoked.
	// If nil, returns a default successful response.
	SetupFunc func(ctx context.Context, req *models.SetupRequest) (*models.Environment, error)

	// ExecuteFunc is called when ExecuteInEnvironment is invoked.
	// If nil, returns a default successful response.
	ExecuteFunc func(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error)

	// DeleteFunc is called when DeleteEnvironment is invoked.
	// If nil, returns nil (success).
	DeleteFunc func(ctx context.Context, envID uuid.UUID) error

	// Call tracking
	SetupCalls   []SetupCall
	ExecuteCalls []ExecuteCall
	DeleteCalls  []DeleteCall
}

// SetupCall records a call to SetupEnvironment.
type SetupCall struct {
	Ctx context.Context
	Req *models.SetupRequest
}

// ExecuteCall records a call to ExecuteInEnvironment.
type ExecuteCall struct {
	Ctx   context.Context
	EnvID uuid.UUID
	Req   *models.ExecuteRequest
}

// DeleteCall records a call to DeleteEnvironment.
type DeleteCall struct {
	Ctx   context.Context
	EnvID uuid.UUID
}

// NewMockExecutor creates a new MockExecutor with default behavior.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{}
}

// SetupEnvironment implements Executor.
func (m *MockExecutor) SetupEnvironment(ctx context.Context, req *models.SetupRequest) (*models.Environment, error) {
	m.SetupCalls = append(m.SetupCalls, SetupCall{Ctx: ctx, Req: req})

	if m.SetupFunc != nil {
		return m.SetupFunc(ctx, req)
	}

	// Default: return a successful environment
	return &models.Environment{
		ID:             uuid.New(),
		VolumeName:     "tee-env-mock-" + uuid.New().String(),
		MainModule:     req.MainModule,
		CreatedAt:      time.Now(),
		ExecutionCount: 0,
		Status:         "ready",
		TTLSeconds:     req.TTLSeconds,
	}, nil
}

// ExecuteInEnvironment implements Executor.
func (m *MockExecutor) ExecuteInEnvironment(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error) {
	m.ExecuteCalls = append(m.ExecuteCalls, ExecuteCall{Ctx: ctx, EnvID: envID, Req: req})

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, envID, req)
	}

	// Default: return a successful execution
	return &models.ExecutionResponse{
		ID:         uuid.New(),
		ExitCode:   0,
		Stdout:     `{"result": "mock"}`,
		Stderr:     "",
		DurationMs: 100,
	}, nil
}

// DeleteEnvironment implements Executor.
func (m *MockExecutor) DeleteEnvironment(ctx context.Context, envID uuid.UUID) error {
	m.DeleteCalls = append(m.DeleteCalls, DeleteCall{Ctx: ctx, EnvID: envID})

	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, envID)
	}

	// Default: return success
	return nil
}

// Reset clears all recorded calls.
func (m *MockExecutor) Reset() {
	m.SetupCalls = nil
	m.ExecuteCalls = nil
	m.DeleteCalls = nil
}

// Verify MockExecutor implements Executor interface
var _ Executor = (*MockExecutor)(nil)
