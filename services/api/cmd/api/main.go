package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/executor"
	"github.com/jsfour/assist-tee/internal/handlers"
	"github.com/jsfour/assist-tee/internal/reaper"
)

func main() {
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Println("  TEE API Server - Trusted Execution Environment")
	fmt.Println("=" + strings.Repeat("=", 78))

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
	} else {
		fmt.Println()
		fmt.Println("✓ gVisor sandboxing: ENABLED")
		fmt.Println("  All code executions will run in hardware-virtualized containers")
		fmt.Println()
	}

	// Connect to database
	fmt.Println("Connecting to database...")
	if err := database.Connect(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Database connected")

	// Initialize schema
	if err := database.InitSchema(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Database schema initialized")

	// Reconcile environments on boot
	fmt.Println("Reconciling environments...")
	if err := reaper.ReconcileEnvironments(); err != nil {
		log.Printf("Warning: reconciliation failed: %v\n", err)
	}
	fmt.Println("✓ Environment reconciliation complete")

	// Start background reaper
	reaper.StartReaper()
	fmt.Println("✓ Background reaper started")

	// Setup routes
	r := mux.NewRouter()
	r.HandleFunc("/environments/setup", handlers.HandleSetup).Methods("POST")
	r.HandleFunc("/environments/{id}/execute", handlers.HandleExecute).Methods("POST")
	r.HandleFunc("/environments/{id}", handlers.HandleDelete).Methods("DELETE")
	r.HandleFunc("/environments", handlers.HandleList).Methods("GET")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Start server
	port := ":8080"
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("🚀 TEE API server listening on %s\n", port)
	fmt.Println(strings.Repeat("=", 80))
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatal(err)
	}
}
