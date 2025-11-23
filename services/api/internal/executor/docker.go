package executor

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/models"
)

var execSemaphore = make(chan struct{}, 50) // Max 50 concurrent executions

// IsGVisorDisabled checks if gVisor is disabled via environment variable
func IsGVisorDisabled() bool {
	return os.Getenv("DISABLE_GVISOR") == "true" || os.Getenv("DISABLE_GVISOR") == "1"
}

func SetupEnvironment(ctx context.Context, req *models.SetupRequest) (*models.Environment, error) {
	envID := uuid.New()
	volumeName := fmt.Sprintf("tee-env-%s", envID.String())

	// 1. Create Docker volume
	cmd := exec.CommandContext(ctx, "docker", "volume", "create", volumeName)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// 2. Write modules to volume
	for filename, content := range req.Modules {
		// Escape single quotes in content
		escapedContent := strings.ReplaceAll(content, "'", "'\\''")

		writeCmd := fmt.Sprintf("cat > /workspace/%s <<'EOF'\n%s\nEOF", filename, escapedContent)
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", volumeName),
			"busybox:latest",
			"sh", "-c", writeCmd,
		)

		if err := cmd.Run(); err != nil {
			// Cleanup volume on failure
			exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
			return nil, fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// 3. Store metadata
	ttl := req.TTLSeconds
	if ttl == 0 {
		ttl = 3600 // Default 1 hour
	}

	metadata := map[string]interface{}{
		"permissions": req.Permissions,
		"moduleCount": len(req.Modules),
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err := database.DB.ExecContext(ctx, `
		INSERT INTO environments (id, volume_name, main_module, metadata, ttl_seconds)
		VALUES ($1, $2, $3, $4, $5)
	`, envID, volumeName, req.MainModule, metadataJSON, ttl)

	if err != nil {
		// Cleanup volume on DB failure
		exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
		return nil, fmt.Errorf("failed to store environment: %w", err)
	}

	return &models.Environment{
		ID:             envID,
		VolumeName:     volumeName,
		MainModule:     req.MainModule,
		CreatedAt:      time.Now(),
		ExecutionCount: 0,
		Status:         "ready",
		Metadata:       metadata,
		TTLSeconds:     ttl,
	}, nil
}

func ExecuteInEnvironment(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error) {
	// Acquire semaphore
	select {
	case execSemaphore <- struct{}{}:
		defer func() { <-execSemaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 1. Look up environment
	var volumeName, mainModule string
	var metadataJSON []byte
	err := database.DB.QueryRowContext(ctx, `
		SELECT volume_name, main_module, metadata
		FROM environments
		WHERE id = $1 AND status = 'ready'
	`, envID).Scan(&volumeName, &mainModule, &metadataJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("environment not found or not ready")
	} else if err != nil {
		return nil, err
	}

	// Parse metadata for permissions
	var metadata map[string]interface{}
	if metadataJSON != nil {
		json.Unmarshal(metadataJSON, &metadata)
	}

	// 2. Apply limits
	timeoutMs := 5000
	memoryMb := 128
	if req.Limits != nil {
		if req.Limits.TimeoutMs > 0 {
			timeoutMs = req.Limits.TimeoutMs
		}
		if req.Limits.MemoryMb > 0 {
			memoryMb = req.Limits.MemoryMb
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// 3. Build execution input
	execID := uuid.New()
	executionInput := map[string]interface{}{
		"event": map[string]interface{}{
			"data": req.Data,
			"env":  req.Env,
		},
		"context": map[string]interface{}{
			"executionId":   execID.String(),
			"environmentId": envID.String(),
			"requestId":     execID.String(),
		},
		"mainModule": mainModule,
	}

	inputJSON, err := json.Marshal(executionInput)
	if err != nil {
		return nil, err
	}

	// 4. Build docker run command
	args := []string{
		"run",
		"--rm",
		"-i",
	}

	// Add gVisor runtime if not disabled
	if !IsGVisorDisabled() {
		args = append(args, "--runtime=runsc")
	} else {
		log.Println("⚠️  WARNING: gVisor is DISABLED - execution is NOT sandboxed!")
	}

	// Continue with other args
	args = append(args,
		"--network=none",
		"--read-only",
		fmt.Sprintf("--memory=%dm", memoryMb),
		"--cpus=0.5",
		"--pids-limit=100",
		"-v", fmt.Sprintf("%s:/workspace:ro", volumeName),
		"deno-runtime:latest",
	)

	// 5. Execute with stdin
	startTime := time.Now()
	cmd := exec.CommandContext(execCtx, "docker", args...)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(startTime)

	// 6. Handle exit
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			return &models.ExecutionResponse{
				ID:         execID,
				ExitCode:   124,
				Stderr:     "Execution timeout exceeded",
				DurationMs: duration.Milliseconds(),
			}, nil
		} else {
			return nil, fmt.Errorf("execution failed: %w", err)
		}
	}

	// 7. Parse structured output from stdout
	var output struct {
		Success bool        `json:"success"`
		Result  interface{} `json:"result"`
		Error   string      `json:"error"`
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	resultJSON := ""

	// Try to parse stdout as structured JSON
	if err := json.Unmarshal([]byte(stdoutStr), &output); err == nil {
		if output.Success {
			resultBytes, _ := json.Marshal(output.Result)
			resultJSON = string(resultBytes)
		} else {
			stderrStr = output.Error
			if exitCode == 0 {
				exitCode = 1
			}
		}
	} else {
		// Fallback: treat stdout as raw output
		resultJSON = stdoutStr
	}

	// 8. Store execution record
	database.DB.ExecContext(ctx, `
		INSERT INTO executions
		(id, environment_id, exit_code, stdout, stderr, duration_ms, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, execID, envID, exitCode, resultJSON, stderrStr, duration.Milliseconds())

	// 9. Update stats
	database.DB.ExecContext(ctx, `
		UPDATE environments
		SET execution_count = execution_count + 1,
			last_executed_at = NOW()
		WHERE id = $1
	`, envID)

	return &models.ExecutionResponse{
		ID:         execID,
		ExitCode:   exitCode,
		Stdout:     resultJSON,
		Stderr:     stderrStr,
		DurationMs: duration.Milliseconds(),
	}, nil
}

func DeleteEnvironment(ctx context.Context, envID uuid.UUID) error {
	// Get volume name
	var volumeName string
	err := database.DB.QueryRowContext(ctx, "SELECT volume_name FROM environments WHERE id = $1", envID).Scan(&volumeName)
	if err != nil {
		return err
	}

	// Remove volume
	exec.Command("docker", "volume", "rm", "-f", volumeName).Run()

	// Delete from DB (cascades to executions)
	_, err = database.DB.ExecContext(ctx, "DELETE FROM environments WHERE id = $1", envID)
	return err
}
