package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/handlers"
	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/middleware"
	"github.com/jsfour/assist-tee/internal/reaper"
)

func main() {
	// Initialize logger first (before any logging)
	logger.Init(&logger.Config{
		Level:      slog.LevelInfo,
		JSONFormat: true,
		AddSource:  false,
	})

	// Print startup banner to stdout (not through logger for visual clarity)
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("  TEE API Server - Trusted Execution Environment")
	fmt.Println("=" + strings.Repeat("=", 78))

	logger.Log.Info("server starting",
		slog.String("version", "1.0.0"),
	)

	// Check gVisor status and display warnings
	if executor.IsGVisorDisabled() {
		fmt.Println()
		fmt.Println("╔" + strings.Repeat("═", 78) + "╗")
		fmt.Println("║" + strings.Repeat(" ", 78) + "║")
		fmt.Println("║  ⚠️  ⚠️  ⚠️   SECURITY WARNING: gVisor is DISABLED   ⚠️  ⚠️  ⚠️          ║")
		fmt.Println("║" + strings.Repeat(" ", 78) + "║")
		fmt.Println("║  Code execution is NOT sandboxed with hardware virtualization!        ║")
		fmt.Println("║  User code can potentially:                                           ║")
		fmt.Println("║    - Access the host kernel                                           ║")
		fmt.Println("║    - Exploit kernel vulnerabilities                                   ║")
		fmt.Println("║    - Perform timing attacks                                           ║")
		fmt.Println("║                                                                        ║")
		fmt.Println("║  This mode should ONLY be used for:                                   ║")
		fmt.Println("║    - Local development on non-Linux systems (macOS/Windows)           ║")
		fmt.Println("║    - Testing purposes                                                 ║")
		fmt.Println("║                                                                        ║")
		fmt.Println("║  DO NOT USE IN PRODUCTION!                                            ║")
		fmt.Println("║                                                                        ║")
		fmt.Println("║  To enable gVisor security:                                           ║")
		fmt.Println("║    1. Remove DISABLE_GVISOR environment variable                      ║")
		fmt.Println("║    2. Ensure runsc is installed: sudo runsc install                   ║")
		fmt.Println("║    3. Restart the service                                             ║")
		fmt.Println("║" + strings.Repeat(" ", 78) + "║")
		fmt.Println("╚" + strings.Repeat("═", 78) + "╝")
		fmt.Println()

		logger.Log.Warn("gVisor is DISABLED - code execution is not sandboxed",
			slog.String("security", "degraded"),
		)
	} else {
		fmt.Println()
		fmt.Println("✓ gVisor sandboxing: ENABLED")
		fmt.Println("  All code executions will run in hardware-virtualized containers")
		fmt.Println()

		logger.Log.Info("gVisor sandboxing enabled",
			slog.String("security", "full"),
		)
	}

	// Connect to database
	logger.Log.Info("connecting to database")
	if err := database.Connect(); err != nil {
		logger.Log.Error("failed to connect to database",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Initialize schema
	if err := database.InitSchema(); err != nil {
		logger.Log.Error("failed to initialize database schema",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Reconcile environments on boot
	logger.Log.Info("reconciling environments on boot")
	if err := reaper.ReconcileEnvironments(); err != nil {
		logger.Log.Warn("reconciliation failed",
			slog.String("error", err.Error()),
		)
	}

	// Start background reaper
	reaper.StartReaper()

	// Setup routes
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/environments/setup", handlers.HandleSetup).Methods("POST")
	r.HandleFunc("/environments/{id}/execute", handlers.HandleExecute).Methods("POST")
	r.HandleFunc("/environments/{id}", handlers.HandleDelete).Methods("DELETE")
	r.HandleFunc("/environments", handlers.HandleList).Methods("GET")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Apply middleware (order matters: recovery -> logging -> routes)
	handler := middleware.Recovery(middleware.RequestLogging(r))

	// Start server
	port := getEnv("PORT", "8080")
	addr := ":" + port

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("🚀 TEE API server listening on %s\n", addr)
	fmt.Println(strings.Repeat("=", 80))

	logger.Log.Info("server listening",
		slog.String("address", addr),
		slog.String("port", port),
	)

	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Log.Error("server failed",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
