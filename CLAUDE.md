# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Assist TEE is a trusted execution environment for running AI-generated code in secure gVisor-sandboxed containers. It uses a two-phase execution model: setup (create environment with code) then execute (run handler function multiple times).

## Build & Run Commands

```bash
# Build all Docker images
make build

# Build individual images
make build-api           # Go API service
make build-runtime-deno  # Deno runtime container
make build-runtime-bun   # Bun runtime container

# Run with gVisor (Linux production)
make run

# Run without gVisor (macOS/Windows development)
make run-dev

# Stop services
make stop

# View logs
make logs
make logs-api

# Run Go tests
cd services/api && go test ./...
```

## Testing

```bash
# Quick functionality test
./scripts/test-full-flow.sh

# All security tests
./scripts/test-all-security.sh

# Individual test suites
./scripts/test-network-sandbox.sh
./scripts/test-filesystem-sandbox.sh
./scripts/test-permissions.sh
./scripts/test-dependencies.sh

# Manual testing
make test-setup                           # Create test environment (Deno)
make test-setup-bun                       # Create test environment (Bun)
make test-execute ENV_ID=<uuid>          # Execute in environment
make list                                 # List environments
```

## Architecture

```
services/
├── api/                      # Go API service (gorilla/mux)
│   ├── cmd/api/main.go       # Entry point, router setup
│   └── internal/
│       ├── handlers/         # HTTP handlers (setup, execute, delete)
│       ├── executor/         # Docker operations (docker.go)
│       ├── database/         # PostgreSQL connection & schema
│       ├── models/           # Data structures (SetupRequest, Environment, etc.)
│       ├── reaper/           # TTL-based environment cleanup
│       └── logger/           # Structured logging
├── runtime/                  # Deno container that runs user code
│   ├── runner.ts             # Reads stdin JSON, calls handler, writes stdout
│   └── Dockerfile
└── runtime-bun/              # Bun container (alternative runtime)
    ├── runner.ts
    └── Dockerfile
```

### Execution Flow

1. **Setup Phase** (`POST /environments/setup`):
   - Creates Docker volume `tee-env-{uuid}`
   - Writes user modules to volume via busybox container
   - Installs dependencies with network access (`deno cache`)
   - Stores metadata in PostgreSQL

2. **Execute Phase** (`POST /environments/:id/execute`):
   - Spawns gVisor container with volume mounted read-only
   - Pipes JSON to stdin: `{event, context, mainModule}`
   - runner.ts calls user's `handler(event, context)`
   - Captures stdout as JSON result

### Key Files

- `services/api/internal/executor/docker.go` - Core Docker/gVisor execution logic
- `services/api/internal/database/db.go` - Schema and connection management
- `services/runtime/runner.ts` - Container entrypoint that calls user handlers

## Environment Variables

- `RUNTIME_IMAGE_DENO` - Deno runtime image (default: `octaviusdeployment/assist-tee-rt-deno:latest`)
- `RUNTIME_IMAGE_BUN` - Bun runtime image (default: `octaviusdeployment/assist-tee-rt-bun:latest`)
- `RUNTIME_IMAGE` - Legacy: fallback for Deno if `RUNTIME_IMAGE_DENO` not set
- `DISABLE_GVISOR=true` - Skip gVisor runtime (dev only)
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` - PostgreSQL connection
- `LOG_LEVEL` - Logging verbosity (default: debug)

## Runtime Selection

Specify runtime in setup request (defaults to "deno"):
```json
{
  "mainModule": "main.ts",
  "runtime": "bun",
  "modules": { ... }
}
```

## Handler Pattern

User code exports a Lambda-style handler:

```typescript
export async function handler(event: any, context: any) {
  // event.data = input from execute request
  // event.env = environment variables
  // context.executionId, context.environmentId
  return { result: "value" };
}
```

## Security Model

Execution containers run with:
- gVisor runtime (`--runtime=runsc`)
- No network (`--network=none`)
- Read-only filesystem (`--read-only`)
- Resource limits (128MB RAM, 0.5 CPU, 5s timeout)
- Volume mounted read-only at `/workspace`
