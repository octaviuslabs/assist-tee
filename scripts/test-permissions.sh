#!/bin/bash

# Deno permissions test script for TEE
# Demonstrates all the Deno permissions that are restricted

set -e

echo "========================================="
echo "TEE Deno Permissions Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if API is running
echo -e "${BLUE}[1/7] Checking API health...${NC}"
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "Error: API is not running. Please run 'make run' first."
    exit 1
fi
echo -e "${GREEN}✓ API is healthy${NC}"
echo ""

# Test 1: --allow-net (network access)
echo -e "${BLUE}[2/7] Test 1: Network permission (--allow-net)...${NC}"
SETUP1=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    await fetch(\"https://www.google.com\");\n    return { permitted: true, permission: \"network\", message: \"⚠️  Network access allowed\" };\n  } catch (error) {\n    return { permitted: false, permission: \"network\", message: \"✓ Network access denied\", error: \"Network access denied. Enable network permissions in execution config to allow.\" };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID1=$(echo $SETUP1 | jq -r '.id')
EXEC1=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID1/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT1=$(echo $EXEC1 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT1" | jq

PERMITTED1=$(echo "$RESULT1" | jq -r '.permitted')
if [ "$PERMITTED1" = "false" ]; then
    echo -e "${GREEN}✓ --allow-net properly restricted${NC}"
else
    echo -e "${RED}✗ WARNING: --allow-net is not restricted${NC}"
fi
echo ""

# Test 2: --allow-read (filesystem read)
echo -e "${BLUE}[3/7] Test 2: Read permission (--allow-read)...${NC}"
SETUP2=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    await Deno.readTextFile(\"/etc/hosts\");\n    return { permitted: true, permission: \"read\", message: \"⚠️  Can read system files\" };\n  } catch (error) {\n    return { permitted: false, permission: \"read\", message: \"✓ System file read denied\", error: \"File read access denied. Enable read permissions in execution config to allow.\" };\n  }\n}"
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

PERMITTED2=$(echo "$RESULT2" | jq -r '.permitted')
if [ "$PERMITTED2" = "false" ]; then
    echo -e "${GREEN}✓ --allow-read properly restricted${NC}"
else
    echo -e "${RED}✗ WARNING: --allow-read is not restricted${NC}"
fi
echo ""

# Test 3: --allow-write (filesystem write)
echo -e "${BLUE}[4/7] Test 3: Write permission (--allow-write)...${NC}"
SETUP3=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    await Deno.writeTextFile(\"/tmp/test.txt\", \"test\");\n    return { permitted: true, permission: \"write\", message: \"⚠️  Can write files\" };\n  } catch (error) {\n    return { permitted: false, permission: \"write\", message: \"✓ File write denied\", error: \"File write access denied. Enable write permissions in execution config to allow.\" };\n  }\n}"
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

PERMITTED3=$(echo "$RESULT3" | jq -r '.permitted')
if [ "$PERMITTED3" = "false" ]; then
    echo -e "${GREEN}✓ --allow-write properly restricted${NC}"
else
    echo -e "${RED}✗ WARNING: --allow-write is not restricted${NC}"
fi
echo ""

# Test 4: --allow-run (subprocess execution)
echo -e "${BLUE}[5/7] Test 4: Run permission (--allow-run)...${NC}"
SETUP4=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    const cmd = new Deno.Command(\"echo\", { args: [\"test\"] });\n    await cmd.output();\n    return { permitted: true, permission: \"run\", message: \"⚠️  Can run commands\" };\n  } catch (error) {\n    return { permitted: false, permission: \"run\", message: \"✓ Command execution denied\", error: \"Subprocess execution denied. Enable run permissions in execution config to allow.\" };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID4=$(echo $SETUP4 | jq -r '.id')
EXEC4=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID4/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT4=$(echo $EXEC4 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT4" | jq

PERMITTED4=$(echo "$RESULT4" | jq -r '.permitted')
if [ "$PERMITTED4" = "false" ]; then
    echo -e "${GREEN}✓ --allow-run properly restricted${NC}"
else
    echo -e "${RED}✗ WARNING: --allow-run is not restricted${NC}"
fi
echo ""

# Test 5: --allow-ffi (foreign function interface)
echo -e "${BLUE}[6/7] Test 5: FFI permission (--allow-ffi)...${NC}"
SETUP5=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    const lib = Deno.dlopen(\"/lib/x86_64-linux-gnu/libc.so.6\", {});\n    return { permitted: true, permission: \"ffi\", message: \"⚠️  Can load native libraries\" };\n  } catch (error) {\n    return { permitted: false, permission: \"ffi\", message: \"✓ FFI access denied\", error: \"Native library access denied. Enable FFI permissions in execution config to allow.\" };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID5=$(echo $SETUP5 | jq -r '.id')
EXEC5=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID5/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT5=$(echo $EXEC5 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT5" | jq

PERMITTED5=$(echo "$RESULT5" | jq -r '.permitted')
if [ "$PERMITTED5" = "false" ]; then
    echo -e "${GREEN}✓ --allow-ffi properly restricted${NC}"
else
    echo -e "${RED}✗ WARNING: --allow-ffi is not restricted${NC}"
fi
echo ""

# Test 6: --allow-hrtime (high resolution time)
echo -e "${BLUE}[7/7] Test 6: High-res time permission (--allow-hrtime)...${NC}"
SETUP6=$(curl -s -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  try {\n    // Try to get high precision time (could be used for timing attacks)\n    const start = performance.now();\n    for (let i = 0; i < 1000; i++) { /* busy work */ }\n    const end = performance.now();\n    const precision = (end - start).toString().split(\".\")[1]?.length || 0;\n    \n    // High precision (>2 decimal places) means hrtime is allowed\n    const highPrecision = precision > 2;\n    \n    return { \n      permitted: highPrecision, \n      permission: \"hrtime\", \n      message: highPrecision ? \"⚠️  High-resolution time available\" : \"✓ High-resolution time restricted\",\n      precision: precision,\n      note: \"High precision timing can enable side-channel attacks\"\n    };\n  } catch (error) {\n    return { permitted: false, permission: \"hrtime\", message: \"✓ Timing restricted\", error: \"High-resolution timing denied. Enable hrtime permissions in execution config to allow.\" };\n  }\n}"
    },
    "ttlSeconds": 300
  }')

ENV_ID6=$(echo $SETUP6 | jq -r '.id')
EXEC6=$(curl -s -X POST http://localhost:8080/environments/$ENV_ID6/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}')

RESULT6=$(echo $EXEC6 | jq -r '.stdout')
echo -e "${YELLOW}Result:${NC}"
echo "$RESULT6" | jq

PERMITTED6=$(echo "$RESULT6" | jq -r '.permitted')
if [ "$PERMITTED6" = "false" ]; then
    echo -e "${GREEN}✓ --allow-hrtime properly restricted${NC}"
else
    echo -e "${YELLOW}⚠️  Note: High-resolution time is available${NC}"
    echo -e "${YELLOW}   This may allow timing-based side-channel attacks${NC}"
fi
echo ""

# Cleanup
echo -e "${BLUE}Cleaning up environments...${NC}"
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID1 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID2 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID3 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID4 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID5 > /dev/null
curl -s -X DELETE http://localhost:8080/environments/$ENV_ID6 > /dev/null
echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

echo "========================================="
echo -e "${GREEN}Deno permissions test complete! ✓${NC}"
echo "========================================="
echo ""
echo "Summary of Permissions:"
echo "  Network access:        $([ "$PERMITTED1" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo "  File read access:      $([ "$PERMITTED2" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo "  File write access:     $([ "$PERMITTED3" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo "  Subprocess execution:  $([ "$PERMITTED4" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo "  Native libraries:      $([ "$PERMITTED5" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo "  High-res timing:       $([ "$PERMITTED6" = "false" ] && echo "✓ Blocked" || echo "⚠️  Allowed")"
echo ""
