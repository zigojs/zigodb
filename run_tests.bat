@echo off
REM ============================================================
REM Zigo-DB Test Runner for Windows
REM ============================================================

setlocal enabledelayedexpansion

echo.
echo ============================================================
echo   Zigo-DB Test Runner - Windows
echo ============================================================
echo.

REM Check if Zig is installed
where zig >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Zig is not installed or not in PATH
    echo Please install Zig from https://ziglang.org/
    exit /b 1
)

REM Check if Go is installed
where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo WARNING: Go is not installed or not in PATH
    echo Go integration tests will be skipped
    set GO_AVAILABLE=0
) else (
    set GO_AVAILABLE=1
)

set ZIG=zig
set GO=go

REM Parse command line arguments
set TARGET=%1
if "%TARGET%"=="" set TARGET=all

echo Running tests for target: %TARGET%
echo.

REM Run tests based on target
if "%TARGET%"=="all" goto test_all
if "%TARGET%"=="core" goto test_core
if "%TARGET%"=="write" goto test_write
if "%TARGET%"=="drain" goto test_drain
if "%TARGET%"=="persistence" goto test_persistence
if "%TARGET%"=="search" goto test_search
if "%TARGET%"=="temporal" goto test_temporal
if "%TARGET%"=="replication" goto test_replication
if "%TARGET%"=="go" goto test_go
if "%TARGET%"=="build" goto build_only
if "%TARGET%"=="help" goto show_help

:show_help
echo Usage: run_tests.bat [target]
echo.
echo Available targets:
echo   all          - Run all tests
echo   core         - Run core data structure tests
echo   write        - Run write path tests
echo   drain        - Run drain mechanism tests
echo   persistence  - Run persistence tests
echo   search       - Run search pool tests
echo   temporal     - Run temporal layer tests
echo   replication  - Run replication tests
echo   go           - Run Go integration tests
echo   build        - Build the library only
echo   help         - Show this help
exit /b 0

:test_all
echo [Running all tests...]
echo.

:test_core
echo [Running Core Data Structure Tests...]
cd db
%ZIG% test ..\tests\core\message_entry_test.zig -isystem . --override-lib-dir .. 
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_write
echo [Running Write Path Tests...]
cd db
%ZIG% test ..\tests\write_path\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_drain
echo [Running Drain Mechanism Tests...]
cd db
%ZIG% test ..\tests\drain\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_persistence
echo [Running Persistence Tests...]
cd db
%ZIG% test ..\tests\persistence\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_search
echo [Running Search Pool Tests...]
cd db
%ZIG% test ..\tests\search\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_temporal
echo [Running Temporal Layer Tests...]
cd db
%ZIG% test ..\tests\temporal\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_replication
echo [Running Replication Tests...]
cd db
%ZIG% test ..\tests\replication\*.zig -isystem . --override-lib-dir .. 2>nul
cd ..
if not "%TARGET%"=="all" goto end_tests

:test_go
if "%GO_AVAILABLE%"=="0" (
    echo WARNING: Go is not available, skipping Go tests
    goto end_tests
)
echo [Running Go Integration Tests...]
cd go
%GO% test -v ./...
cd ..
goto end_tests

:build_only
echo [Building ZigoDB library...]
cd db
%ZIG% build-lib -O ReleaseSafe -femit-h=zig_db.h -isystem . zigo_db.zig
cd ..
goto end_tests

:end_tests
echo.
echo ============================================================
echo   Test Run Complete
echo ============================================================
exit /b 0
