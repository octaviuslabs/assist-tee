package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jsfour/assist-tee/internal/logger"
)

var bearerToken string
var authDisabled bool

func InitAuth() error {
	bearerToken = os.Getenv("BEARER_TOKEN")
	authDisabled = os.Getenv("DISABLE_BEARER_TOKEN") == "true"

	if !authDisabled && bearerToken == "" {
		return &AuthConfigError{Message: "BEARER_TOKEN environment variable is required (set DISABLE_BEARER_TOKEN=true to disable)"}
	}

	if authDisabled {
		logger.Log.Warn("Bearer token authentication is DISABLED",
			slog.String("security", "degraded"),
		)
	}

	return nil
}

type AuthConfigError struct {
	Message string
}

func (e *AuthConfigError) Error() string {
	return e.Message
}

func BearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health checks (required for load balancers/k8s probes)
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if authDisabled {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			requestID := logger.GetRequestID(r.Context())
			logger.Log.Warn("missing authorization header",
				slog.String("request_id", requestID),
				slog.String("path", r.URL.Path),
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			requestID := logger.GetRequestID(r.Context())
			logger.Log.Warn("invalid authorization header format",
				slog.String("request_id", requestID),
				slog.String("path", r.URL.Path),
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(bearerToken)) != 1 {
			requestID := logger.GetRequestID(r.Context())
			logger.Log.Warn("invalid bearer token",
				slog.String("request_id", requestID),
				slog.String("path", r.URL.Path),
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
