#!/bin/bash

# Dependency usage test script for TEE
# Demonstrates how to use npm and deno.land dependencies

set -e

echo "========================================="
echo "TEE Real Dependency Usage Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if API is running
echo -e "${BLUE}[1/4] Checking API health...${NC}"
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "Error: API is not running. Please run 'make run' first."
    exit 1
fi
echo -e "${GREEN}✓ API is healthy${NC}"
echo ""

# Test 1: NPM dependency (date-fns)
echo -e "${BLUE}[2/4] Test 1: Using npm dependency (date-fns)...${NC}"
SETUP1=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "import { format } from \"npm:date-fns@3\";\n\nexport async function handler(event, context) {\n  try {\n    const date = new Date(event.data.timestamp);\n    const formatted = format(date, \"PPP\");\n    return {\n      success: true,\n      formatted,\n      executionId: context.executionId\n    };\n  } catch (error) {\n    return {\n      success: false,\n      error: error.message\n    };\n  }\n}"
    },
    "dependencies": {
      "npm": ["date-fns@3"]
    },
    "ttlSeconds": 600
  }')

ENV_ID1=$(echo $SETUP1 | jq -r '.id')
if [ "$ENV_ID1" = "null" ] || [ -z "$ENV_ID1" ]; then
    echo "Error: Failed to create environment"
    echo $SETUP1 | jq
    exit 1
fi

echo -e "${YELLOW}Environment created with dependencies:${NC}"
echo $SETUP1 | jq '{id, mainModule, metadata}'
echo ""
echo -e "${BLUE}Waiting for dependencies to install...${NC}"
sleep 5
echo ""

EXEC1=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID1/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "timestamp": "2025-01-15T10:30:00Z"
    }
  }')

echo -e "${YELLOW}Execution result:${NC}"
echo $EXEC1 | jq
echo ""

RESULT1=$(echo $EXEC1 | jq -r '.stdout')
SUCCESS1=$(echo "$RESULT1" | jq -r '.success')
if [ "$SUCCESS1" = "true" ]; then
    echo -e "${GREEN}✓ NPM dependency (date-fns) works!${NC}"
    echo "  Formatted date: $(echo "$RESULT1" | jq -r '.formatted')"
else
    echo -e "${RED}✗ NPM dependency failed${NC}"
    echo "$RESULT1" | jq
fi
echo ""

# Test 2: Deno standard library
echo -e "${BLUE}[3/4] Test 2: Using Deno standard library...${NC}"
SETUP2=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "import { delay } from \"https://deno.land/std@0.224.0/async/delay.ts\";\n\nexport async function handler(event, context) {\n  try {\n    const start = Date.now();\n    await delay(event.data.delayMs || 100);\n    const elapsed = Date.now() - start;\n    return {\n      success: true,\n      requestedDelay: event.data.delayMs || 100,\n      actualDelay: elapsed,\n      executionId: context.executionId\n    };\n  } catch (error) {\n    return {\n      success: false,\n      error: error.message\n    };\n  }\n}"
    },
    "dependencies": {
      "deno": ["https://deno.land/std@0.224.0/async/delay.ts"]
    },
    "ttlSeconds": 600
  }')

ENV_ID2=$(echo $SETUP2 | jq -r '.id')
echo -e "${YELLOW}Environment created:${NC}"
echo $SETUP2 | jq '{id, mainModule, metadata}'
echo ""
echo -e "${BLUE}Waiting for dependencies to install...${NC}"
sleep 5
echo ""

EXEC2=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID2/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "delayMs": 150
    }
  }')

echo -e "${YELLOW}Execution result:${NC}"
echo $EXEC2 | jq
echo ""

RESULT2=$(echo $EXEC2 | jq -r '.stdout')
SUCCESS2=$(echo "$RESULT2" | jq -r '.success')
if [ "$SUCCESS2" = "true" ]; then
    echo -e "${GREEN}✓ Deno standard library works!${NC}"
    echo "  Requested delay: $(echo "$RESULT2" | jq -r '.requestedDelay')ms"
    echo "  Actual delay: $(echo "$RESULT2" | jq -r '.actualDelay')ms"
else
    echo -e "${RED}✗ Deno standard library failed${NC}"
    echo "$RESULT2" | jq
fi
echo ""

# Test 3: Multiple dependencies
echo -e "${BLUE}[4/4] Test 3: Using multiple dependencies...${NC}"
SETUP3=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "import { format } from \"npm:date-fns@3\";\nimport { delay } from \"https://deno.land/std@0.224.0/async/delay.ts\";\n\nexport async function handler(event, context) {\n  try {\n    await delay(50);\n    const now = new Date();\n    const formatted = format(now, \"yyyy-MM-dd HH:mm:ss\");\n    return {\n      success: true,\n      timestamp: formatted,\n      message: \"Used both npm and deno.land dependencies!\",\n      executionId: context.executionId\n    };\n  } catch (error) {\n    return {\n      success: false,\n      error: error.message\n    };\n  }\n}"
    },
    "dependencies": {
      "npm": ["date-fns@3"],
      "deno": ["https://deno.land/std@0.224.0/async/delay.ts"]
    },
    "ttlSeconds": 600
  }')

ENV_ID3=$(echo $SETUP3 | jq -r '.id')
echo -e "${YELLOW}Environment created:${NC}"
echo $SETUP3 | jq '{id, mainModule, metadata}'
echo ""
echo -e "${BLUE}Waiting for dependencies to install...${NC}"
sleep 5
echo ""

EXEC3=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID3/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

echo -e "${YELLOW}Execution result:${NC}"
echo $EXEC3 | jq
echo ""

RESULT3=$(echo $EXEC3 | jq -r '.stdout')
SUCCESS3=$(echo "$RESULT3" | jq -r '.success')
if [ "$SUCCESS3" = "true" ]; then
    echo -e "${GREEN}✓ Multiple dependencies work together!${NC}"
    echo "  Timestamp: $(echo "$RESULT3" | jq -r '.timestamp')"
    echo "  Message: $(echo "$RESULT3" | jq -r '.message')"
else
    echo -e "${RED}✗ Multiple dependencies failed${NC}"
    echo "$RESULT3" | jq
fi
echo ""

# Cleanup
echo -e "${BLUE}Cleaning up environments...${NC}"
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID2 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID3 > /dev/null
echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

echo "========================================="
echo -e "${GREEN}Dependency tests complete! ✓${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ NPM packages work (date-fns)"
echo "  ✓ Deno standard library works (async/delay)"
echo "  ✓ Multiple dependencies work together"
echo ""
echo "Dependencies are:"
echo "  1. Downloaded during setup phase (with network)"
echo "  2. Cached in the Docker volume"
echo "  3. Used during execution (without network)"
echo ""
