# Assist TEE - Trusted Execution Environment

A secure, sandboxed code execution service for AI agents. Execute TypeScript/JavaScript code in isolated gVisor containers with Lambda-style handlers.

## Features

- **Lambda-style handlers** - Familiar `export async function handler(event, context)` pattern
- **Two-phase execution** - Setup once, execute many times for optimal performance
- **Strong isolation** - gVisor runtime provides VM-level security
- **Environment reuse** - Prepared environments stored in Docker volumes
- **Automatic cleanup** - TTL-based garbage collection of unused environments
- **Execution tracking** - All executions logged to PostgreSQL

## Project Structure

```
assist-tee/
├── services/
│   ├── api/                    # Go API service
│   │   ├── cmd/api/            # Main entry point
│   │   ├── internal/           # Internal packages
│   │   │   ├── database/       # PostgreSQL connection
│   │   │   ├── models/         # Data structures
│   │   │   ├── executor/       # Docker operations
│   │   │   ├── handlers/       # HTTP handlers
│   │   │   └── reaper/         # Background cleanup
│   │   ├── Dockerfile          # API service container
│   │   ├── go.mod              # Go dependencies
│   │   └── go.sum
│   └── runtime/                # Deno runtime service
│       ├── runner.ts           # Stdin wrapper that calls user handlers
│       └── Dockerfile          # Runtime container
├── docs/                       # Documentation
│   ├── design.md               # Architecture documentation
│   ├── GVISOR.md               # gVisor security configuration
│   ├── BUILD.md                # Build and compilation guide
│   └── TYPESCRIPT_EXAMPLES.md  # TypeScript usage examples
├── examples/                   # Example user code
│   ├── main.ts
│   └── utils.ts
├── scripts/
│   └── test-full-flow.sh      # End-to-end test
├── docker-compose.yml          # Production (gVisor enabled)
├── docker-compose.dev.yml      # Development (gVisor disabled)
├── Makefile                    # Build and run commands
├── README.md                   # This file
├── QUICKSTART.md              # 5-minute getting started
└── .gitignore
```

## Quick Start

### Prerequisites

- Docker 24.0+ with Docker Compose
- gVisor runtime (runsc) installed
- Linux OS (for gVisor support)

### Install gVisor

```bash
# Install runsc runtime
curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list
sudo apt-get update && sudo apt-get install -y runsc

# Configure Docker to use gVisor
sudo runsc install
sudo systemctl restart docker

# Verify installation
docker run --rm --runtime=runsc hello-world
```

### Build and Run

```bash
# Build all service images
make build

# Start all services (API + PostgreSQL)
make run

# Check logs
docker-compose logs -f tee-api

# Verify it's running
curl http://localhost:8080/health
```

## API Usage

### 1. Setup an Environment

Create a new execution environment with your code:

```bash
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  const { a, b } = event.data;\n  return { sum: a + b };\n}",
      "utils.ts": "export const add = (a, b) => a + b;"
    },
    "ttlSeconds": 3600
  }'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "volumeName": "tee-env-550e8400-...",
  "mainModule": "main.ts",
  "status": "ready",
  "createdAt": "2025-01-15T10:30:00Z",
  "executionCount": 0,
  "ttlSeconds": 3600
}
```

### 2. Execute Code

Run your code multiple times in the same environment:

```bash
ENV_ID="550e8400-e29b-41d4-a716-446655440000"

curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "a": 5, "b": 3 },
    "env": { "DEBUG": "true" },
    "limits": {
      "timeoutMs": 5000,
      "memoryMb": 128
    }
  }'
```

Response:
```json
{
  "id": "exec-abc123...",
  "exitCode": 0,
  "stdout": "{\"sum\":8}",
  "stderr": "",
  "durationMs": 127
}
```

### 3. List Environments

```bash
curl http://localhost:8080/environments
```

### 4. Delete an Environment

```bash
curl -X DELETE http://localhost:8080/environments/$ENV_ID
```

## Writing User Code

Your code must export a `handler` function:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  // event.data = input data from execution request
  // event.env = environment variables
  // context.executionId = unique execution ID
  // context.environmentId = environment ID

  // Import other modules
  const { add } = await import("./utils.ts");

  // Process data
  const result = add(event.data.a, event.data.b);

  // Return result (auto JSON serialized)
  return {
    sum: result,
    timestamp: new Date().toISOString()
  };
}
```

```typescript
// utils.ts
export function add(a: number, b: number): number {
  return a + b;
}
```

## Testing with Example Code

```bash
# Run the full test flow
./scripts/test-full-flow.sh
```

This will:
1. ✓ Check API health
2. ✓ Create an execution environment
3. ✓ Run code twice (reusing the environment)
4. ✓ List environments
5. ✓ Clean up

## Architecture

```
Client → TEE API (Go) → PostgreSQL (metadata)
                     → Docker Volumes (code storage)
                     → gVisor Containers (execution)
```

### Execution Flow

1. **Setup**: Create volume, write modules, store metadata (~500ms-2s)
2. **Execute**: Spawn container, pipe JSON to stdin, run handler, capture stdout (~100-200ms)
3. **Cleanup**: Automatic reaping after TTL expires

## Service Management

### Build Individual Services

```bash
# Build API service
cd services/api && docker build -t tee-api:latest .

# Build runtime service
cd services/runtime && docker build -t deno-runtime:latest .

# Or use Makefile
make build-api
make build-runtime
```

### Run Individual Services

The services are designed to run together via docker-compose, but you can run them individually:

```bash
# Start PostgreSQL first
docker run -d --name postgres \
  -e POSTGRES_USER=tee \
  -e POSTGRES_PASSWORD=tee \
  -e POSTGRES_DB=tee \
  -p 5432:5432 \
  postgres:16-alpine

# Run API service
docker run -d --name tee-api \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e DB_HOST=postgres \
  tee-api:latest

# Runtime service runs on-demand (spawned by API)
```

## Security

- **gVisor Runtime**: Hardware virtualization for kernel isolation
- **Network Isolation**: `--network=none` by default
- **Read-only Filesystem**: Container root is read-only
- **Resource Limits**: Memory, CPU, and timeout limits enforced
- **Permissions**: Fine-grained Deno permissions (future enhancement)

## Configuration

Environment variables for the API service:

- `DB_HOST` - PostgreSQL host (default: postgres)
- `DB_PORT` - PostgreSQL port (default: 5432)
- `DB_USER` - PostgreSQL user (default: tee)
- `DB_PASSWORD` - PostgreSQL password (default: tee)
- `DB_NAME` - PostgreSQL database (default: tee)
- `LOG_LEVEL` - Log level (default: debug)
- `DISABLE_GVISOR` - Set to `true` or `1` to disable gVisor (⚠️ DEV ONLY!)

### Disabling gVisor (Development Mode)

For development on macOS/Windows where gVisor is not available:

**Option 1: Use dev compose file** (Recommended)
```bash
make run-dev
# or
docker-compose -f docker-compose.dev.yml up -d
```

**Option 2: Set environment variable**
```yaml
# docker-compose.yml
environment:
  - DISABLE_GVISOR=true
```

**Option 3: Set when running**
```bash
DISABLE_GVISOR=true go run cmd/api/main.go
```

⚠️ **WARNING:** When gVisor is disabled:
- Code is NOT sandboxed with hardware virtualization
- User code can potentially access the host kernel
- This mode should ONLY be used for local development
- DO NOT USE IN PRODUCTION!

The service will display prominent warnings on startup when gVisor is disabled.

## Development

```bash
# Install Go dependencies (from services/api/)
cd services/api && go mod download

# Run locally (requires PostgreSQL)
cd services/api && go run cmd/api/main.go

# Build
cd services/api && go build -o tee-api ./cmd/api

# Run tests
cd services/api && go test ./...
```

## Monitoring

### View active environments

```bash
curl http://localhost:8080/environments | jq
```

### Check Docker volumes

```bash
docker volume ls | grep tee-env
```

### View execution logs

```bash
# API logs
docker-compose logs -f tee-api

# Database
docker exec -it tee-postgres psql -U tee -d tee
SELECT * FROM environments ORDER BY created_at DESC LIMIT 10;
SELECT * FROM executions ORDER BY started_at DESC LIMIT 10;
```

## Performance

| Operation | Latency | Notes |
|-----------|---------|-------|
| Environment setup | 500ms - 2s | One-time cost |
| Execution | 100-200ms | Per execution |
| Volume I/O | <10ms | Reading modules |
| DB operations | <5ms | Metadata lookups |

## Troubleshooting

### gVisor not working

```bash
# Check if runsc is installed
which runsc

# Verify Docker can use runsc
docker run --rm --runtime=runsc hello-world

# Check Docker daemon config
cat /etc/docker/daemon.json
```

### Container spawning fails

```bash
# Check Docker socket is mounted
docker exec tee-api ls -la /var/run/docker.sock

# Check permissions
docker exec tee-api docker ps
```

### Database connection issues

```bash
# Check PostgreSQL is running
docker-compose ps postgres

# Test connection
docker exec tee-api nc -zv postgres 5432
```

## Future Enhancements

- [ ] Dependency pre-installation in setup phase
- [ ] Warm container pools for faster execution
- [ ] Multi-language support (Python, Go, Rust)
- [ ] WebSocket streaming for long-running executions
- [ ] Metrics and observability dashboards
- [ ] Fine-grained Deno permissions enforcement
- [ ] Environment snapshots and cloning

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
