# Quick Start Guide

Get the TEE running in 5 minutes.

## Prerequisites Check

```bash
# Check Docker
docker --version
# Should be 24.0+

# Check Docker Compose
docker-compose --version
# Should be 2.0+
```

## Step 1: Install gVisor

**Note:** gVisor only works on Linux. If you're on macOS/Windows, you'll need to use a Linux VM or skip gVisor (but you'll lose the security isolation).

### On Linux (Ubuntu/Debian):

```bash
# Install gVisor
curl -fsSL https://gvisor.dev/archive.key | sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" | sudo tee /etc/apt/sources.list.d/gvisor.list
sudo apt-get update && sudo apt-get install -y runsc

# Configure Docker
sudo runsc install
sudo systemctl restart docker

# Test it
docker run --rm --runtime=runsc hello-world
```

### On macOS/Windows (Development Only):

For development without gVisor security, use the dev compose file:

```bash
# Use the dev mode (gVisor disabled)
make run-dev

# Or manually
docker-compose -f docker-compose.dev.yml up -d
```

⚠️ **Warning:** This removes the security isolation. Only use for development!

**Alternatively**, you can disable gVisor by setting an environment variable in `docker-compose.yml`:
```yaml
environment:
  - DISABLE_GVISOR=true  # Add this line
```

## Step 2: Build and Run

```bash
# Build all service images
make build

# Start all services (PostgreSQL + TEE API)
make run

# Check it's working
curl http://localhost:8080/health
# Should return: OK
```

## Step 3: Test It

### Quick Test

```bash
# Run basic functionality test
./scripts/test-full-flow.sh
```

This will:
1. ✓ Check API health
2. ✓ Create an execution environment
3. ✓ Run code twice (reusing the environment)
4. ✓ List environments
5. ✓ Clean up

### Complete Security Test (Optional)

```bash
# Run all security tests (network, filesystem, permissions, dependencies)
./scripts/test-all-security.sh
```

This runs 5 test suites demonstrating:
- Network isolation (HTTP, DNS, WebSocket blocked)
- Filesystem sandboxing (read/write restrictions)
- Deno permissions (all restricted)
- Dependency handling (local vs remote)

See [docs/TESTING.md](docs/TESTING.md) for details.

## Step 4: Create Your Own Environment

```bash
# 1. Setup
ENV_RESPONSE=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) { return { message: \"Hello from TEE!\", data: event.data }; }"
    }
  }')

# Get the environment ID
ENV_ID=$(echo $ENV_RESPONSE | jq -r '.id')
echo "Environment ID: $ENV_ID"

# 2. Execute
curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "name": "World" }
  }' | jq

# 3. Execute again (fast!)
curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "name": "TEE" }
  }' | jq
```

## Common Commands

```bash
# View logs
make logs

# List all environments
make list

# Stop everything
make stop

# Stop and remove all data
make clean

# Restart
make stop && make run
```

## Build Individual Services

Each service is in its own folder with its own Dockerfile:

```bash
# Build API service
cd services/api && docker build -t tee-api:latest .

# Build runtime service
cd services/runtime && docker build -t deno-runtime:latest .

# Or use Makefile shortcuts
make build-api
make build-runtime
```

## What's Next?

- **TypeScript Examples**: See [docs/TYPESCRIPT_EXAMPLES.md](docs/TYPESCRIPT_EXAMPLES.md) for detailed code examples
- **Architecture**: Read [docs/design.md](docs/design.md) for architecture details
- **Security**: Check [docs/GVISOR.md](docs/GVISOR.md) for gVisor configuration
- **Build Details**: See [docs/BUILD.md](docs/BUILD.md) for compilation info
- **API Reference**: Read the full [README.md](./README.md) for API documentation

## Troubleshooting

### "gVisor not found"
- You're not on Linux, or runsc isn't installed
- Solution: Install gVisor (Linux only) or remove `--runtime=runsc` for dev

### "Cannot connect to Docker daemon"
- Docker isn't running
- Solution: `sudo systemctl start docker` (Linux) or start Docker Desktop (Mac/Windows)

### "Connection refused to postgres"
- PostgreSQL container isn't ready yet
- Solution: Wait 10 seconds and try again, or check logs with `make logs`

### Execution fails with "module not found"
- The mainModule doesn't match a key in modules
- Solution: Ensure `"mainModule": "main.ts"` matches a module name in the modules object

## Need Help?

1. Check logs: `make logs`
2. Verify services: `docker-compose ps`
3. Check database: `docker exec -it tee-postgres psql -U tee -d tee -c 'SELECT * FROM environments;'`
4. Check Docker volumes: `docker volume ls | grep tee-env`

Happy hacking! 🚀
