```
 ░▒▓██████▓▒░  
░▒▓█▓▒░░▒▓█▓▒░ 
░▒▓█▓▒░░▒▓█▓▒░ 
░▒▓█▓▒░░▒▓█▓▒░  Octavius Labs
░▒▓█▓▒░░▒▓█▓▒░ 
░▒▓█▓▒░░▒▓█▓▒░ 
 ░▒▓██████▓▒░
```

# Assist TEE - Trusted Execution Environment

> **Note:** This project was built with the help of Claude Code.

## Why?

AI agents are increasingly capable of writing and executing code. But running
AI-generated code is inherently risky—the code might have bugs, access resources
it shouldn't, or behave unexpectedly. You can't just `eval()` it and hope for
the best.

**Assist TEE solves this problem** by providing a secure sandbox where AI agents
can execute code without risking your infrastructure. Every execution runs in a
hardware-isolated [gVisor](https://gvisor.dev/) container with:

- **No network access** - Code can't phone home or exfiltrate data
- **No filesystem access** - Can only read its own modules
- **Resource limits** - CPU, memory, and time constraints prevent runaway
  processes
- **Fresh containers** - Each execution starts clean, no state leakage between
  runs

The result: AI agents can write and run TypeScript/JavaScript code with the
confidence that even malicious code can't escape the sandbox.

## How It Works

Assist TEE uses a **two-phase execution model** inspired by AWS Lambda:

1. **Setup Phase** (~500ms-2s): Upload your code modules once, install
   dependencies with network access, then lock down the environment
2. **Execute Phase** (~100-200ms): Run your handler function many times against
   the prepared environment—fast, isolated, and stateless

```
┌─────────────┐     Setup      ┌─────────────────┐     Execute     ┌─────────────┐
│  AI Agent   │ ──────────────▶│  TEE API (Go)   │ ──────────────▶ │   gVisor    │
│             │   modules +     │                 │   event data    │  Container  │
│             │   dependencies  │   PostgreSQL    │                 │  (Deno)     │
└─────────────┘                └─────────────────┘                 └─────────────┘
```

Your code exports a simple `handler` function—just like Lambda:

```typescript
export async function handler(event: any, context: any) {
  const { a, b } = event.data;
  return { sum: a + b };
}
```

## Inspirations

- [Cloudflare's Code Mode](https://blog.cloudflare.com/code-mode/) - AI writes
  code to interact with MCP servers
- [Anthropic's Code Execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp) -
  Secure code execution for AI agents

## Features

- **Lambda-style handlers** - Familiar
  `export async function handler(event, context)` pattern
- **Two-phase execution** - Setup once, execute many times for optimal
  performance
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
│   ├── TYPESCRIPT_EXAMPLES.md  # TypeScript usage examples
│   ├── DEPENDENCIES.md         # External dependency management
│   ├── TESTING.md              # Testing guide
│   └── SECURITY.md             # Security model
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

### Option 1: Using Docker Hub (Recommended)

Pull the pre-built images and run:

```bash
# Pull images
docker pull octaviusdeployment/assist-tee-api:latest
docker pull octaviusdeployment/assist-tee-rt-deno:latest

# Start PostgreSQL
docker run -d --name tee-postgres \
  -e POSTGRES_USER=tee \
  -e POSTGRES_PASSWORD=tee \
  -e POSTGRES_DB=tee \
  -p 5432:5432 \
  postgres:16-alpine

# Start the API
docker run -d --name tee-api \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e DB_HOST=host.docker.internal \
  -e DB_USER=tee \
  -e DB_PASSWORD=tee \
  -e DB_NAME=tee \
  -e RUNTIME_IMAGE=octaviusdeployment/assist-tee-rt-deno:latest \
  -e DISABLE_GVISOR=true \
  octaviusdeployment/assist-tee-api:latest

# Verify it's running
curl http://localhost:8080/health
```

> **Note:** Set `DISABLE_GVISOR=true` on macOS/Windows. On Linux with gVisor installed, remove this flag for full sandboxing.

### Option 2: Build from Source

```bash
# Clone the repo
git clone https://github.com/your-org/assist-tee.git
cd assist-tee

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

**With external dependencies:**

```bash
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "import { format } from \"npm:date-fns@3\";\nexport async function handler(event, context) {\n  return { formatted: format(new Date(), \"PPP\") };\n}"
    },
    "dependencies": {
      "npm": ["date-fns@3"],
      "deno": ["https://deno.land/std@0.224.0/async/delay.ts"]
    },
    "ttlSeconds": 3600
  }'
```

Dependencies are downloaded during setup (with network) and cached for execution
(without network). See [docs/DEPENDENCIES.md](docs/DEPENDENCIES.md) for details.

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
    timestamp: new Date().toISOString(),
  };
}
```

```typescript
// utils.ts
export function add(a: number, b: number): number {
  return a + b;
}
```

## Testing

### Quick Test

```bash
# Run basic functionality test
./scripts/test-full-flow.sh
```

### Complete Security Test Suite

```bash
# Run all security and sandboxing tests
./scripts/test-all-security.sh
```

This runs 5 comprehensive test suites:

1. ✓ **Basic functionality** - Environment setup, execution, cleanup
2. ✓ **Network sandboxing** - HTTP, DNS, WebSocket blocking
3. ✓ **Filesystem sandboxing** - Read/write restrictions, command blocking
4. ✓ **Deno permissions** - All permission flags (net, read, write, run, ffi,
   hrtime)
5. ✓ **Dependency handling** - Local vs remote imports, npm packages

### Individual Test Suites

```bash
# Test network isolation
./scripts/test-network-sandbox.sh

# Test filesystem restrictions
./scripts/test-filesystem-sandbox.sh

# Test Deno permissions
./scripts/test-permissions.sh

# Test dependency handling
./scripts/test-dependencies.sh
```

See [docs/TESTING.md](docs/TESTING.md) for detailed testing documentation.

## Architecture

```
Client → TEE API (Go) → PostgreSQL (metadata)
                     → Docker Volumes (code storage)
                     → gVisor Containers (execution)
```

### Execution Flow

1. **Setup**: Create volume, write modules, store metadata (~500ms-2s)
2. **Execute**: Spawn container, pipe JSON to stdin, run handler, capture stdout
   (~100-200ms)
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

The services are designed to run together via docker-compose, but you can run
them individually:

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
- **Network Isolation**: `--network=none` by default (configurable via whitelist)
- **Read-only Filesystem**: Container root is read-only
- **Resource Limits**: Memory, CPU, and timeout limits enforced
- **Permission Whitelisting**: Fine-grained control over network access and environment variables

### Permission Whitelisting

Configure allowed network domains and environment variables during setup:

```bash
curl -X POST http://localhost:8080/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {"main.ts": "export async function handler(e,c) { return await fetch(\"https://api.example.com/data\").then(r => r.json()); }"},
    "permissions": {
      "allowNet": ["api.example.com", "cdn.example.com:443"],
      "allowEnv": ["API_KEY", "DEBUG"]
    }
  }'
```

- **allowNet**: List of domains the code can access (enables `--network=bridge` + Deno `--allow-net=...`)
- **allowEnv**: List of env var names that can be passed from execute requests to the container

See [docs/SECURITY.md](docs/SECURITY.md#permission-whitelisting) for details.

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

| Operation         | Latency    | Notes            |
| ----------------- | ---------- | ---------------- |
| Environment setup | 500ms - 2s | One-time cost    |
| Execution         | 100-200ms  | Per execution    |
| Volume I/O        | <10ms      | Reading modules  |
| DB operations     | <5ms       | Metadata lookups |

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

## Roadmap

- [x] Dependency pre-installation in setup phase
- [ ] Warm container pools for faster execution
- [ ] Multi-language support (Python, Go, Rust)
- [ ] WebSocket streaming for long-running executions
- [ ] Metrics and observability dashboards
- [ ] Fine-grained Deno permissions enforcement
- [ ] Environment snapshots and cloning

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file
for details.

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md)
and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a pull request.

## Security

If you discover a security vulnerability, please do NOT open a public issue.
Instead, email the maintainers directly. See
[CONTRIBUTING.md](CONTRIBUTING.md#security) for details.
