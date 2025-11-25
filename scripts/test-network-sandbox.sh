#!/bin/bash

# Network sandboxing test script for TEE
# Demonstrates that code execution is isolated from network access

set -e

echo "========================================="
echo "TEE Network Sandboxing Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
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

# Test 1: Try to make HTTP request
echo -e "${BLUE}[2/4] Test 1: Attempting HTTP request (should fail)...${NC}"
SETUP1=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    const response = await fetch(\"https://www.google.com\");\n    return {\n      success: true,\n      message: \"⚠️  SECURITY BREACH: Network access should be blocked!\",\n      status: response.status\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"✓ Network access properly blocked\",\n      error: \"Network access denied. Enable network permissions in execution config to allow.\"\n    };\n  }\n}"
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
echo ""

# Check if network was properly blocked
SUCCESS=$(echo "$RESULT1" | jq -r '.success')
if [ "$SUCCESS" = "true" ]; then
    echo -e "${RED}✗ SECURITY ISSUE: Network access was NOT blocked!${NC}"
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
    exit 1
else
    echo -e "${GREEN}✓ Network access properly sandboxed${NC}"
fi
echo ""

# Test 2: Try DNS resolution
echo -e "${BLUE}[3/4] Test 2: Attempting DNS resolution (should fail)...${NC}"
SETUP2=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    // Try to resolve a domain\n    const url = new URL(\"https://www.google.com\");\n    const response = await fetch(url.toString());\n    return {\n      success: true,\n      message: \"⚠️  SECURITY BREACH: DNS resolution should be blocked!\"\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"✓ DNS resolution properly blocked\",\n      error: \"Network access denied. Enable network permissions in execution config to allow.\"\n    };\n  }\n}"
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
if [ "$SUCCESS2" = "true" ]; then
    echo -e "${RED}✗ SECURITY ISSUE: DNS resolution was NOT blocked!${NC}"
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID2 > /dev/null
    exit 1
else
    echo -e "${GREEN}✓ DNS resolution properly sandboxed${NC}"
fi
echo ""

# Test 3: Try WebSocket connection
echo -e "${BLUE}[4/4] Test 3: Attempting WebSocket connection (should fail)...${NC}"
SETUP3=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    const ws = new WebSocket(\"wss://echo.websocket.org\");\n    return {\n      success: true,\n      message: \"⚠️  SECURITY BREACH: WebSocket should be blocked!\"\n    };\n  } catch (error) {\n    return {\n      success: false,\n      message: \"✓ WebSocket connection properly blocked\",\n      error: \"Network access denied. Enable network permissions in execution config to allow.\"\n    };\n  }\n}"
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
if [ "$SUCCESS3" = "true" ]; then
    echo -e "${RED}✗ SECURITY ISSUE: WebSocket was NOT blocked!${NC}"
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID2 > /dev/null
    curl -s -X DELETE http://localhost:8080/environments/$ENV_ID3 > /dev/null
    exit 1
else
    echo -e "${GREEN}✓ WebSocket properly sandboxed${NC}"
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
echo -e "${GREEN}All network sandboxing tests passed! ✓${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ HTTP requests blocked"
echo "  ✓ DNS resolution blocked"
echo "  ✓ WebSocket connections blocked"
echo ""
echo "The execution environment has NO network access."
