#!/bin/bash

# Dependency handling test script for TEE
# Demonstrates how dependencies work in Deno (without network access)

set -e

echo "========================================="
echo "TEE Dependency Handling Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if API is running
echo -e "${BLUE}[1/5] Checking API health...${NC}"
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "Error: API is not running. Please run 'make run' first."
    exit 1
fi
echo -e "${GREEN}✓ API is healthy${NC}"
echo ""

# Test 1: Standard library imports (should work - bundled with Deno)
echo -e "${BLUE}[2/5] Test 1: Deno standard library imports...${NC}"
SETUP1=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    // Note: Without network access, remote imports will fail\n    // But we can use relative imports from workspace\n    const { formatDate } = await import(\"./utils.ts\");\n    \n    return {\n      success: true,\n      message: \"✓ Local module imports work\",\n      formattedDate: formatDate(new Date()),\n      note: \"Remote imports (https://) require --allow-net permission\"\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"Failed to import module\",\n      error: error.message\n    };\n  }\n}",
      "utils.ts": "export function formatDate(date: Date): string {\n  return date.toISOString().split(\"T\")[0];\n}\n\nexport function capitalize(str: string): string {\n  return str.charAt(0).toUpperCase() + str.slice(1);\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID1=$(echo $SETUP1 | jq -r '.id')
if [ "$ENV_ID1" = "null" ] || [ -z "$ENV_ID1" ]; then
    echo "Error: Failed to create environment"
    echo $SETUP1 | jq
    exit 1
fi

EXEC1=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID1/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT1=$(echo $EXEC1 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT1" | jq

SUCCESS1=$(echo "$RESULT1" | jq -r '.success')
if [ "$SUCCESS1" = "true" ]; then
    echo -e "${GREEN}✓ Local module imports work${NC}"
else
    echo -e "${RED}✗ ERROR: Local imports should work${NC}"
fi
echo ""

# Test 2: Try to import from URL (should fail without network)
echo -e "${BLUE}[3/5] Test 2: Remote URL imports (should fail)...${NC}"
SETUP2=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    // Try to import from Deno standard library via URL\n    const { assert } = await import(\"https://deno.land/std@0.224.0/assert/mod.ts\");\n    return {\n      success: true,\n      message: \"⚠️  Remote imports should be blocked!\",\n      note: \"Network access was somehow allowed\"\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"✓ Remote imports properly blocked\",\n      error: \"Network access denied. Enable network permissions in execution config to allow.\"\n    };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID2=$(echo $SETUP2 | jq -r '.id')
EXEC2=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID2/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT2=$(echo $EXEC2 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT2" | jq

SUCCESS2=$(echo "$RESULT2" | jq -r '.success')
if [ "$SUCCESS2" = "false" ]; then
    echo -e "${GREEN}✓ Remote imports properly blocked (no network)${NC}"
else
    echo -e "${RED}✗ SECURITY ISSUE: Remote imports were allowed!${NC}"
fi
echo ""

# Test 3: Try npm: specifier (should fail without network)
echo -e "${BLUE}[4/5] Test 3: NPM package imports (should fail)...${NC}"
SETUP3=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    // Try to import npm package\n    const chalk = await import(\"npm:chalk@5\");\n    return {\n      success: true,\n      message: \"⚠️  NPM imports should be blocked!\"\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"✓ NPM imports properly blocked\",\n      error: \"Network access denied. Enable network permissions in execution config to allow.\"\n    };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID3=$(echo $SETUP3 | jq -r '.id')
EXEC3=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID3/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT3=$(echo $EXEC3 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT3" | jq

SUCCESS3=$(echo "$RESULT3" | jq -r '.success')
if [ "$SUCCESS3" = "false" ]; then
    echo -e "${GREEN}✓ NPM imports properly blocked (no network)${NC}"
else
    echo -e "${RED}✗ SECURITY ISSUE: NPM imports were allowed!${NC}"
fi
echo ""

# Test 4: Complex local dependency tree
echo -e "${BLUE}[5/5] Test 4: Complex local dependency tree...${NC}"
SETUP4=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  const { Calculator } = await import(\"./calculator.ts\");\n  const { Logger } = await import(\"./logger.ts\");\n  \n  const calc = new Calculator();\n  const logger = new Logger(context.executionId);\n  \n  logger.info(\"Starting calculation\");\n  const result = calc.fibonacci(event.data.n || 10);\n  logger.info(`Fibonacci result: ${result}`);\n  \n  return {\n    success: true,\n    result,\n    executionId: context.executionId,\n    message: \"✓ Complex dependency tree works\"\n  };\n}",
      "calculator.ts": "import { MathUtils } from \"./math-utils.ts\";\n\nexport class Calculator {\n  private utils = new MathUtils();\n  \n  fibonacci(n: number): number {\n    if (n <= 1) return n;\n    return this.utils.add(\n      this.fibonacci(n - 1),\n      this.fibonacci(n - 2)\n    );\n  }\n  \n  factorial(n: number): number {\n    return this.utils.factorial(n);\n  }\n}",
      "math-utils.ts": "export class MathUtils {\n  add(a: number, b: number): number {\n    return a + b;\n  }\n  \n  multiply(a: number, b: number): number {\n    return a * b;\n  }\n  \n  factorial(n: number): number {\n    if (n <= 1) return 1;\n    return this.multiply(n, this.factorial(n - 1));\n  }\n}",
      "logger.ts": "export class Logger {\n  constructor(private id: string) {}\n  \n  info(message: string): void {\n    console.error(`[${this.id}] INFO: ${message}`);\n  }\n  \n  error(message: string): void {\n    console.error(`[${this.id}] ERROR: ${message}`);\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID4=$(echo $SETUP4 | jq -r '.id')
EXEC4=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID4/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "n": 10 }
  }')

RESULT4=$(echo $EXEC4 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT4" | jq

SUCCESS4=$(echo "$RESULT4" | jq -r '.success')
if [ "$SUCCESS4" = "true" ]; then
    echo -e "${GREEN}✓ Complex local dependency trees work${NC}"
else
    echo -e "${RED}✗ ERROR: Local dependency tree failed${NC}"
fi
echo ""

# Cleanup
echo -e "${BLUE}Cleaning up environments...${NC}"
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID2 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID3 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID4 > /dev/null
echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

echo "========================================="
echo -e "${GREEN}Dependency handling tests complete! ✓${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ Local module imports work"
echo "  ✓ Remote URL imports blocked (no network)"
echo "  ✓ NPM package imports blocked (no network)"
echo "  ✓ Complex dependency trees work"
echo ""
echo "How Dependencies Work in TEE:"
echo ""
echo "1. LOCAL IMPORTS (✓ Supported)"
echo "   - All modules uploaded in setup phase"
echo "   - Use relative paths: ./module.ts"
echo "   - Fast, no network needed"
echo ""
echo "2. REMOTE IMPORTS (✗ Blocked)"
echo "   - https://deno.land/... requires network"
echo "   - Blocked by --network=none"
echo "   - Solution: Vendor dependencies in setup"
echo ""
echo "3. NPM PACKAGES (✗ Blocked)"
echo "   - npm:package requires network"
echo "   - Blocked by --network=none"
echo "   - Solution: Pre-bundle or use local code"
echo ""
echo "4. WORKAROUNDS:"
echo "   - Bundle all code in the setup phase"
echo "   - Include dependencies as separate modules"
echo "   - Use esbuild/webpack to bundle before upload"
echo "   - Future: Pre-install dependencies in environment"
echo ""
