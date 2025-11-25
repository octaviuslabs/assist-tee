package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/models"
)

func init() {
	// Initialize logger for tests
	logger.Init(nil)
}

func TestHandleSetup_Success(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	reqBody := models.SetupRequest{
		MainModule: "main.ts",
		Modules: map[string]string{
			"main.ts": "export function handler() { return 'hello'; }",
		},
		TTLSeconds: 3600,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp models.Environment
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.MainModule != "main.ts" {
		t.Errorf("expected MainModule 'main.ts', got '%s'", resp.MainModule)
	}

	if resp.Status != "ready" {
		t.Errorf("expected Status 'ready', got '%s'", resp.Status)
	}

	if len(mock.SetupCalls) != 1 {
		t.Errorf("expected 1 setup call, got %d", len(mock.SetupCalls))
	}
}

func TestHandleSetup_InvalidJSON(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != "invalid_request" {
		t.Errorf("expected code 'invalid_request', got '%s'", resp.Code)
	}
}

func TestHandleSetup_MissingMainModule(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	reqBody := models.SetupRequest{
		MainModule: "", // Missing
		Modules: map[string]string{
			"main.ts": "export function handler() {}",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got '%s'", resp.Code)
	}
}

func TestHandleSetup_EmptyModules(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	reqBody := models.SetupRequest{
		MainModule: "main.ts",
		Modules:    map[string]string{}, // Empty
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got '%s'", resp.Code)
	}
}

func TestHandleSetup_MainModuleNotInModules(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	reqBody := models.SetupRequest{
		MainModule: "main.ts",
		Modules: map[string]string{
			"other.ts": "export function handler() {}", // main.ts not present
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got '%s'", resp.Code)
	}
}

func TestHandleSetup_ExecutorError(t *testing.T) {
	mock := executor.NewMockExecutor()
	mock.SetupFunc = func(ctx context.Context, req *models.SetupRequest) (*models.Environment, error) {
		return nil, errors.New("docker volume creation failed")
	}
	server := NewServer(mock)

	reqBody := models.SetupRequest{
		MainModule: "main.ts",
		Modules: map[string]string{
			"main.ts": "export function handler() {}",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/environments/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleSetup(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "setup_failed" {
		t.Errorf("expected code 'setup_failed', got '%s'", resp.Code)
	}

	if resp.Error != "docker volume creation failed" {
		t.Errorf("expected error message 'docker volume creation failed', got '%s'", resp.Error)
	}
}
