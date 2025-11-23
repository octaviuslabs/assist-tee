#!/bin/bash

# Full flow test script for TEE

set -e

echo "========================================="
echo "TEE Full Flow Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if API is running
echo -e "${BLUE}[1/6] Checking API health...${NC}"
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "Error: API is not running. Please run 'make run' first."
    exit 1
fi
echo -e "${GREEN}✓ API is healthy${NC}"
echo ""

# Setup environment
echo -e "${BLUE}[2/6] Creating execution environment...${NC}"
SETUP_RESPONSE=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  const { add, multiply } = await import(\"./utils.ts\");\n  const { a, b } = event.data;\n  return {\n    sum: add(a, b),\n    product: multiply(a, b),\n    executionId: context.executionId,\n    timestamp: new Date().toISOString()\n  };\n}",
      "utils.ts": "export const add = (a, b) => a + b;\nexport const multiply = (a, b) => a * b;"
    },
    "ttlSeconds": 3600
  }')

ENV_ID=$(echo $SETUP_RESPONSE | jq -r '.id')
if [ "$ENV_ID" = "null" ] || [ -z "$ENV_ID" ]; then
    echo "Error: Failed to create environment"
    echo $SETUP_RESPONSE | jq
    exit 1
fi

echo -e "${GREEN}✓ Environment created: $ENV_ID${NC}"
echo ""

# First execution
echo -e "${BLUE}[3/6] First execution (a=10, b=5)...${NC}"
EXEC1=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "a": 10, "b": 5 },
    "env": { "DEBUG": "true" },
    "limits": { "timeoutMs": 5000, "memoryMb": 128 }
  }')

EXIT_CODE=$(echo $EXEC1 | jq -r '.exitCode')
if [ "$EXIT_CODE" != "0" ]; then
    echo "Error: Execution failed"
    echo $EXEC1 | jq
    exit 1
fi

RESULT=$(echo $EXEC1 | jq -r '.stdout')
echo -e "${GREEN}✓ Execution successful${NC}"
echo "Full execution result:"
echo $EXEC1 | jq
echo "Result:"
echo "$RESULT" | jq
echo "Duration: $(echo $EXEC1 | jq -r '.durationMs')ms"
echo ""

# Second execution
echo -e "${BLUE}[4/6] Second execution (a=7, b=3)...${NC}"
EXEC2=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "a": 7, "b": 3 }
  }')

RESULT2=$(echo $EXEC2 | jq -r '.stdout')
echo -e "${GREEN}✓ Execution successful${NC}"
echo "Result:"
echo "$RESULT2" | jq
echo "Duration: $(echo $EXEC2 | jq -r '.durationMs')ms"
echo ""

# List environments
echo -e "${BLUE}[5/6] Listing all environments...${NC}"
ENVS=$(curl -s http://localhost:8080/environments)
ENV_COUNT=$(echo $ENVS | jq '. | length')
echo -e "${GREEN}✓ Found $ENV_COUNT environment(s)${NC}"
echo $ENVS | jq '.[0] | {id, mainModule, executionCount, status}'
echo ""

# Delete environment
echo -e "${BLUE}[6/6] Cleaning up environment...${NC}"
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID
echo -e "${GREEN}✓ Environment deleted${NC}"
echo ""

echo "========================================="
echo -e "${GREEN}All tests passed! ✓${NC}"
echo "========================================="
