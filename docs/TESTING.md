# Testing Guide

Comprehensive testing suite for the TEE system demonstrating security sandboxing and functionality.

## Quick Start

Run all tests:
```bash
./scripts/test-all-security.sh
```

This runs all 5 test suites in sequence and verifies complete security isolation.

## Test Suites

### 1. Basic Functionality Test

**Script:** `./scripts/test-full-flow.sh`

**Purpose:** Verifies core TEE functionality works correctly.

**Tests:**
- API health check
- Environment setup
- Code execution (multiple times in same environment)
- Environment listing
- Environment cleanup

**Expected Result:** All operations succeed, demonstrating the two-phase execution model.

**Run:**
```bash
./scripts/test-full-flow.sh
```

---

### 2. Network Sandboxing Test

**Script:** `./scripts/test-network-sandbox.sh`

**Purpose:** Demonstrates that code execution is completely isolated from network access.

**Tests:**
1. **HTTP Requests** - Attempts to fetch from Google
2. **DNS Resolution** - Attempts to resolve domain names
3. **WebSocket Connections** - Attempts to open WebSocket

**Expected Result:** All network operations fail, proving `--network=none` is enforced.

**Run:**
```bash
./scripts/test-network-sandbox.sh
```

**Example Output:**
```
✓ Network access properly sandboxed
✓ DNS resolution properly sandboxed
✓ WebSocket properly sandboxed
```

---

### 3. Filesystem Sandboxing Test

**Script:** `./scripts/test-filesystem-sandbox.sh`

**Purpose:** Demonstrates filesystem access restrictions.

**Tests:**
1. **Workspace Files** - Can read files in /workspace (✓ should work)
2. **System Files** - Attempts to read /etc/passwd (✗ should fail)
3. **Write Outside Workspace** - Attempts to write to /tmp (✗ should fail)
4. **Command Execution** - Attempts to run shell commands (✗ should fail)
5. **Environment Variables** - Tests env var access control

**Expected Result:**
- Workspace files accessible
- System files blocked
- Write operations restricted
- Command execution blocked

**Run:**
```bash
./scripts/test-filesystem-sandbox.sh
```

**Example Output:**
```
✓ Workspace files readable
✓ System files protected
✓ Write operations restricted
✓ Command execution blocked
✓ Environment variables controlled
```

---

### 4. Deno Permissions Test

**Script:** `./scripts/test-permissions.sh`

**Purpose:** Tests all Deno permission flags are properly restricted.

**Tests:**
1. **--allow-net** - Network access
2. **--allow-read** - Filesystem read (outside workspace)
3. **--allow-write** - Filesystem write (outside workspace)
4. **--allow-run** - Subprocess execution
5. **--allow-ffi** - Foreign Function Interface (native libraries)
6. **--allow-hrtime** - High-resolution time (timing attacks)

**Expected Result:** All permissions blocked (except hrtime which may be allowed with degraded precision).

**Run:**
```bash
./scripts/test-permissions.sh
```

**Example Output:**
```
--allow-net:    Network access        ✓ Blocked
--allow-read:   File read access      ✓ Blocked
--allow-write:  File write access     ✓ Blocked
--allow-run:    Run subprocesses      ✓ Blocked
--allow-ffi:    Native libraries      ✓ Blocked
--allow-hrtime: High-res timing       ⚠️  Allowed
```

**Note on --allow-hrtime:** High-resolution timing may be available with reduced precision. This is generally acceptable but be aware it could enable timing-based side-channel attacks.

---

### 5. Dependency Handling Test

**Script:** `./scripts/test-dependencies.sh`

**Purpose:** Demonstrates how dependencies work without network access.

**Tests:**
1. **Local Module Imports** - Import from workspace (✓ should work)
2. **Remote URL Imports** - Import from https://deno.land (✗ should fail)
3. **NPM Package Imports** - Import npm packages (✗ should fail)
4. **Complex Dependency Trees** - Multiple local modules importing each other (✓ should work)

**Expected Result:**
- Local imports work
- Remote imports blocked (no network)
- NPM imports blocked (no network)
- Complex local dependency trees work

**Run:**
```bash
./scripts/test-dependencies.sh
```

**Example Output:**
```
✓ Local module imports work
✓ Remote URL imports blocked (no network)
✓ NPM package imports blocked (no network)
✓ Complex dependency trees work
```

---

## Running Individual Tests

Each test can be run independently:

```bash
# Test basic functionality
./scripts/test-full-flow.sh

# Test network sandboxing
./scripts/test-network-sandbox.sh

# Test filesystem sandboxing
./scripts/test-filesystem-sandbox.sh

# Test Deno permissions
./scripts/test-permissions.sh

# Test dependency handling
./scripts/test-dependencies.sh

# Run all tests
./scripts/test-all-security.sh
```

## Prerequisites

Before running tests, ensure:

1. **Services are running:**
   ```bash
   make run
   # or for dev mode without gVisor:
   make run-dev
   ```

2. **API is healthy:**
   ```bash
   curl http://localhost:8080/health
   # Should return: {"status":"ok"}
   ```

3. **Required tools installed:**
   - `curl` - HTTP requests
   - `jq` - JSON parsing
   - `bash` - Script execution

## Understanding Test Results

### Success Indicators

- `✓` Green checkmark - Test passed, security working as expected
- `✗` Red X - Test failed, security issue detected
- `⚠️` Yellow warning - Informational, may require attention

### What Tests Verify

1. **Security Boundaries**
   - Network isolation (--network=none)
   - Filesystem restrictions (read-only root, limited workspace access)
   - Permission enforcement (Deno security model)

2. **Functional Correctness**
   - Two-phase execution (setup + execute)
   - Environment reuse
   - Module system
   - Dependency handling

3. **Attack Surface**
   - No external network access
   - No system file access
   - No command execution
   - No native code loading
   - Limited timing precision

## Test Coverage

| Security Feature | Test Suite | Status |
|-----------------|------------|--------|
| Network isolation | Network Sandboxing | ✓ |
| Filesystem read restrictions | Filesystem Sandboxing | ✓ |
| Filesystem write restrictions | Filesystem Sandboxing | ✓ |
| Command execution blocking | Filesystem Sandboxing | ✓ |
| DNS resolution blocking | Network Sandboxing | ✓ |
| WebSocket blocking | Network Sandboxing | ✓ |
| HTTP/HTTPS blocking | Network Sandboxing | ✓ |
| Subprocess blocking | Permissions | ✓ |
| FFI blocking | Permissions | ✓ |
| Remote import blocking | Dependencies | ✓ |
| NPM import blocking | Dependencies | ✓ |
| Local module support | Dependencies | ✓ |
| Environment variable control | Filesystem Sandboxing | ✓ |

## Continuous Integration

These tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
name: Security Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build services
        run: make build

      - name: Start services
        run: make run

      - name: Wait for API
        run: |
          for i in {1..30}; do
            if curl -sf http://localhost:8080/health; then
              break
            fi
            sleep 1
          done

      - name: Run security tests
        run: ./scripts/test-all-security.sh
```

## Troubleshooting Tests

### "API is not running"

**Solution:** Start the services first:
```bash
make run
```

### "Connection refused"

**Solution:** Wait for services to fully start (usually 10-30 seconds):
```bash
docker-compose logs -f tee-api
```

### Tests pass but with warnings

**Common warnings:**
- High-resolution timing available (--allow-hrtime)
  - This is generally acceptable but reduces timing attack protection

### Tests fail unexpectedly

1. Check service logs:
   ```bash
   docker-compose logs tee-api
   ```

2. Verify gVisor status:
   ```bash
   docker info | grep -i runtime
   ```

3. Check Docker network:
   ```bash
   docker network ls
   docker run --rm --network=none alpine ping -c 1 google.com
   # Should fail
   ```

## Writing New Tests

Use the existing test scripts as templates. Key patterns:

```bash
# 1. Create environment
SETUP=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) { ... }"
    },
    "ttlSeconds": 300
  }')

# 2. Extract environment ID
ENV_ID=$(echo $SETUP | jq -r '.id')

# 3. Execute code
EXEC=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {...}}')

# 4. Parse result
RESULT=$(echo $EXEC | jq -r '.stdout')

# 5. Verify expectations
if [ "$(echo $RESULT | jq -r '.success')" = "true" ]; then
  echo "✓ Test passed"
else
  echo "✗ Test failed"
fi

# 6. Cleanup
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID
```

## Security Considerations

These tests verify:
- **Defense in depth** - Multiple layers (Docker, gVisor, Deno)
- **Fail-secure** - Tests expect operations to fail
- **Principle of least privilege** - Only workspace access granted
- **Isolation** - Each execution is independent

## Next Steps

After running tests:
1. Review test output for any warnings
2. Check [GVISOR.md](GVISOR.md) for security configuration
3. Read [TYPESCRIPT_EXAMPLES.md](TYPESCRIPT_EXAMPLES.md) for usage patterns
4. See [design.md](design.md) for architecture details
