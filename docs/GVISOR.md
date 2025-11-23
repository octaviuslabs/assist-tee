# gVisor Configuration Guide

## What is gVisor?

gVisor is an application kernel that provides a substantial security boundary between the application and the host kernel. It implements most of the Linux system call interface in userspace, acting as a sandbox.

For the TEE, gVisor provides:
- **Hardware virtualization** - Prevents kernel exploits
- **Syscall interception** - Filters dangerous system calls
- **Process isolation** - Each execution is fully isolated
- **Resource containment** - Hard limits on CPU, memory, I/O

## gVisor vs No gVisor

| Feature | With gVisor | Without gVisor |
|---------|-------------|----------------|
| **Security** | VM-level isolation | Container-level only |
| **Kernel access** | Blocked | Accessible |
| **Exploits** | Contained | Can affect host |
| **Performance** | ~1% overhead | No overhead |
| **Startup time** | +50-100ms | Baseline |
| **Platform** | Linux only | All platforms |

## When to Use Each Mode

### ✅ **Use gVisor (Production)**

- Production deployments
- Multi-tenant environments
- Untrusted user code
- Security-critical applications
- Linux hosts

### ⚠️ **Disable gVisor (Development Only)**

- Local development on macOS/Windows
- Quick testing and iteration
- Debugging issues
- Non-Linux systems

## Installation

### Linux (Ubuntu/Debian)

```bash
# Install gVisor (runsc)
curl -fsSL https://gvisor.dev/archive.key | \
  sudo gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] \
  https://storage.googleapis.com/gvisor/releases release main" | \
  sudo tee /etc/apt/sources.list.d/gvisor.list

sudo apt-get update
sudo apt-get install -y runsc

# Configure Docker to use gVisor
sudo runsc install
sudo systemctl restart docker

# Verify installation
docker run --rm --runtime=runsc hello-world
```

### macOS/Windows

gVisor requires Linux kernel features and cannot run on macOS or Windows. For development on these platforms:

1. Use the dev compose file: `make run-dev`
2. Or set `DISABLE_GVISOR=true`

## Disabling gVisor

### Method 1: Dev Compose File (Recommended)

```bash
# Use the pre-configured dev environment
make run-dev

# Or manually
docker-compose -f docker-compose.dev.yml up -d
```

### Method 2: Environment Variable

Edit `docker-compose.yml`:
```yaml
services:
  tee-api:
    environment:
      - DISABLE_GVISOR=true  # Add this line
```

Or set when running:
```bash
DISABLE_GVISOR=true docker-compose up
```

### Method 3: Direct Binary

```bash
cd services/api
DISABLE_GVISOR=true go run cmd/api/main.go
```

## Warning Messages

When gVisor is disabled, you'll see prominent warnings:

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║  ⚠️  ⚠️  ⚠️   SECURITY WARNING: gVisor is DISABLED   ⚠️  ⚠️  ⚠️            ║
║                                                                              ║
║  Code execution is NOT sandboxed with hardware virtualization!              ║
║  User code can potentially:                                                 ║
║    - Access the host kernel                                                 ║
║    - Exploit kernel vulnerabilities                                         ║
║    - Perform timing attacks                                                 ║
║                                                                              ║
║  DO NOT USE IN PRODUCTION!                                                  ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

Additionally, every execution will log:
```
⚠️  WARNING: gVisor is DISABLED - execution is NOT sandboxed!
```

## Verification

Check if gVisor is enabled:

```bash
# Check Docker runtime config
docker info | grep -i runtime

# Should show:
# Runtimes: io.containerd.runc.v2 runc runsc

# Test gVisor execution
docker run --rm --runtime=runsc alpine echo "gVisor works!"
```

Check TEE logs:
```bash
docker-compose logs tee-api | head -20

# With gVisor:
# ✓ gVisor sandboxing: ENABLED

# Without gVisor:
# ⚠️  WARNING: gVisor is DISABLED
```

## Troubleshooting

### "unknown or invalid runtime name: runsc"

**Cause:** gVisor not installed or not configured in Docker

**Solution:**
```bash
# Install gVisor
sudo apt-get install -y runsc

# Configure Docker
sudo runsc install
sudo systemctl restart docker
```

### "operation not permitted" when running with gVisor

**Cause:** Docker daemon doesn't have permission to use runsc

**Solution:**
```bash
# Ensure runsc is in PATH
which runsc  # Should show /usr/bin/runsc

# Restart Docker daemon
sudo systemctl restart docker

# Try again
docker run --rm --runtime=runsc hello-world
```

### Slow startup with gVisor

**Expected behavior.** gVisor adds 50-100ms per container startup due to the security boundary. This is acceptable for the added security.

### Development is slow with gVisor checks

Disable gVisor for development:
```bash
make run-dev  # Uses docker-compose.dev.yml with DISABLE_GVISOR=true
```

## Performance Impact

Based on production testing:

| Metric | Without gVisor | With gVisor | Delta |
|--------|----------------|-------------|-------|
| Setup time | 500-800ms | 600-900ms | +100ms |
| Execution time | 100ms | 150ms | +50ms |
| Memory overhead | 128MB | 133MB | +5MB |
| CPU overhead | 0.5 core | 0.51 core | +1% |

**Conclusion:** gVisor adds minimal overhead (~50-100ms per execution) for substantial security benefits.

## Security Boundaries

### With gVisor

```
User Code
    ↓
Deno Runtime
    ↓
gVisor (runsc) - ← Security boundary
    ↓
Host Kernel
    ↓
Hardware
```

### Without gVisor

```
User Code
    ↓
Deno Runtime
    ↓
Container namespace - ← Weaker boundary
    ↓
Host Kernel - ← Can be exploited
    ↓
Hardware
```

## Best Practices

1. **Always use gVisor in production**
2. **Never disable gVisor for untrusted code**
3. **Use dev mode only on trusted networks**
4. **Monitor logs for disabled warnings**
5. **Regularly update gVisor**: `sudo apt-get update && sudo apt-get upgrade runsc`

## References

- [gVisor Documentation](https://gvisor.dev/)
- [gVisor Security](https://gvisor.dev/docs/architecture_guide/security/)
- [Installing gVisor](https://gvisor.dev/docs/user_guide/install/)
- [Docker Runtime Configuration](https://docs.docker.com/engine/reference/commandline/dockerd/#docker-runtime-execution-options)
