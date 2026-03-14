#!/bin/bash
# ============================================================
# Zigo-DB Test Runner for Linux/Mac
# ============================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo "============================================================"
echo "  Zigo-DB Test Runner - Linux/Mac"
echo "============================================================"
echo ""

# Check if Zig is installed
if ! command -v zig &> /dev/null; then
    echo -e "${RED}ERROR: Zig is not installed or not in PATH${NC}"
    echo "Please install Zig from https://ziglang.org/"
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}WARNING: Go is not installed or not in PATH${NC}"
    echo "Go integration tests will be skipped"
    GO_AVAILABLE=0
else
    GO_AVAILABLE=1
fi

ZIG=zig
GO=go

# Default target
TARGET=${1:-all}

echo "Running tests for target: $TARGET"
echo ""

# Run tests based on target
case $TARGET in
    all)
        echo -e "${BLUE}[Running all tests...]${NC}"
        run_test_all
        ;;
    core)
        echo -e "${BLUE}[Running Core Data Structure Tests...]${NC}"
        run_test_core
        ;;
    write)
        echo -e "${BLUE}[Running Write Path Tests...]${NC}"
        run_test_write
        ;;
    drain)
        echo -e "${BLUE}[Running Drain Mechanism Tests...]${NC}"
        run_test_drain
        ;;
    persistence)
        echo -e "${BLUE}[Running Persistence Tests...]${NC}"
        run_test_persistence
        ;;
    search)
        echo -e "${BLUE}[Running Search Pool Tests...]${NC}"
        run_test_search
        ;;
    temporal)
        echo -e "${BLUE}[Running Temporal Layer Tests...]${NC}"
        run_test_temporal
        ;;
    replication)
        echo -e "${BLUE}[Running Replication Tests...]${NC}"
        run_test_replication
        ;;
    go)
        echo -e "${BLUE}[Running Go Integration Tests...]${NC}"
        run_test_go
        ;;
    build)
        echo -e "${BLUE}[Building ZigoDB library...]${NC}"
        run_build
        ;;
    help)
        show_help
        exit 0
        ;;
    *)
        echo -e "${RED}Unknown target: $TARGET${NC}"
        show_help
        exit 1
        ;;
esac

run_test_all() {
    run_test_core
    run_test_write
    run_test_drain
    run_test_persistence
    run_test_search
    run_test_temporal
    run_test_replication
    if [ "$GO_AVAILABLE" = "1" ]; then
        run_test_go
    fi
}

run_test_core() {
    echo -e "${BLUE}Running Core Data Structure Tests...${NC}"
    cd db
    $ZIG test ../tests/core/message_entry_test.zig -isystem . --override-lib-dir ..
    cd ..
}

run_test_write() {
    echo -e "${BLUE}Running Write Path Tests...${NC}"
    if ls tests/write_path/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/write_path/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No write path tests found${NC}"
    fi
}

run_test_drain() {
    echo -e "${BLUE}Running Drain Mechanism Tests...${NC}"
    if ls tests/drain/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/drain/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No drain tests found${NC}"
    fi
}

run_test_persistence() {
    echo -e "${BLUE}Running Persistence Tests...${NC}"
    if ls tests/persistence/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/persistence/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No persistence tests found${NC}"
    fi
}

run_test_search() {
    echo -e "${BLUE}Running Search Pool Tests...${NC}"
    if ls tests/search/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/search/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No search tests found${NC}"
    fi
}

run_test_temporal() {
    echo -e "${BLUE}Running Temporal Layer Tests...${NC}"
    if ls tests/temporal/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/temporal/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No temporal tests found${NC}"
    fi
}

run_test_replication() {
    echo -e "${BLUE}Running Replication Tests...${NC}"
    if ls tests/replication/*.zig >/dev/null 2>&1; then
        cd db
        $ZIG test ../tests/replication/*.zig -isystem . --override-lib-dir ..
        cd ..
    else
        echo -e "${YELLOW}No replication tests found${NC}"
    fi
}

run_test_go() {
    if [ "$GO_AVAILABLE" = "0" ]; then
        echo -e "${YELLOW}WARNING: Go is not available, skipping Go tests${NC}"
        return
    fi
    echo -e "${BLUE}Running Go Integration Tests...${NC}"
    cd go
    $GO test -v ./...
    cd ..
}

run_build() {
    echo -e "${BLUE}Building ZigoDB library...${NC}"
    cd db
    $ZIG build-lib -O ReleaseSafe -femit-h=zig_db.h -isystem . zigo_db.zig
    cd ..
}

show_help() {
    echo "Usage: $0 [target]"
    echo ""
    echo "Available targets:"
    echo "  all          - Run all tests"
    echo "  core         - Run core data structure tests"
    echo "  write        - Run write path tests"
    echo "  drain        - Run drain mechanism tests"
    echo "  persistence  - Run persistence tests"
    echo "  search       - Run search pool tests"
    echo "  temporal     - Run temporal layer tests"
    echo "  replication  - Run replication tests"
    echo "  go           - Run Go integration tests"
    echo "  build        - Build the library only"
    echo "  help         - Show this help"
    echo ""
    echo "Examples:"
    echo "  $0"
    echo "  $0 core"
    echo "  $0 build"
}

echo ""
echo "============================================================"
echo -e "  ${GREEN}Test Run Complete${NC}"
echo "============================================================"
