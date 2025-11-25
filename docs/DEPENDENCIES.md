# Dependency Management

Guide to using external dependencies (npm packages, deno.land modules) in the TEE.

## Overview

The TEE supports external dependencies through a two-phase approach:

1. **Setup Phase** (with network): Dependencies are downloaded and cached
2. **Execution Phase** (without network): Code runs using cached dependencies

This provides both **convenience** (use npm/deno.land) and **security** (no network during execution).

## How It Works

### Setup Phase

When you create an environment with dependencies:

```json
{
  "mainModule": "main.ts",
  "modules": {
    "main.ts": "import { format } from 'npm:date-fns@3';\nexport async function handler(event, context) {\n  return { formatted: format(new Date(), 'yyyy-MM-dd') };\n}"
  },
  "dependencies": {
    "npm": ["date-fns@3", "lodash@4"],
    "deno": ["https://deno.land/std@0.224.0/assert/mod.ts"]
  },
  "ttlSeconds": 3600
}
```

The TEE will:
1. Create a Docker volume
2. Write your modules to the volume
3. **Run a dependency installation step** with network enabled
4. Cache all dependencies in the volume
5. Store the environment (ready for execution)

### Execution Phase

When you execute code:

```json
{
  "data": { "value": 42 }
}
```

The TEE will:
1. Spawn a container **without network** (`--network=none`)
2. Mount the volume with cached dependencies
3. Run your code using the cached deps
4. Return the result

## Supported Dependency Types

### 1. NPM Packages

Use the `npm:` specifier:

```typescript
// main.ts
import { format } from "npm:date-fns@3";
import _ from "npm:lodash@4";

export async function handler(event, context) {
  const date = format(new Date(), "yyyy-MM-dd");
  const data = _.chunk([1, 2, 3, 4], 2);

  return { date, data };
}
```

**Setup request:**
```json
{
  "mainModule": "main.ts",
  "modules": {
    "main.ts": "..."
  },
  "dependencies": {
    "npm": ["date-fns@3", "lodash@4"]
  }
}
```

### 2. Deno Standard Library

Use the `https://deno.land/std/` URL:

```typescript
// main.ts
import { assert } from "https://deno.land/std@0.224.0/assert/mod.ts";
import { delay } from "https://deno.land/std@0.224.0/async/delay.ts";

export async function handler(event, context) {
  assert(event.data.value > 0, "Value must be positive");
  await delay(100);

  return { validated: true };
}
```

**Setup request:**
```json
{
  "mainModule": "main.ts",
  "modules": {
    "main.ts": "..."
  },
  "dependencies": {
    "deno": [
      "https://deno.land/std@0.224.0/assert/mod.ts",
      "https://deno.land/std@0.224.0/async/delay.ts"
    ]
  }
}
```

### 3. Third-Party Deno Modules

Use full URLs:

```typescript
// main.ts
import { v4 } from "https://deno.land/x/uuid@v3.0.0/mod.ts";

export async function handler(event, context) {
  return { id: v4.generate() };
}
```

**Setup request:**
```json
{
  "mainModule": "main.ts",
  "modules": {
    "main.ts": "..."
  },
  "dependencies": {
    "deno": ["https://deno.land/x/uuid@v3.0.0/mod.ts"]
  }
}
```

## Complete Example

### Setup Environment with Dependencies

```bash
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "import { format } from \"npm:date-fns@3\";\nimport { delay } from \"https://deno.land/std@0.224.0/async/delay.ts\";\n\nexport async function handler(event, context) {\n  await delay(10);\n  const formatted = format(new Date(event.data.timestamp), \"PPP\");\n  return {\n    formatted,\n    executionId: context.executionId\n  };\n}"
    },
    "dependencies": {
      "npm": ["date-fns@3"],
      "deno": ["https://deno.land/std@0.224.0/async/delay.ts"]
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
  "dependenciesCached": true,
  "createdAt": "2025-01-15T10:30:00Z"
}
```

### Execute with Dependencies

```bash
ENV_ID="550e8400-e29b-41d4-a716-446655440000"

curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "timestamp": "2025-01-15T10:30:00Z"
    }
  }'
```

Response:
```json
{
  "id": "exec-abc123...",
  "exitCode": 0,
  "stdout": "{\"formatted\":\"January 15th, 2025\",\"executionId\":\"exec-abc123...\"}",
  "stderr": "",
  "durationMs": 145
}
```

## Implementation Details

### Dependency Installation Process

During setup, the TEE runs a special installation container:

```bash
docker run --rm \
  --network=bridge \  # Network ENABLED
  -v volume:/workspace \
  -v volume:/deno-dir \
  deno-runtime:latest \
  sh -c "
    export DENO_DIR=/deno-dir
    # Install npm packages
    deno cache --node-modules-dir npm:date-fns@3
    deno cache --node-modules-dir npm:lodash@4

    # Cache deno modules
    deno cache https://deno.land/std@0.224.0/assert/mod.ts
    deno cache https://deno.land/std@0.224.0/async/delay.ts
  "
```

This creates:
- `/workspace/` - Your code modules
- `/deno-dir/` - Cached dependencies

### Execution Process

During execution, the TEE runs:

```bash
docker run --rm \
  --network=none \  # Network DISABLED
  -v volume:/workspace:ro \
  -v volume:/deno-dir:ro \
  -e DENO_DIR=/deno-dir \
  deno-runtime:latest
```

Dependencies are loaded from `/deno-dir/` cache - no network needed!

## Best Practices

### 1. Pin Versions

Always specify exact versions:

```typescript
// ✓ Good
import { format } from "npm:date-fns@3.0.0";
import { assert } from "https://deno.land/std@0.224.0/assert/mod.ts";

// ✗ Bad
import { format } from "npm:date-fns";  // Unpredictable
import { assert } from "https://deno.land/std/assert/mod.ts";  // No version
```

### 2. Minimize Dependencies

Only include what you need:

```typescript
// ✓ Good - specific import
import { format } from "npm:date-fns@3/format";

// ✗ Bad - imports everything
import * as dateFns from "npm:date-fns@3";
```

### 3. List All Dependencies

Include all transitive dependencies in the setup request:

```json
{
  "dependencies": {
    "npm": [
      "express@4",
      "body-parser@1.20.0",  // Express dependency
      "cookie-parser@1.4.6"   // Express dependency
    ]
  }
}
```

### 4. Test Locally First

Test with Deno locally before uploading:

```bash
# Test your code with dependencies
deno run --allow-net --allow-env main.ts

# Verify imports work
deno cache main.ts
```

## Troubleshooting

### "Module not found" during execution

**Cause:** Dependency wasn't cached during setup

**Solution:** Add the dependency to the `dependencies` field:

```json
{
  "dependencies": {
    "npm": ["missing-package@1.0.0"]
  }
}
```

### "Network access denied" during dependency import

**Cause:** Dependency not in cache, trying to download during execution

**Solution:** Ensure dependency was listed in setup request

### Setup takes a long time

**Cause:** Large dependencies being downloaded

**Solutions:**
- Use specific imports instead of entire packages
- Consider bundling dependencies locally first
- Split large dependencies across multiple environments

### Version conflicts

**Cause:** Multiple versions of same package

**Solution:** Use exact versions and list all dependencies explicitly

## Security Considerations

### Setup Phase Security

During setup (with network):
- Dependencies downloaded from trusted registries (npm, deno.land)
- Code is not executed during dependency installation
- Network is only available during caching, not execution

### Execution Phase Security

During execution (without network):
- No network access - dependencies run from cache only
- Can't download additional code or exfiltrate data
- Dependencies are read-only

### Malicious Dependencies

**Risk:** Compromised npm/deno packages

**Mitigations:**
1. Pin exact versions
2. Review dependency code before use
3. Use well-known, audited packages
4. Consider vendoring critical dependencies
5. Set short TTLs to limit exposure

## Advanced: Pre-bundling

For maximum performance and security, pre-bundle dependencies:

```bash
# Bundle locally
deno bundle main.ts bundle.js

# Upload bundle (no dependencies needed)
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "bundle.js",
    "modules": {
      "bundle.js": "...<bundled code>..."
    }
  }'
```

This approach:
- ✓ No dependency installation needed
- ✓ Faster setup
- ✓ No external code at runtime
- ✓ Maximum security

## Future Enhancements

Planned features:

1. **Dependency lockfile** - Cryptographic hashes for integrity
2. **Shared dependency cache** - Reuse deps across environments
3. **Dependency scanning** - Automated vulnerability detection
4. **Allowlist/blocklist** - Restrict allowed packages
5. **Custom registries** - Private npm/deno registries

## API Reference

### Setup Request with Dependencies

```typescript
interface SetupRequest {
  mainModule: string;
  modules: Record<string, string>;
  dependencies?: {
    npm?: string[];      // npm packages: ["pkg@version"]
    deno?: string[];     // deno URLs: ["https://..."]
  };
  ttlSeconds?: number;
}
```

### Setup Response

```typescript
interface SetupResponse {
  id: string;
  volumeName: string;
  mainModule: string;
  status: "ready" | "installing" | "failed";
  dependenciesCached: boolean;
  dependencyCount?: number;
  createdAt: string;
}
```

## See Also

- [TESTING.md](TESTING.md) - Test dependency handling
- [TYPESCRIPT_EXAMPLES.md](TYPESCRIPT_EXAMPLES.md) - Code examples
- [SECURITY.md](SECURITY.md) - Security model
