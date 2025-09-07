#!/bin/bash

# Test script for Stage 1: Environment Isolation
# Verifies that both normal and isolated modes work correctly

set -e

echo "========================================"
echo "Stage 1: Environment Isolation Testing"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    if [ "$1" = "SUCCESS" ]; then
        echo -e "${GREEN}✓ $2${NC}"
    elif [ "$1" = "FAIL" ]; then
        echo -e "${RED}✗ $2${NC}"
        exit 1
    elif [ "$1" = "INFO" ]; then
        echo -e "${YELLOW}ℹ $2${NC}"
    fi
}

# Function to cleanup processes
cleanup() {
    if [ ! -z "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
    fi
    if [ ! -z "$ISOLATED_SERVER_PID" ]; then
        kill $ISOLATED_SERVER_PID 2>/dev/null || true
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Test 1: Build the project
print_status "INFO" "Building project..."
go build -o bin/gonav . || {
    print_status "FAIL" "Failed to build project"
}
print_status "SUCCESS" "Project built successfully"

# Test 2: Run unit tests
print_status "INFO" "Running unit tests..."
go test ./... || {
    print_status "FAIL" "Unit tests failed"
}
print_status "SUCCESS" "All unit tests passed"

# Test 3: Test help flag
print_status "INFO" "Testing help flag..."
./bin/gonav --help 2>&1 | grep -q "isolated" || {
    print_status "FAIL" "Help flag doesn't show isolated option"
}
print_status "SUCCESS" "Help flag shows isolated option"

# Test 4: Test normal mode server startup
print_status "INFO" "Testing normal mode server startup..."
./bin/gonav > normal_output.log 2>&1 &
SERVER_PID=$!
sleep 3

if ! kill -0 $SERVER_PID 2>/dev/null; then
    print_status "FAIL" "Normal mode server failed to start"
fi

# Check if server responds
curl -s http://localhost:8080/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0 > /dev/null || {
    print_status "FAIL" "Normal mode server not responding"
}

kill $SERVER_PID
wait $SERVER_PID 2>/dev/null || true
SERVER_PID=""
print_status "SUCCESS" "Normal mode server works correctly"

# Test 5: Test isolated mode server startup  
print_status "INFO" "Testing isolated mode server startup..."
./bin/gonav --isolated > isolated_output.log 2>&1 &
ISOLATED_SERVER_PID=$!
sleep 3

if ! kill -0 $ISOLATED_SERVER_PID 2>/dev/null; then
    print_status "FAIL" "Isolated mode server failed to start"
fi

# Check if server responds
curl -s http://localhost:8080/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0 > /dev/null || {
    print_status "FAIL" "Isolated mode server not responding"
}

# Verify isolated environment was created
grep -q "Isolated Go environment created" isolated_output.log || {
    print_status "FAIL" "Isolated environment not created"
}

grep -q "Running in isolated mode" isolated_output.log || {
    print_status "FAIL" "Server not running in isolated mode"
}

kill $ISOLATED_SERVER_PID
wait $ISOLATED_SERVER_PID 2>/dev/null || true
ISOLATED_SERVER_PID=""
print_status "SUCCESS" "Isolated mode server works correctly"

# Test 6: Verify cleanup happens
print_status "INFO" "Testing cleanup functionality..."
grep -q "Cleaning up isolated environment" isolated_output.log || {
    print_status "FAIL" "Cleanup not performed"
}
print_status "SUCCESS" "Cleanup performed correctly"

# Test 7: Test host environment isolation
print_status "INFO" "Testing host environment isolation..."
go test -v ./internal/repo -run TestManagerHostIsolation || {
    print_status "FAIL" "Host environment isolation test failed"
}
print_status "SUCCESS" "Host environment properly isolated"

# Test 8: Test compatibility between modes
print_status "INFO" "Testing compatibility between normal and isolated modes..."
go test -v ./internal/repo -run TestManagerCompatibility || {
    print_status "FAIL" "Compatibility test failed"
}
print_status "SUCCESS" "Normal and isolated modes are compatible"

# Cleanup log files
rm -f normal_output.log isolated_output.log

echo ""
echo "========================================"
print_status "SUCCESS" "All Stage 1 tests passed!"
echo "========================================"
echo ""
print_status "INFO" "Stage 1 Implementation Complete:"
print_status "INFO" "• Environment isolation module created and tested"
print_status "INFO" "• Repository manager supports optional isolation"
print_status "INFO" "• Server supports --isolated flag"
print_status "INFO" "• Host environment properly isolated"
print_status "INFO" "• Cleanup functionality working"
print_status "INFO" "• Full backward compatibility maintained"
echo ""
print_status "INFO" "Ready to proceed to Stage 2!"