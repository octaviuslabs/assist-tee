package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/models"
)

func TestHandleExecute_Success(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	envID := uuid.New()
	reqBody := models.ExecuteRequest{
		Data: map[string]interface{}{
			"name": "test",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/"+envID.String()+"/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Set up mux vars
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp models.ExecutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", resp.ExitCode)
	}

	if len(mock.ExecuteCalls) != 1 {
		t.Errorf("expected 1 execute call, got %d", len(mock.ExecuteCalls))
	}

	if mock.ExecuteCalls[0].EnvID != envID {
		t.Errorf("expected envID %s, got %s", envID, mock.ExecuteCalls[0].EnvID)
	}
}

func TestHandleExecute_InvalidID(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	reqBody := models.ExecuteRequest{
		Data: map[string]interface{}{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/not-a-uuid/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Set up mux vars with invalid UUID
	req = mux.SetURLVars(req, map[string]string{"id": "not-a-uuid"})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "invalid_id" {
		t.Errorf("expected code 'invalid_id', got '%s'", resp.Code)
	}

	if len(mock.ExecuteCalls) != 0 {
		t.Errorf("expected 0 execute calls, got %d", len(mock.ExecuteCalls))
	}
}

func TestHandleExecute_InvalidJSON(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	envID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/environments/"+envID.String()+"/execute", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "invalid_request" {
		t.Errorf("expected code 'invalid_request', got '%s'", resp.Code)
	}
}

func TestHandleExecute_WithLimits(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	envID := uuid.New()
	reqBody := models.ExecuteRequest{
		Data: map[string]interface{}{"key": "value"},
		Limits: &models.ResourceLimits{
			TimeoutMs: 10000,
			MemoryMb:  256,
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/"+envID.String()+"/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify limits were passed to executor
	if len(mock.ExecuteCalls) != 1 {
		t.Fatalf("expected 1 execute call, got %d", len(mock.ExecuteCalls))
	}

	if mock.ExecuteCalls[0].Req.Limits == nil {
		t.Fatal("expected limits to be passed to executor")
	}

	if mock.ExecuteCalls[0].Req.Limits.TimeoutMs != 10000 {
		t.Errorf("expected TimeoutMs 10000, got %d", mock.ExecuteCalls[0].Req.Limits.TimeoutMs)
	}

	if mock.ExecuteCalls[0].Req.Limits.MemoryMb != 256 {
		t.Errorf("expected MemoryMb 256, got %d", mock.ExecuteCalls[0].Req.Limits.MemoryMb)
	}
}

func TestHandleExecute_ExecutorError(t *testing.T) {
	mock := executor.NewMockExecutor()
	mock.ExecuteFunc = func(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error) {
		return nil, errors.New("container execution failed")
	}
	server := NewServer(mock)

	envID := uuid.New()
	reqBody := models.ExecuteRequest{
		Data: map[string]interface{}{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/"+envID.String()+"/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "execution_failed" {
		t.Errorf("expected code 'execution_failed', got '%s'", resp.Code)
	}

	if resp.Error != "container execution failed" {
		t.Errorf("expected error 'container execution failed', got '%s'", resp.Error)
	}
}

func TestHandleExecute_NonZeroExitCode(t *testing.T) {
	mock := executor.NewMockExecutor()
	mock.ExecuteFunc = func(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error) {
		return &models.ExecutionResponse{
			ID:         uuid.New(),
			ExitCode:   1,
			Stdout:     "",
			Stderr:     "Error: something went wrong",
			DurationMs: 50,
		}, nil
	}
	server := NewServer(mock)

	envID := uuid.New()
	reqBody := models.ExecuteRequest{
		Data: map[string]interface{}{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/"+envID.String()+"/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleExecute(rec, req)

	// Non-zero exit code is still a successful HTTP response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp models.ExecutionResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", resp.ExitCode)
	}

	if resp.Stderr != "Error: something went wrong" {
		t.Errorf("expected stderr 'Error: something went wrong', got '%s'", resp.Stderr)
	}
}
