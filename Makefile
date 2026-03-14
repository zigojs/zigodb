# ============================================================
# Zigo-DB Makefile
# Supports both Windows and Linux
# ============================================================

# Detect OS
ifeq ($(OS),Windows_NT)
    DETECTED_OS := windows
    ZIG := zig.exe
    GO := go.exe
    RM := del /Q
    COPY := copy /Y
    MKDIR := mkdir
    TESTS_DIR := tests
    TEST_EXT := .exe
else
    DETECTED_OS := linux
    ZIG := zig
    GO := go
    RM := rm -f
    COPY := cp
    MKDIR := mkdir -p
    TESTS_DIR := tests
    TEST_EXT :=
endif

# Project paths
DB_DIR := db
GO_DIR := go

# Zig build settings
ZIG_TARGET := native-native
ZIG_OPT := ReleaseSafe

# Colors (works on both Windows and Linux)
ifeq ($(DETECTED_OS),windows)
    RED := 
    GREEN := 
    YELLOW := 
    BLUE := 
    NC := 
else
    RED := \033[0;31m
    GREEN := \033[0;32m
    YELLOW := \033[0;33m
    BLUE := \033[0;34m
    NC := \033[0m
endif

# ============================================================
# Build Targets
# ============================================================

.PHONY: all build build-shared build-static test test-core test-write test-drain test-persistence test-search test-temporal test-replication test-go clean help

all: build

# Build the ZigoDB library (default target - Windows)
build:
	@echo "$(BLUE)Building ZigoDB library...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target x86_64-windows-msvc -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

# Build shared library for CGO (native)
build-shared:
	@echo "$(BLUE)Building shared library for CGO (native)...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

# Build static library (no DLL required - native)
build-static:
	@echo "$(BLUE)Building static library (no DLL - native)...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -lc -femit-h=zig_db.h -isystem . zigo_db.zig

# ============================================================
# Cross-compilation Targets
# ============================================================
# Build for different platforms (output goes to db/)

build-windows:
	@echo "$(BLUE)Building for Windows...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target x86_64-windows-gnu -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

build-linux:
	@echo "$(BLUE)Building for Linux x64...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target x86_64-linux-gnu -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

build-linux-arm64:
	@echo "$(BLUE)Building for Linux ARM64...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target aarch64-linux-gnu -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

build-macos:
	@echo "$(BLUE)Building for macOS x64...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target x86_64-macos -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

build-macos-arm64:
	@echo "$(BLUE)Building for macOS ARM64...$(NC)"
	cd $(DB_DIR) && $(ZIG) build-lib -O $(ZIG_OPT) -target aarch64-macos -lc -dynamic -femit-h=zig_db.h -isystem . zigo_db.zig

# ============================================================
# Test Targets
# ============================================================

test: test-core test-write test-drain test-persistence test-search test-temporal test-replication
	@echo "$(GREEN)All tests completed!$(NC)"

# Run core data structure tests
test-core:
	@echo "$(BLUE)Running core data structure tests...$(NC)"
	$(ZIG) test tests/core/message_entry_test.zig

# Run write path tests
test-write:
	@echo "$(BLUE)Running write path tests...$(NC)"
	$(ZIG) test tests/write_path/write_path_test.zig

# Run drain mechanism tests
test-drain:
	@echo "$(BLUE)Running drain mechanism tests...$(NC)"
	$(ZIG) test tests/drain/drain_test.zig

# Run persistence tests
test-persistence:
	@echo "$(BLUE)Running persistence tests...$(NC)"
	$(ZIG) test tests/persistence/persistence_test.zig

# Run search pool tests
test-search:
	@echo "$(BLUE)Running search pool tests...$(NC)"
	$(ZIG) test tests/search/search_pool_test.zig

# Run temporal layer tests
test-temporal:
	@echo "$(BLUE)Running temporal layer tests...$(NC)"
	$(ZIG) test tests/temporal/temporal_layer_test.zig

# Run replication tests
test-replication:
	@echo "$(BLUE)Running replication tests...$(NC)"
	$(ZIG) test tests/replication/replication_test.zig

# ============================================================
# Go Integration Tests
# ============================================================

test-go:
	@echo "$(BLUE)Running Go integration tests...$(NC)"
	cd $(GO_DIR) && $(GO) test -v ./...

# ============================================================
# Build & Test Combined
# ============================================================

build-and-test: build test

build-and-test-go: build-shared test-go

# ============================================================
# Development Targets
# ============================================================

# Format code
fmt:
	@echo "$(BLUE)Formatting Zig code...$(NC)"
	$(ZIG) fmt tests/
	$(ZIG) fmt $(DB_DIR)/*.zig

# ============================================================
# Cleanup
# ============================================================

clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	$(RM) $(DB_DIR)/*.o
	$(RM) $(DB_DIR)/*.h
	$(RM) $(DB_DIR)/*.dll
	$(RM) $(DB_DIR)/*.so
	$(RM) $(DB_DIR)/*.a

clean-tests:
	@echo "$(YELLOW)Cleaning test artifacts...$(NC)"
	$(RM) $(DB_DIR)/test$(TEST_EXT)

# ============================================================
# Help
# ============================================================

help:
	@echo "$(BLUE)=============================================$(NC)"
	@echo "$(BLUE)  Zigo-DB Makefile - Available Targets$(NC)"
	@echo "$(BLUE)=============================================$(NC)"
	@echo ""
	@echo "Build Targets:"
	@echo "  build              Build ZigoDB library (Windows)"
	@echo "  build-shared      Build shared library (DLL)"
	@echo "  build-static      Build static library"
	@echo ""
	@echo "Cross-compilation:"
	@echo "  build-windows      Build for Windows"
	@echo "  build-linux        Build for Linux x64"
	@echo "  build-linux-arm64 Build for Linux ARM64"
	@echo "  build-macos        Build for macOS x64"
	@echo "  build-macos-arm64  Build for macOS ARM64"
	@echo ""
	@echo "Test Targets:"
	@echo "  test               Run all tests"
	@echo "  test-go            Run Go integration tests"
	@echo ""
	@echo "Development:"
	@echo "  build-and-test     Build and run all tests"
	@echo "  build-and-test-go  Build and run Go tests"
	@echo "  fmt                Format code"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make build-shared"
	@echo "  make test-go"
	@echo ""
	@echo "Current OS: $(DETECTED_OS)"
	@echo "$(BLUE)=============================================$(NC)"
