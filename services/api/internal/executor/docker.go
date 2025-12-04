package executor

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jsfour/assist-tee/internal/database"
	"github.com/jsfour/assist-tee/internal/logger"
	"github.com/jsfour/assist-tee/internal/models"
)

var execSemaphore = make(chan struct{}, 50) // Max 50 concurrent executions

// RuntimeImage returns the Docker image to use for code execution based on runtime type
func RuntimeImage(runtime models.Runtime) string {
	switch runtime {
	case models.RuntimeBun:
		if img := os.Getenv("RUNTIME_IMAGE_BUN"); img != "" {
			return img
		}
		return "octaviusdeployment/assist-tee-rt-bun:latest"
	default: // deno is the default
		if img := os.Getenv("RUNTIME_IMAGE_DENO"); img != "" {
			return img
		}
		// Also check legacy RUNTIME_IMAGE for backwards compatibility
		if img := os.Getenv("RUNTIME_IMAGE"); img != "" {
			return img
		}
		return "octaviusdeployment/assist-tee-rt-deno:latest"
	}
}

// RuntimeUserID returns the UID of the user in the runtime container
func RuntimeUserID(runtime models.Runtime) string {
	switch runtime {
	case models.RuntimeBun:
		return "1000" // bun user in oven/bun image
	default:
		return "1000" // deno user in denoland/deno image
	}
}

// IsGVisorDisabled checks if gVisor is disabled via environment variable
func IsGVisorDisabled() bool {
	return os.Getenv("DISABLE_GVISOR") == "true" || os.Getenv("DISABLE_GVISOR") == "1"
}

func (e *DockerExecutor) SetupEnvironment(ctx context.Context, req *models.SetupRequest) (*models.Environment, error) {
	envID := uuid.New()
	volumeName := fmt.Sprintf("tee-env-%s", envID.String())
	log := logger.FromContext(ctx)

	// Default to deno runtime if not specified
	runtime := req.Runtime
	if runtime == "" {
		runtime = models.RuntimeDeno
	}

	log.Debug("starting environment setup",
		slog.String("environment_id", envID.String()),
		slog.String("volume_name", volumeName),
		slog.String("main_module", req.MainModule),
		slog.String("runtime", string(runtime)),
		slog.Int("module_count", len(req.Modules)),
	)

	// 1. Create Docker volume
	log.Debug("creating docker volume",
		slog.String("volume_name", volumeName),
	)
	cmd := exec.CommandContext(ctx, "docker", "volume", "create", volumeName)
	if err := cmd.Run(); err != nil {
		log.Error("failed to create docker volume",
			slog.String("volume_name", volumeName),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// 2. Write modules to volume
	// The deno user in the container has UID 1000, so we need to set ownership
	for filename, content := range req.Modules {
		log.Debug("writing module to volume",
			slog.String("filename", filename),
			slog.Int("content_length", len(content)),
		)

		// Escape single quotes in content
		escapedContent := strings.ReplaceAll(content, "'", "'\\''")

		writeCmd := fmt.Sprintf("cat > /workspace/%s <<'EOF'\n%s\nEOF", filename, escapedContent)
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", volumeName),
			"busybox:latest",
			"sh", "-c", writeCmd,
		)

		if err := cmd.Run(); err != nil {
			log.Error("failed to write module",
				slog.String("filename", filename),
				slog.String("error", err.Error()),
			)
			// Cleanup volume on failure
			exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
			return nil, fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// 2b. Fix ownership for runtime user (UID 1000 in both deno and bun images)
	log.Debug("setting volume ownership for runtime user",
		slog.String("runtime", string(runtime)),
		slog.String("uid", RuntimeUserID(runtime)),
	)
	chownCmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", volumeName),
		"busybox:latest",
		"sh", "-c", fmt.Sprintf("chown -R %s:%s /workspace", RuntimeUserID(runtime), RuntimeUserID(runtime)),
	)
	if err := chownCmd.Run(); err != nil {
		log.Warn("failed to set volume ownership",
			slog.String("error", err.Error()),
		)
		// Don't fail - it might still work if deps aren't needed
	}

	log.Debug("all modules written successfully",
		slog.Int("module_count", len(req.Modules)),
	)

	// 3. Install dependencies (if specified)
	if req.Dependencies != nil && (len(req.Dependencies.NPM) > 0 || len(req.Dependencies.Deno) > 0) {
		depCount := len(req.Dependencies.NPM) + len(req.Dependencies.Deno)
		log.Info("installing dependencies",
			slog.String("environment_id", envID.String()),
			slog.String("runtime", string(runtime)),
			slog.Int("npm_count", len(req.Dependencies.NPM)),
			slog.Int("deno_count", len(req.Dependencies.Deno)),
			slog.Int("total_count", depCount),
		)

		if err := installDependencies(ctx, volumeName, req.Dependencies, runtime); err != nil {
			log.Error("dependency installation failed",
				slog.String("environment_id", envID.String()),
				slog.String("error", err.Error()),
			)
			// Cleanup volume on failure
			exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
			return nil, fmt.Errorf("failed to install dependencies: %w", err)
		}

		log.Info("dependencies installed successfully",
			slog.String("environment_id", envID.String()),
		)
	}

	// 4. Store metadata
	ttl := req.TTLSeconds
	if ttl == 0 {
		ttl = 3600 // Default 1 hour
	}

	depCount := 0
	if req.Dependencies != nil {
		depCount = len(req.Dependencies.NPM) + len(req.Dependencies.Deno)
	}

	metadata := map[string]interface{}{
		"permissions":     req.Permissions,
		"moduleCount":     len(req.Modules),
		"dependencyCount": depCount,
		"hasDependencies": depCount > 0,
		"runtime":         string(runtime),
	}
	metadataJSON, _ := json.Marshal(metadata)

	log.Debug("storing environment metadata",
		slog.String("environment_id", envID.String()),
		slog.String("runtime", string(runtime)),
		slog.Int("ttl_seconds", ttl),
	)

	_, err := database.DB.ExecContext(ctx, `
		INSERT INTO environments (id, volume_name, main_module, runtime, metadata, ttl_seconds)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, envID, volumeName, req.MainModule, string(runtime), metadataJSON, ttl)

	if err != nil {
		log.Error("failed to store environment in database",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		// Cleanup volume on DB failure
		exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
		return nil, fmt.Errorf("failed to store environment: %w", err)
	}

	log.Info("environment setup completed",
		slog.String("environment_id", envID.String()),
		slog.String("volume_name", volumeName),
		slog.String("main_module", req.MainModule),
		slog.String("runtime", string(runtime)),
		slog.Int("module_count", len(req.Modules)),
		slog.Int("dependency_count", depCount),
		slog.Int("ttl_seconds", ttl),
	)

	return &models.Environment{
		ID:             envID,
		VolumeName:     volumeName,
		MainModule:     req.MainModule,
		Runtime:        runtime,
		CreatedAt:      time.Now(),
		ExecutionCount: 0,
		Status:         "ready",
		Metadata:       metadata,
		TTLSeconds:     ttl,
	}, nil
}

func (e *DockerExecutor) ExecuteInEnvironment(ctx context.Context, envID uuid.UUID, req *models.ExecuteRequest) (*models.ExecutionResponse, error) {
	log := logger.FromContext(ctx)

	// Acquire semaphore
	log.Debug("acquiring execution semaphore",
		slog.String("environment_id", envID.String()),
	)
	select {
	case execSemaphore <- struct{}{}:
		defer func() { <-execSemaphore }()
	case <-ctx.Done():
		log.Warn("context cancelled while waiting for semaphore",
			slog.String("environment_id", envID.String()),
		)
		return nil, ctx.Err()
	}

	// 1. Look up environment
	var volumeName, mainModule string
	var runtimeStr sql.NullString
	var metadataJSON []byte
	err := database.DB.QueryRowContext(ctx, `
		SELECT volume_name, main_module, runtime, metadata
		FROM environments
		WHERE id = $1 AND status = 'ready'
	`, envID).Scan(&volumeName, &mainModule, &runtimeStr, &metadataJSON)

	if err == sql.ErrNoRows {
		log.Warn("environment not found or not ready",
			slog.String("environment_id", envID.String()),
		)
		return nil, fmt.Errorf("environment not found or not ready")
	} else if err != nil {
		log.Error("database query failed",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// Parse runtime, default to deno for backwards compatibility
	runtime := models.RuntimeDeno
	if runtimeStr.Valid && runtimeStr.String != "" {
		runtime = models.Runtime(runtimeStr.String)
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
		log.Error("failed to marshal execution input",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	log.Debug("starting container execution",
		slog.String("environment_id", envID.String()),
		slog.String("execution_id", execID.String()),
		slog.String("volume_name", volumeName),
		slog.String("main_module", mainModule),
		slog.String("runtime", string(runtime)),
		slog.Int("timeout_ms", timeoutMs),
		slog.Int("memory_mb", memoryMb),
	)

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
		log.Warn("gVisor is disabled - execution is not sandboxed",
			slog.String("environment_id", envID.String()),
			slog.String("execution_id", execID.String()),
		)
	}

	// Continue with other args
	args = append(args,
		"--network=none",
		"--read-only",
		fmt.Sprintf("--memory=%dm", memoryMb),
		"--cpus=0.5",
		"--pids-limit=100",
		"-v", fmt.Sprintf("%s:/workspace:ro", volumeName),
	)

	// Add runtime-specific cache directory mounts and environment variables
	switch runtime {
	case models.RuntimeBun:
		args = append(args,
			"-v", fmt.Sprintf("%s:/home/bun/.bun:ro", volumeName), // Bun cache location
		)
	default: // deno
		args = append(args,
			"-v", fmt.Sprintf("%s:/deno-dir:ro", volumeName), // Deno cache location
			"-e", "DENO_DIR=/deno-dir",                       // Tell Deno where to find cache
		)
	}

	args = append(args, RuntimeImage(runtime))

	// 5. Execute with stdin
	startTime := time.Now()
	cmd := exec.CommandContext(execCtx, "docker", args...)
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Create streaming writers that log output in real-time
	stdoutWriter := &streamingWriter{
		log:    log,
		stream: "stdout",
		prefix: "execution output",
		envID:  envID.String(),
		execID: execID.String(),
	}
	stderrWriter := &streamingWriter{
		log:    log,
		stream: "stderr",
		prefix: "execution output",
		envID:  envID.String(),
		execID: execID.String(),
	}

	// Also capture full output for parsing the result
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(stdoutWriter, &stdout)
	cmd.Stderr = io.MultiWriter(stderrWriter, &stderr)

	err = cmd.Run()

	// Flush any remaining buffered output
	stdoutWriter.Flush()
	stderrWriter.Flush()
	duration := time.Since(startTime)

	// 6. Handle exit
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			log.Debug("execution completed with non-zero exit",
				slog.String("execution_id", execID.String()),
				slog.Int("exit_code", exitCode),
			)
		} else if execCtx.Err() == context.DeadlineExceeded {
			log.Warn("execution timeout exceeded",
				slog.String("environment_id", envID.String()),
				slog.String("execution_id", execID.String()),
				slog.Int("timeout_ms", timeoutMs),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)
			return &models.ExecutionResponse{
				ID:         execID,
				ExitCode:   124,
				Stderr:     "Execution timeout exceeded",
				DurationMs: duration.Milliseconds(),
			}, nil
		} else {
			log.Error("execution failed",
				slog.String("environment_id", envID.String()),
				slog.String("execution_id", execID.String()),
				slog.String("error", err.Error()),
			)
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

	log.Debug("execution output parsed",
		slog.String("execution_id", execID.String()),
		slog.Bool("success", output.Success),
		slog.Int("stdout_length", len(stdoutStr)),
		slog.Int("stderr_length", len(stderrStr)),
	)

	// 8. Store execution record
	_, dbErr := database.DB.ExecContext(ctx, `
		INSERT INTO executions
		(id, environment_id, exit_code, stdout, stderr, duration_ms, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, execID, envID, exitCode, resultJSON, stderrStr, duration.Milliseconds())

	if dbErr != nil {
		log.Warn("failed to store execution record",
			slog.String("execution_id", execID.String()),
			slog.String("error", dbErr.Error()),
		)
	}

	// 9. Update stats
	_, dbErr = database.DB.ExecContext(ctx, `
		UPDATE environments
		SET execution_count = execution_count + 1,
			last_executed_at = NOW()
		WHERE id = $1
	`, envID)

	if dbErr != nil {
		log.Warn("failed to update environment stats",
			slog.String("environment_id", envID.String()),
			slog.String("error", dbErr.Error()),
		)
	}

	log.Info("execution completed",
		slog.String("environment_id", envID.String()),
		slog.String("execution_id", execID.String()),
		slog.Int("exit_code", exitCode),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Bool("success", exitCode == 0),
	)

	return &models.ExecutionResponse{
		ID:         execID,
		ExitCode:   exitCode,
		Stdout:     resultJSON,
		Stderr:     stderrStr,
		DurationMs: duration.Milliseconds(),
	}, nil
}

func (e *DockerExecutor) DeleteEnvironment(ctx context.Context, envID uuid.UUID) error {
	log := logger.FromContext(ctx)

	// Get volume name
	var volumeName string
	err := database.DB.QueryRowContext(ctx, "SELECT volume_name FROM environments WHERE id = $1", envID).Scan(&volumeName)
	if err != nil {
		log.Error("failed to find environment for deletion",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	log.Debug("deleting environment",
		slog.String("environment_id", envID.String()),
		slog.String("volume_name", volumeName),
	)

	// Remove volume
	if err := exec.Command("docker", "volume", "rm", "-f", volumeName).Run(); err != nil {
		log.Warn("failed to remove docker volume",
			slog.String("volume_name", volumeName),
			slog.String("error", err.Error()),
		)
	}

	// Delete from DB (cascades to executions)
	_, err = database.DB.ExecContext(ctx, "DELETE FROM environments WHERE id = $1", envID)
	if err != nil {
		log.Error("failed to delete environment from database",
			slog.String("environment_id", envID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	log.Info("environment deleted",
		slog.String("environment_id", envID.String()),
		slog.String("volume_name", volumeName),
	)

	return nil
}

// streamingWriter wraps a logger to stream output line by line
type streamingWriter struct {
	log     *slog.Logger
	stream  string // "stdout" or "stderr"
	prefix  string // log message prefix (e.g., "dependency install", "execution")
	envID   string // optional environment ID for context
	execID  string // optional execution ID for context
	buffer  []byte
}

func (w *streamingWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		idx := bytes.IndexByte(w.buffer, '\n')
		if idx == -1 {
			break
		}

		line := string(w.buffer[:idx])
		w.buffer = w.buffer[idx+1:]

		if line != "" {
			attrs := []any{
				slog.String("stream", w.stream),
				slog.String("output", line),
			}
			if w.envID != "" {
				attrs = append(attrs, slog.String("env_id", w.envID))
			}
			if w.execID != "" {
				attrs = append(attrs, slog.String("exec_id", w.execID))
			}
			w.log.Info(w.prefix, attrs...)
		}
	}

	return len(p), nil
}

func (w *streamingWriter) Flush() {
	// Flush any remaining content
	if len(w.buffer) > 0 {
		attrs := []any{
			slog.String("stream", w.stream),
			slog.String("output", string(w.buffer)),
		}
		if w.envID != "" {
			attrs = append(attrs, slog.String("env_id", w.envID))
		}
		if w.execID != "" {
			attrs = append(attrs, slog.String("exec_id", w.execID))
		}
		w.log.Info(w.prefix, attrs...)
		w.buffer = nil
	}
}

// installDependencies caches dependencies in the volume with network access
func installDependencies(ctx context.Context, volumeName string, deps *models.Dependencies, runtime models.Runtime) error {
	if deps == nil {
		return nil
	}

	log := logger.FromContext(ctx)

	var cacheCommands []string
	var dockerArgs []string

	switch runtime {
	case models.RuntimeBun:
		// Build bun install commands
		if len(deps.NPM) > 0 {
			log.Info("preparing npm dependencies for bun",
				slog.Any("packages", deps.NPM),
			)
			// Bun can install all packages at once
			cacheCommands = append(cacheCommands, fmt.Sprintf("bun install %s", strings.Join(deps.NPM, " ")))
		}

		// Deno-specific URLs are not supported in bun
		if len(deps.Deno) > 0 {
			log.Warn("deno dependencies are not supported in bun runtime, ignoring",
				slog.Any("modules", deps.Deno),
			)
		}

		if len(cacheCommands) == 0 {
			log.Debug("no dependencies to install")
			return nil
		}

		cacheScript := strings.Join(cacheCommands, " && ")

		log.Info("starting dependency installation",
			slog.String("volume_name", volumeName),
			slog.String("runtime", string(runtime)),
			slog.Int("command_count", len(cacheCommands)),
			slog.String("script", cacheScript),
		)

		// Build docker command for bun
		dockerArgs = []string{
			"run", "--rm",
			"--entrypoint", "sh",
			"--network=bridge",
			"-v", fmt.Sprintf("%s:/workspace", volumeName),
			"-w", "/workspace",
			RuntimeImage(runtime),
			"-c", cacheScript,
		}

	default: // deno
		// Build deno cache commands
		if len(deps.NPM) > 0 {
			log.Info("preparing npm dependencies",
				slog.Any("packages", deps.NPM),
			)
			for _, pkg := range deps.NPM {
				cacheCommands = append(cacheCommands, fmt.Sprintf("deno cache --node-modules-dir npm:%s", pkg))
			}
		}

		// Cache deno dependencies
		if len(deps.Deno) > 0 {
			log.Info("preparing deno dependencies",
				slog.Any("modules", deps.Deno),
			)
			for _, url := range deps.Deno {
				cacheCommands = append(cacheCommands, fmt.Sprintf("deno cache %s", url))
			}
		}

		if len(cacheCommands) == 0 {
			log.Debug("no dependencies to install")
			return nil
		}

		cacheScript := strings.Join(cacheCommands, " && ")

		log.Info("starting dependency installation",
			slog.String("volume_name", volumeName),
			slog.String("runtime", string(runtime)),
			slog.Int("command_count", len(cacheCommands)),
			slog.String("script", cacheScript),
		)

		// Build docker command for deno
		dockerArgs = []string{
			"run", "--rm",
			"--entrypoint", "sh",
			"--network=bridge",
			"-v", fmt.Sprintf("%s:/workspace", volumeName),
			"-v", fmt.Sprintf("%s:/deno-dir", volumeName),
			"-e", "DENO_DIR=/deno-dir",
			"-w", "/workspace",
			RuntimeImage(runtime),
			"-c", cacheScript,
		}
	}

	// Run dependency installation with streaming output
	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Create streaming writers that log output in real-time
	stdoutWriter := &streamingWriter{log: log, stream: "stdout", prefix: "dependency install"}
	stderrWriter := &streamingWriter{log: log, stream: "stderr", prefix: "dependency install"}

	// Also capture full output for error reporting
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(stdoutWriter, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(stderrWriter, &stderrBuf)

	err := cmd.Run()

	// Flush any remaining buffered output
	stdoutWriter.Flush()
	stderrWriter.Flush()

	duration := time.Since(startTime)

	if err != nil {
		log.Error("dependency installation failed",
			slog.String("volume_name", volumeName),
			slog.String("runtime", string(runtime)),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", duration.Milliseconds()),
		)
		// Include both stdout and stderr in error for debugging
		combinedOutput := stderrBuf.String()
		if combinedOutput == "" {
			combinedOutput = stdoutBuf.String()
		}
		return fmt.Errorf("dependency installation failed: %w - output: %s", err, combinedOutput)
	}

	log.Info("dependency installation completed",
		slog.String("volume_name", volumeName),
		slog.String("runtime", string(runtime)),
		slog.Int64("duration_ms", duration.Milliseconds()),
	)

	return nil
}
