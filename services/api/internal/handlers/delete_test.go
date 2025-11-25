package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/executor"
)

func TestHandleDelete_Success(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	envID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/environments/"+envID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleDelete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	if len(mock.DeleteCalls) != 1 {
		t.Errorf("expected 1 delete call, got %d", len(mock.DeleteCalls))
	}

	if mock.DeleteCalls[0].EnvID != envID {
		t.Errorf("expected envID %s, got %s", envID, mock.DeleteCalls[0].EnvID)
	}
}

func TestHandleDelete_InvalidID(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodDelete, "/environments/not-a-uuid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "not-a-uuid"})

	rec := httptest.NewRecorder()
	server.HandleDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "invalid_id" {
		t.Errorf("expected code 'invalid_id', got '%s'", resp.Code)
	}

	if len(mock.DeleteCalls) != 0 {
		t.Errorf("expected 0 delete calls, got %d", len(mock.DeleteCalls))
	}
}

func TestHandleDelete_ExecutorError(t *testing.T) {
	mock := executor.NewMockExecutor()
	mock.DeleteFunc = func(ctx context.Context, envID uuid.UUID) error {
		return errors.New("volume not found")
	}
	server := NewServer(mock)

	envID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/environments/"+envID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": envID.String()})

	rec := httptest.NewRecorder()
	server.HandleDelete(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "delete_failed" {
		t.Errorf("expected code 'delete_failed', got '%s'", resp.Code)
	}

	if resp.Error != "volume not found" {
		t.Errorf("expected error 'volume not found', got '%s'", resp.Error)
	}
}

func TestHandleDelete_EmptyID(t *testing.T) {
	mock := executor.NewMockExecutor()
	server := NewServer(mock)

	req := httptest.NewRequest(http.MethodDelete, "/environments/", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	rec := httptest.NewRecorder()
	server.HandleDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Code != "invalid_id" {
		t.Errorf("expected code 'invalid_id', got '%s'", resp.Code)
	}
}
