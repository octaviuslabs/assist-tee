# Test Scripts

This directory contains comprehensive test scripts for the TEE system.

## Test Scripts

### test-all-security.sh
**Master test suite** - Runs all tests in sequence.

```bash
./scripts/test-all-security.sh
```

Runs all 5 test suites and provides a comprehensive security verification.

---

### test-full-flow.sh
**Basic functionality test** - Verifies core TEE operations.

```bash
./scripts/test-full-flow.sh
```

**What it tests:**
- API health check
- Environment setup (creating volumes, writing modules)
- Code execution (multiple times in same environment)
- Environment listing
- Environment cleanup

**Expected outcome:** All operations succeed, demonstrating the two-phase execution model works correctly.

---

### test-network-sandbox.sh
**Network isolation test** - Proves network access is blocked.

```bash
./scripts/test-network-sandbox.sh
```

**What it tests:**
- HTTP/HTTPS requests (fetch to google.com)
- DNS resolution (URL parsing and resolution)
- WebSocket connections (wss:// protocol)

**Expected outcome:** All network operations fail with errors, proving `--network=none` is enforced.

**Why this matters:** Without network isolation, malicious code could:
- Exfiltrate data to external servers
- Download additional malware
- Participate in DDoS attacks
- Make unauthorized API calls

---

### test-filesystem-sandbox.sh
**Filesystem restrictions test** - Proves filesystem is sandboxed.

```bash
./scripts/test-filesystem-sandbox.sh
```

**What it tests:**
1. Reading workspace files (should work)
2. Reading system files like /etc/passwd (should fail)
3. Writing outside workspace to /tmp (should fail)
4. Executing shell commands (should fail)
5. Environment variable access (controlled)

**Expected outcome:**
- ✓ Workspace files accessible
- ✗ System files blocked
- ✗ Write operations outside workspace blocked
- ✗ Command execution blocked
- ✓ Environment variables controlled

**Why this matters:** Without filesystem restrictions, malicious code could:
- Read sensitive system files (SSH keys, passwords)
- Modify system configurations
- Plant backdoors
- Execute arbitrary commands

---

### test-permissions.sh
**Deno permissions test** - Verifies all Deno permission flags are restricted.

```bash
./scripts/test-permissions.sh
```

**What it tests:**
- `--allow-net` - Network access
- `--allow-read` - File read (outside workspace)
- `--allow-write` - File write (outside workspace)
- `--allow-run` - Subprocess execution
- `--allow-ffi` - Foreign Function Interface (loading native libraries)
- `--allow-hrtime` - High-resolution time (timing attacks)

**Expected outcome:** All permissions blocked (hrtime may be allowed with reduced precision).

**Why this matters:** Deno's permission system provides defense-in-depth:
- Even if Docker isolation fails, Deno blocks dangerous operations
- Multiple layers of security
- Principle of least privilege

---

### test-dependencies.sh
**Dependency handling test** - Shows how dependencies work without network.

```bash
./scripts/test-dependencies.sh
```

**What it tests:**
1. Local module imports with `./` paths (should work)
2. Remote URL imports from https://deno.land (should fail - no network)
3. NPM package imports with `npm:` specifier (should fail - no network)
4. Complex local dependency trees (should work)

**Expected outcome:**
- ✓ Local imports work
- ✗ Remote imports blocked
- ✗ NPM imports blocked
- ✓ Complex local trees work

**How to use dependencies:** Bundle all dependencies in the setup phase as separate modules.

**Why this matters:** Understanding dependency limitations helps you:
- Structure code properly (bundle dependencies)
- Avoid runtime import failures
- Plan for vendoring/bundling requirements

---

## Running Tests

### Prerequisites

1. Start the services:
   ```bash
   make run
   # or for dev mode:
   make run-dev
   ```

2. Verify API is running:
   ```bash
   curl http://localhost:8080/health
   ```

### Run All Tests

```bash
./scripts/test-all-security.sh
```

### Run Individual Tests

```bash
# Basic functionality
./scripts/test-full-flow.sh

# Network sandboxing
./scripts/test-network-sandbox.sh

# Filesystem sandboxing
./scripts/test-filesystem-sandbox.sh

# Deno permissions
./scripts/test-permissions.sh

# Dependency handling
./scripts/test-dependencies.sh
```

## Understanding Results

### Success Indicators

- `✓` Green - Test passed, behavior is correct
- `✗` Red - Test failed, security issue detected
- `⚠️` Yellow - Warning, requires attention

### What "Should Fail" Means

Many tests verify that dangerous operations **fail**. This is correct behavior:

```
✓ Network access properly blocked
```

This means the test tried to access the network (malicious behavior) and was correctly blocked (security working).

### Test Output

Each test provides:
1. Colored status messages
2. JSON output showing what happened
3. Summary of all checks
4. Explanation of what was verified

Example:
```
Result:
{
  "success": false,
  "message": "✓ Network access properly blocked",
  "error": "TypeError: error sending request..."
}

✓ Network access properly sandboxed
```

## CI/CD Integration

These tests can be run in CI pipelines:

```yaml
# GitHub Actions example
- name: Run security tests
  run: |
    make run &
    sleep 10
    ./scripts/test-all-security.sh
```

## Test Coverage Summary

| Security Feature | Test Script | Verified |
|-----------------|-------------|----------|
| Network isolation | network-sandbox | ✓ |
| DNS blocking | network-sandbox | ✓ |
| WebSocket blocking | network-sandbox | ✓ |
| System file read | filesystem-sandbox | ✓ |
| Arbitrary write | filesystem-sandbox | ✓ |
| Command execution | filesystem-sandbox | ✓ |
| Environment variables | filesystem-sandbox | ✓ |
| Deno --allow-net | permissions | ✓ |
| Deno --allow-read | permissions | ✓ |
| Deno --allow-write | permissions | ✓ |
| Deno --allow-run | permissions | ✓ |
| Deno --allow-ffi | permissions | ✓ |
| Deno --allow-hrtime | permissions | ✓ |
| Local imports | dependencies | ✓ |
| Remote imports | dependencies | ✓ |
| NPM imports | dependencies | ✓ |
| Dependency trees | dependencies | ✓ |
| Basic functionality | full-flow | ✓ |

## Troubleshooting

### "API is not running"
Start services: `make run`

### "Connection refused"
Wait for services to start (10-30 seconds)

### "jq: command not found"
Install jq: `brew install jq` (macOS) or `apt-get install jq` (Linux)

### Tests timeout
Increase timeout or check service logs: `docker-compose logs`

## Documentation

See [docs/TESTING.md](../docs/TESTING.md) for comprehensive testing documentation.

## Security Philosophy

These tests embody the "trust but verify" principle:
- We trust Docker + gVisor + Deno provide isolation
- We verify with actual attempts to break isolation
- Tests prove security by attempting attacks
- Failures (of attacks) prove security works

**The best security test is an actual attack that fails.**
