#!/bin/bash

# Master security test suite for TEE
# Runs all security and sandboxing tests

set -e

echo "========================================="
echo "TEE Complete Security Test Suite"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Check if API is running
echo -e "${BLUE}Checking API health...${NC}"
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo -e "${RED}Error: API is not running.${NC}"
    echo "Please run 'make run' or 'make run-dev' first."
    exit 1
fi
echo -e "${GREEN}✓ API is healthy${NC}"
echo ""

# Test 1: Basic functionality
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Suite 1: Basic Functionality${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
if bash "$SCRIPT_DIR/test-full-flow.sh"; then
    echo -e "${GREEN}✓ Basic functionality tests passed${NC}"
else
    echo -e "${RED}✗ Basic functionality tests failed${NC}"
    exit 1
fi
echo ""

# Test 2: Network sandboxing
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Suite 2: Network Sandboxing${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
if bash "$SCRIPT_DIR/test-network-sandbox.sh"; then
    echo -e "${GREEN}✓ Network sandboxing tests passed${NC}"
else
    echo -e "${RED}✗ Network sandboxing tests failed${NC}"
    exit 1
fi
echo ""

# Test 3: Filesystem sandboxing
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Suite 3: Filesystem Sandboxing${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
if bash "$SCRIPT_DIR/test-filesystem-sandbox.sh"; then
    echo -e "${GREEN}✓ Filesystem sandboxing tests passed${NC}"
else
    echo -e "${RED}✗ Filesystem sandboxing tests failed${NC}"
    exit 1
fi
echo ""

# Test 4: Deno permissions
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Suite 4: Deno Permissions${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
if bash "$SCRIPT_DIR/test-permissions.sh"; then
    echo -e "${GREEN}✓ Deno permissions tests passed${NC}"
else
    echo -e "${RED}✗ Deno permissions tests failed${NC}"
    exit 1
fi
echo ""

# Test 5: Dependency handling
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Suite 5: Dependency Handling${NC}"
echo -e "${BLUE}════════════════════════════════════════${NC}"
echo ""
if bash "$SCRIPT_DIR/test-dependencies.sh"; then
    echo -e "${GREEN}✓ Dependency handling tests passed${NC}"
else
    echo -e "${RED}✗ Dependency handling tests failed${NC}"
    exit 1
fi
echo ""

# Summary
echo "========================================="
echo -e "${GREEN}ALL SECURITY TESTS PASSED! ✓✓✓${NC}"
echo "========================================="
echo ""
echo "Test Results:"
echo "  ✓ Basic functionality"
echo "  ✓ Network sandboxing (HTTP, DNS, WebSocket)"
echo "  ✓ Filesystem sandboxing (read/write/execute)"
echo "  ✓ Deno permissions (net, read, write, run, ffi, hrtime)"
echo "  ✓ Dependency handling (local, remote, npm)"
echo ""
echo "The TEE is properly sandboxed and secure!"
echo ""
