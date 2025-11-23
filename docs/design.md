# Assist Trusted Execution Environment (TEE)

## Overview

The Trusted Execution Environment (TEE) is a service that allows AI agents and other callers to execute code in isolated, sandboxed environments. The system provides strong security guarantees through hardware virtualization (gVisor) while maintaining good performance through environment reuse.

### Inspirations

- [Cloudflare Code Mode](https://blog.cloudflare.com/code-mode/) - AI writes code to interact with MCP servers
- [Anthropic Code Execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp) - Secure code execution for AI agents

### Key Features

- **Lambda-style handler pattern** - Users export a `handler(event, context)` function
- **Two-phase execution** - Separate setup (slow) from execution (fast)
- **Environment reuse** - Set up once, execute many times
- **Strong isolation** - gVisor runtime provides VM-level security
- **Stateless execution** - Fresh container per execution via stdin/stdout
- **Automatic cleanup** - TTL-based reaping of unused environments

## Architecture

### High-Level System Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client (AI Agent)                       │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       │ 1. POST /environments/setup
                       │    (modules, mainModule, permissions)
                       ↓
┌─────────────────────────────────────────────────────────────────┐
│                        TEE API Service (Go)                     │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  HTTP Router (gorilla/mux)                                │  │
│  │  - POST   /environments/setup                             │  │
│  │  - POST   /environments/:id/execute                       │  │
│  │  - DELETE /environments/:id                               │  │
│  │  - GET    /environments                                   │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────┴─────────────────────────────┐   │
│  │         Environment Manager                             │   │
│  │  - Create/delete Docker volumes                         │   │
│  │  - Write modules to volumes                             │   │
│  │  - Spawn execution containers                           │   │
│  │  - Pipe stdin/stdout                                    │   │
│  └───────────────────────────────────────────────────────┬─┘   │
│                              │                             │     │
│  ┌───────────────────────────┴──────────┐  ┌─────────────▼───┐ │
│  │     PostgreSQL Database              │  │  Reaper Service │ │
│  │  - environments table                │  │  (background)   │ │
│  │  - executions table                  │  │  - TTL cleanup  │ │
│  └──────────────────────────────────────┘  └─────────────────┘ │
└──────────────────┬──────────────────────────────────────────────┘
                   │
                   │ docker run -i --runtime=runsc
                   │ + volume mount + stdin
                   ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Docker Host                                  │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Volume: env-{uuid}                                    │    │
│  │  ├── main.ts       (user's handler)                    │    │
│  │  ├── utils.ts      (user's modules)                    │    │
│  │  └── node_modules/ (future: cached dependencies)       │    │
│  └────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              │ mounted to                        │
│                              ↓                                   │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Execution Container (gVisor sandbox)                  │    │
│  │  ┌──────────────────────────────────────────────────┐  │    │
│  │  │  Deno Runtime                                    │  │    │
│  │  │  ┌────────────────────────────────────────────┐  │  │    │
│  │  │  │  /runtime/runner.ts                        │  │  │    │
│  │  │  │  1. Read JSON from stdin                   │  │  │    │
│  │  │  │  2. import /workspace/main.ts              │  │  │    │
│  │  │  │  3. result = handler(event, context)       │  │  │    │
│  │  │  │  4. Write JSON to stdout                   │  │  │    │
│  │  │  └────────────────────────────────────────────┘  │  │    │
│  │  └──────────────────────────────────────────────────┘  │    │
│  │  Resources: 128MB RAM, 0.5 CPU, no network            │    │
│  └────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Component Overview

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Go API     │────▶│  PostgreSQL  │     │    Docker    │
│   Service    │     │   Database   │     │    Daemon    │
│  (Router)    │     │  (Metadata)  │     │  (Runtime)   │
└──────┬───────┘     └──────────────┘     └──────┬───────┘
       │                                          │
       │  Manages lifecycle                       │
       └──────────────────────────────────────────┘
                 Creates/executes containers
```

## Two-Phase Execution Model

### Phase 1: Setup (Cold Start)

**Purpose:** Create and prepare an execution environment

```
POST /environments/setup
{
  "mainModule": "main.ts",
  "modules": {
    "main.ts": "export async function handler(event, ctx) { ... }",
    "utils.ts": "export const add = (a, b) => a + b;"
  },
  "permissions": {
    "allowNet": ["api.github.com:443"],
    "allowRead": true
  },
  "ttlSeconds": 3600
}

Response:
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "volumeName": "env-550e8400-...",
  "status": "ready",
  "createdAt": "2025-01-15T10:30:00Z"
}
```

**Operations:**
1. Generate environment UUID
2. Create Docker volume: `env-{uuid}`
3. Write all modules to volume using helper container
4. Store metadata in PostgreSQL
5. Return environment ID

**Timing:** ~500ms - 2s (one-time cost)

### Phase 2: Execute (Warm Execution)

**Purpose:** Run code in the prepared environment

```
POST /environments/{id}/execute
{
  "data": { "a": 5, "b": 3 },
  "env": { "DEBUG": "true" },
  "limits": {
    "timeoutMs": 5000,
    "memoryMb": 128
  }
}

Response:
{
  "id": "exec-abc123...",
  "exitCode": 0,
  "stdout": "{\"sum\":8}",
  "stderr": "",
  "durationMs": 127
}
```

**Operations:**
1. Look up environment in database
2. Build execution input JSON
3. Spawn container with:
   - Volume mounted read-only at `/workspace`
   - gVisor runtime
   - Resource limits (memory, CPU, timeout)
   - Network isolation
4. Pipe input JSON to container stdin
5. Container runs `/runtime/runner.ts`:
   - Reads stdin
   - Loads user's `handler` from volume
   - Calls `handler(event, context)`
   - Writes result to stdout
6. Capture stdout/stderr
7. Parse result
8. Store execution record
9. Update statistics

**Timing:** ~100-200ms per execution

## API Specification

### Endpoints

#### `POST /environments/setup`

Create a new execution environment.

**Request:**
```typescript
{
  mainModule: string;              // Entry point (e.g., "main.ts")
  modules: Record<string, string>; // Filename → source code
  permissions?: {
    allowNet?: boolean | string[];    // Network access
    allowRead?: boolean | string[];   // Filesystem read
    allowWrite?: boolean | string[];  // Filesystem write
    allowEnv?: boolean | string[];    // Environment variables
    allowRun?: boolean | string[];    // Subprocess execution
    allowFfi?: boolean;               // Foreign function interface
    allowHrtime?: boolean;            // High-resolution time
  };
  ttlSeconds?: number;             // Auto-cleanup (default: 3600)
}
```

**Response:**
```typescript
{
  id: string;                      // Environment UUID
  volumeName: string;              // Docker volume name
  mainModule: string;
  createdAt: string;               // ISO 8601 timestamp
  status: "ready" | "error";
  executionCount: number;          // Initially 0
  ttlSeconds: number;
}
```

#### `POST /environments/:id/execute`

Execute code in an existing environment.

**Request:**
```typescript
{
  data?: any;                      // Input data for handler
  env?: Record<string, string>;    // Environment variables
  limits?: {
    timeoutMs?: number;            // Default: 5000
    memoryMb?: number;             // Default: 128
  };
}
```

**Response:**
```typescript
{
  id: string;                      // Execution UUID
  exitCode: number;                // 0 = success, >0 = error
  stdout: string;                  // Handler result (JSON)
  stderr: string;                  // Error messages
  durationMs: number;              // Execution time
}
```

#### `DELETE /environments/:id`

Delete an environment and its volume.

**Response:** `204 No Content`

#### `GET /environments`

List all environments.

**Response:**
```typescript
[
  {
    id: string;
    volumeName: string;
    mainModule: string;
    createdAt: string;
    lastExecutedAt?: string;
    executionCount: number;
    status: string;
    ttlSeconds: number;
  }
]
```

## Database Schema

### `environments` Table

```sql
CREATE TABLE environments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volume_name       VARCHAR(255) NOT NULL UNIQUE,
    main_module       VARCHAR(255) NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    last_executed_at  TIMESTAMP,
    execution_count   INTEGER NOT NULL DEFAULT 0,
    status            VARCHAR(50) NOT NULL DEFAULT 'ready',
    metadata          JSONB,
    ttl_seconds       INTEGER DEFAULT 3600,

    INDEX idx_created_at (created_at),
    INDEX idx_last_executed_at (last_executed_at),
    INDEX idx_status (status)
);
```

### `executions` Table

```sql
CREATE TABLE executions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    started_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMP,
    exit_code       INTEGER,
    stdout          TEXT,
    stderr          TEXT,
    duration_ms     INTEGER,

    INDEX idx_environment_id (environment_id),
    INDEX idx_started_at (started_at)
);
```

## Implementation Pseudocode

### Setup Environment

```go
func setupEnvironment(req SetupRequest) (Environment, error) {
    // 1. Generate IDs
    envID := uuid.New()
    volumeName := "tee-env-" + envID.String()

    // 2. Create Docker volume
    exec("docker", "volume", "create", volumeName)

    // 3. Write each module to volume using helper container
    for filename, content in req.Modules {
        exec("docker", "run", "--rm",
            "-v", volumeName + ":/workspace",
            "alpine:latest",
            "sh", "-c", "cat > /workspace/" + filename + " <<EOF\n" + content + "\nEOF"
        )
    }

    // 4. Store in database
    db.Insert("environments", {
        id: envID,
        volume_name: volumeName,
        main_module: req.MainModule,
        metadata: json(req.Permissions),
        ttl_seconds: req.TTLSeconds || 3600,
        status: "ready"
    })

    // 5. Return environment
    return Environment{id: envID, volumeName: volumeName, ...}
}
```

### Execute in Environment

```go
func executeInEnvironment(envID UUID, req ExecuteRequest) (ExecutionResponse, error) {
    // 1. Look up environment
    env := db.Query("SELECT volume_name, main_module FROM environments WHERE id = ?", envID)

    // 2. Build execution input
    execID := uuid.New()
    input := {
        "event": {
            "data": req.Data,
            "env": req.Env
        },
        "context": {
            "executionId": execID,
            "environmentId": envID,
            "requestId": execID
        },
        "mainModule": env.MainModule
    }
    inputJSON := json.Marshal(input)

    // 3. Spawn container with stdin
    cmd := exec.Command("docker", "run",
        "--rm",                              // Remove after exit
        "-i",                                // Interactive (stdin)
        "--runtime=runsc",                   // gVisor isolation
        "--network=none",                    // No network
        "--read-only",                       // Read-only root
        "--memory=" + req.Limits.MemoryMb + "m",
        "--cpus=0.5",
        "-v", env.VolumeName + ":/workspace:ro",
        "deno-runtime:latest"
    )

    cmd.Stdin = bytes.NewReader(inputJSON)  // Pipe JSON to stdin

    // 4. Execute with timeout
    ctx, cancel := context.WithTimeout(req.Limits.TimeoutMs)
    defer cancel()

    stdout, stderr := cmd.Run(ctx)

    // 5. Parse output
    var output struct {
        Success bool
        Result  any
        Error   string
    }
    json.Unmarshal(stdout, &output)

    // 6. Store execution record
    db.Insert("executions", {
        id: execID,
        environment_id: envID,
        exit_code: cmd.ExitCode,
        stdout: stdout,
        stderr: stderr,
        duration_ms: elapsed
    })

    // 7. Update stats
    db.Exec("UPDATE environments SET execution_count = execution_count + 1, last_executed_at = NOW() WHERE id = ?", envID)

    return ExecutionResponse{...}
}
```

### Runtime Wrapper (Deno/TypeScript)

```typescript
// /runtime/runner.ts - Runs inside container

async function main() {
    // 1. Read stdin
    const stdinData = await readStdin()
    const input = JSON.parse(stdinData)

    // 2. Set environment variables
    for (const [key, value] of Object.entries(input.event.env)) {
        Deno.env.set(key, value)
    }

    // 3. Load user module
    const module = await import("/workspace/" + input.mainModule)

    if (typeof module.handler !== "function") {
        throw Error("Module must export 'handler' function")
    }

    // 4. Call handler
    const result = await module.handler(input.event, input.context)

    // 5. Write result to stdout
    console.log(JSON.stringify({
        success: true,
        result: result
    }))
}

main().catch(error => {
    console.log(JSON.stringify({
        success: false,
        error: error.message,
        stack: error.stack
    }))
    Deno.exit(1)
})
```

### Background Reaper

```go
func environmentReaper() {
    ticker := time.NewTicker(5 * time.Minute)

    for range ticker.C {
        // Find expired environments
        envs := db.Query(`
            SELECT id, volume_name
            FROM environments
            WHERE created_at + (ttl_seconds || ' seconds')::interval < NOW()
        `)

        for env in envs {
            // Remove volume
            exec("docker", "volume", "rm", "-f", env.VolumeName)

            // Delete from database (cascades to executions)
            db.Delete("environments", env.ID)
        }
    }
}
```

### Boot Reconciliation

```go
func reconcileEnvironments() {
    // Get all Docker volumes
    dockerVolumes := exec("docker", "volume", "ls", "--format", "{{.Name}}")
    volumeSet := parseLines(dockerVolumes)

    // Get all environments from DB
    dbEnvs := db.Query("SELECT id, volume_name FROM environments")

    for env in dbEnvs {
        // If volume doesn't exist, delete environment
        if !volumeSet.has(env.VolumeName) {
            db.Delete("environments", env.ID)
        }
    }
}
```

## User Code Pattern

Users write code following the Lambda handler pattern:

```typescript
// main.ts - User's entry point

export async function handler(event: any, context: any) {
    // event.data = input data from execution request
    // event.env = environment variables (also set in Deno.env)
    // context.executionId = unique execution ID
    // context.environmentId = environment ID
    // context.requestId = request ID

    // Can import other modules
    const { add } = await import("./utils.ts");

    // Process data
    const result = add(event.data.a, event.data.b);

    // Return result (will be JSON serialized)
    return {
        sum: result,
        timestamp: new Date().toISOString(),
        executionId: context.executionId
    };
}
```

```typescript
// utils.ts - User's utility module

export function add(a: number, b: number): number {
    return a + b;
}
```

## Security Model

### Isolation Layers

1. **gVisor Runtime** - Hardware-virtualized kernel, prevents kernel exploits
2. **Docker Namespaces** - Process, network, mount isolation
3. **Read-only Filesystem** - Container root is read-only
4. **Network Isolation** - `--network=none` by default
5. **Resource Limits** - Memory, CPU, PID limits enforced
6. **Deno Permissions** - Fine-grained file/network/subprocess control

### Permission Model

Permissions are specified during setup and enforced by Deno at runtime:

```typescript
{
  "allowNet": ["api.github.com:443"],  // Whitelist specific hosts
  "allowRead": ["/workspace"],         // Only read from workspace
  "allowWrite": false,                 // No write access
  "allowEnv": ["DEBUG"],               // Only specific env vars
  "allowRun": false,                   // No subprocess execution
  "allowFfi": false,                   // No native libraries
  "allowHrtime": false                 // No high-res timers
}
```

### Default Security Posture

- **Network:** Disabled (`--network=none`)
- **Filesystem:** Read-only except `/tmp`
- **Subprocesses:** Disabled
- **FFI:** Disabled
- **Memory:** 128MB limit
- **CPU:** 0.5 core limit
- **Timeout:** 5 second limit

## Runtime Support

- **Language:** TypeScript/JavaScript
- **Runtime:** Deno (latest stable)
- **Dependency Management:**
  - `npm:` specifiers for npm packages
  - `jsr:` specifiers for Deno packages
  - `https://` imports for ESM modules

## Future Enhancements

### Dependency Pre-installation

```typescript
// Setup request includes dependencies
{
  "mainModule": "main.ts",
  "modules": { ... },
  "dependencies": {
    "lodash": "npm:lodash@4.17.21",
    "postgres": "jsr:@std/postgres@0.17.0"
  }
}

// Setup phase runs: deno cache --reload
// Subsequent executions reuse cached deps
```

### Warm Container Pools

Instead of spawning fresh containers, maintain a pool of warm containers:
- Faster execution (<10ms vs ~100ms)
- Trade-off: higher memory usage
- Useful for high-frequency executions

### Execution Streaming

Stream stdout/stderr in real-time for long-running executions:

```typescript
// WebSocket endpoint
ws://api/environments/:id/execute/stream
```

### Multi-language Support

Add runtimes for Python, Go, Rust:

```typescript
{
  "runtime": "python:3.11",
  "mainModule": "main.py",
  "modules": {
    "main.py": "def handler(event, context): ..."
  }
}
```

### Snapshots and Cloning

Clone environments for testing:

```
POST /environments/:id/clone
→ Creates new environment with same modules
```

## Deployment

### Docker Compose

```yaml
services:
  postgres:
    image: postgres:16-alpine
    volumes:
      - postgres-data:/var/lib/postgresql/data

  tee-api:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - postgres
```

### System Requirements

- **OS:** Linux (for gVisor support)
- **Docker:** 24.0+ with gVisor runtime installed
- **CPU:** 2+ cores recommended
- **Memory:** 4GB+ (depends on concurrent executions)
- **Storage:** SSD recommended for volume performance

### Installing gVisor

```bash
# Install runsc runtime
curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list
sudo apt-get update && sudo apt-get install -y runsc

# Configure Docker to use gVisor
sudo runsc install
sudo systemctl restart docker
```

## Performance Characteristics

| Operation | Latency | Notes |
|-----------|---------|-------|
| Environment setup | 500ms - 2s | One-time cost |
| First execution | 100-200ms | Container cold start |
| Subsequent executions | 100-200ms | Fresh container each time |
| Volume I/O | <10ms | Reading modules from volume |
| Database operations | <5ms | Metadata lookups |

## Monitoring and Observability

### Metrics to Track

- Environment creation rate
- Execution count per environment
- Average execution duration
- Error rate
- Resource utilization (CPU, memory)
- Active environments count
- Reaped environments count

### Logging

- All executions logged to database
- Structured logs for API requests
- Docker container logs for debugging
- Audit trail for security events

## Conclusion

This design provides a secure, performant, and scalable execution environment for AI-generated code. The two-phase model optimizes for both cold starts (setup) and warm executions (execute), while the Lambda-style handler pattern provides a familiar and ergonomic developer experience.
