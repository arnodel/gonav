#!/bin/bash

# Test script for Unified Mode: Isolated Environment + Enhanced Packages Analysis
# Verifies that the simplified, always-enabled mode works correctly

set -e

echo "================================================"
echo "Unified Mode Testing: Isolated + Enhanced Analysis"
echo "================================================"

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
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Test 1: Build the project
print_status "INFO" "Building project with unified isolated+packages mode..."
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

# Test 3: Test unified mode server startup
print_status "INFO" "Testing unified mode server startup..."
./bin/gonav > unified_output.log 2>&1 &
SERVER_PID=$!
sleep 3

if ! kill -0 $SERVER_PID 2>/dev/null; then
    print_status "FAIL" "Unified mode server failed to start"
fi

# Check if server responds
curl -s http://localhost:8080/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0 > /dev/null || {
    print_status "FAIL" "Server not responding to API requests"
}

# Verify both isolation and packages mode are enabled by default
grep -q "Running with isolated Go environment" unified_output.log || {
    print_status "FAIL" "Isolated environment not enabled"
}

grep -q "Enhanced analyzer with golang.org/x/tools/go/packages enabled" unified_output.log || {
    print_status "FAIL" "Enhanced packages mode not enabled"
}

grep -q "Running in isolated mode" unified_output.log || {
    print_status "FAIL" "Server not running in isolated mode"
}

grep -q "Running with enhanced golang.org/x/tools/go/packages analysis" unified_output.log || {
    print_status "FAIL" "Server not running with enhanced packages analysis"
}

print_status "SUCCESS" "Unified mode server works correctly"

# Test 4: Test help output shows no flags (simplified)
print_status "INFO" "Testing help output..."
help_output=$(timeout 3s ./bin/gonav --help 2>&1 || true)
echo "$help_output" | grep -q "isolated\|packages" && {
    print_status "FAIL" "Help output still shows old flags (should be simplified)"
} || true
echo "$help_output" | grep -q "Usage of" || {
    print_status "FAIL" "Help output not showing usage"
}
print_status "SUCCESS" "Help output is simplified (no mode flags)"

# Test 5: Test packages analyzer integration
print_status "INFO" "Testing packages analyzer integration..."
go test -v ./internal/analyzer -run TestPackagesAnalyzer || {
    print_status "FAIL" "Packages analyzer integration tests failed"
}
print_status "SUCCESS" "Packages analyzer integration tests passed"

# Test 6: Test frontend loading
print_status "INFO" "Testing frontend loading..."
curl -s http://localhost:8080/ | grep -q "Go Navigator" || {
    print_status "FAIL" "Frontend not loading correctly"
}
print_status "SUCCESS" "Frontend loads correctly"

# Test 7: Test API response structure
print_status "INFO" "Testing API response structure..."
response=$(curl -s "http://localhost:8080/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0" | head -c 500)
echo "$response" | grep -q '"moduleAtVersion"' || {
    print_status "FAIL" "API response missing expected structure"
}
print_status "SUCCESS" "API response has correct structure"

kill $SERVER_PID
wait $SERVER_PID 2>/dev/null || true
SERVER_PID=""

# Cleanup log files
rm -f unified_output.log

echo ""
echo "================================================"
print_status "SUCCESS" "All Unified Mode tests passed!"
echo "================================================"
echo ""
print_status "INFO" "Unified Implementation Complete:"
print_status "INFO" "• Isolated Go environment always enabled"
print_status "INFO" "• golang.org/x/tools/go/packages always enabled" 
print_status "INFO" "• Simplified codebase - no mode flags"
print_status "INFO" "• Enhanced cross-module navigation"
print_status "INFO" "• Backward API compatibility maintained"
print_status "INFO" "• Comprehensive testing suite"
echo ""
print_status "INFO" "Codebase successfully simplified!"