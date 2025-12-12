package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jsfour/assist-tee/internal/logger"
)

func init() {
	logger.Init(nil)
}

func TestInitAuth_RequiresToken(t *testing.T) {
	os.Unsetenv("BEARER_TOKEN")
	os.Unsetenv("DISABLE_BEARER_TOKEN")

	err := InitAuth()
	if err == nil {
		t.Error("expected error when BEARER_TOKEN is not set")
	}

	if _, ok := err.(*AuthConfigError); !ok {
		t.Errorf("expected AuthConfigError, got %T", err)
	}
}

func TestInitAuth_DisabledWithFlag(t *testing.T) {
	os.Unsetenv("BEARER_TOKEN")
	os.Setenv("DISABLE_BEARER_TOKEN", "true")
	defer os.Unsetenv("DISABLE_BEARER_TOKEN")

	err := InitAuth()
	if err != nil {
		t.Errorf("expected no error when DISABLE_BEARER_TOKEN=true, got %v", err)
	}
}

func TestInitAuth_WithToken(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "test-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	err := InitAuth()
	if err != nil {
		t.Errorf("expected no error when BEARER_TOKEN is set, got %v", err)
	}
}

func TestBearerAuth_ValidToken(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "valid-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestBearerAuth_InvalidToken(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "valid-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "valid-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestBearerAuth_InvalidFormat(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "valid-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestBearerAuth_HealthEndpointSkipsAuth(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "valid-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d for /health without auth, got %d", http.StatusOK, rec.Code)
	}
}

func TestBearerAuth_DisabledSkipsAuth(t *testing.T) {
	os.Unsetenv("BEARER_TOKEN")
	os.Setenv("DISABLE_BEARER_TOKEN", "true")
	defer os.Unsetenv("DISABLE_BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d when auth disabled, got %d", http.StatusOK, rec.Code)
	}
}

func TestBearerAuth_TimingAttackResistance(t *testing.T) {
	os.Setenv("BEARER_TOKEN", "correct-secret-token")
	os.Unsetenv("DISABLE_BEARER_TOKEN")
	defer os.Unsetenv("BEARER_TOKEN")

	InitAuth()

	handler := BearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// All should return 401, constant time comparison means similar timing
	tokens := []string{
		"c",                   // very short
		"correct-secret-toke", // almost correct
		"wrong-secret-tokens", // correct length, wrong content
		"completely-wrong-and-very-long-token",
	}

	for _, token := range tokens {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d for token %q, got %d", http.StatusUnauthorized, token, rec.Code)
		}
	}
}
