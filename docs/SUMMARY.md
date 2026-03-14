# ZigoDB Project Summary

> This document provides a comprehensive overview for AI/LLMs to understand and work with the ZigoDB project.

## Project Overview

**ZigoDB** is a high-performance binary message database written in **Zig** with **Go bindings** via CGO. Designed for real-time applications like multiplayer games and chat systems.

- **Language**: Zig (core), Go (bindings)
- **License**: Open source
- **Repository**: github.com/zigojs/zigodb

---

## Directory Structure

```
zigo-db/
├── db/                    # Zig core engine
│   ├── zigo_db.zig       # Main engine (Zig)
│   ├── types.zig         # Type definitions
│   ├── zigo_db.h         # C header (CGO)
│   ├── zigo_db.dll       # Pre-built Windows DLL
│   ├── zigo_db.lib       # Pre-built Windows import lib
│   └── shared_memory.zig # Memory management
│
├── go/                    # Go bindings
│   ├── bridge.go         # Main CGO bridge
│   ├── dumper.go         # Async chunk dumper
│   ├── go.mod            # Go module definition
│   └── examples/         # Usage examples (21 examples)
│
├── tests/                 # Zig test suite
│   ├── core/             # Core data structure tests
│   ├── write_path/       # Write path tests
│   ├── drain/            # Drain mechanism tests
│   ├── persistence/      # Persistence tests
│   ├── search/           # Search pool tests
│   ├── temporal/         # Temporal layer tests
│   └── replication/      # Replication tests
│
├── docs/                  # Documentation
│   ├── go_api.md         # Go API reference
│   └── zig_core.md       # Zig core documentation
│
├── Makefile              # Build automation
├── changelog.md          # Version history
└── README.md             # Main readme
```

---

## Key Files

### Core Engine
| File | Purpose |
|------|---------|
| `db/zigo_db.zig` | Main database engine |
| `db/types.zig` | Data structures (MessageEntry, Region, ZigoDB) |
| `db/shared_memory.zig` | Memory allocation |
| `db/zigo_db.h` | C header for CGO |

### Go Bindings
| File | Purpose |
|------|---------|
| `go/bridge.go` | Main CGO bindings |
| `go/dumper.go` | Async chunk export |

### Documentation
| File | Purpose |
|------|---------|
| `docs/go_api.md` | Go API reference |
| `docs/zig_core.md` | Zig core documentation |
| `changelog.md` | Version history and features |

---

## Building

### Prerequisites
- Zig 0.16+
- Go 1.21+
- For Windows: MinGW or Visual Studio

### Build Commands
```bash
# Build Zig library (creates db/zigo_db.dll)
make build

# Build shared library
make build-shared

# Build static library
make build-static

# Cross-compilation
make build-linux      # Linux x64
make build-macos      # macOS x64

# Run tests
make test
make test-go          # Go integration tests
```

---

## Go Usage

```go
package main

import zigodb "github.com/zigojs/zigodb"

func main() {
    // Initialize
    err := zigodb.Init()
    if err != nil {
        panic(err)
    }
    defer zigodb.Shutdown()

    // Write message
    uuid, err := zigodb.Global().Write([]byte(`{"msg": "hello"}`))
    
    // Read, search, export...
}
```

### Import
```go
import zigodb "github.com/zigojs/zigodb"
```

---

## Features

### Core Features
- **Double Buffer** - RW_A / RW_B dual 16MB regions
- **Atomic Cursor** - Multi-core write safety
- **Drain Mechanism** - 90% capacity trigger
- **Search Pool** - 1024 x 64KB segments

### Data Features
- **Room ID** - Multi-room support
- **State Hash** - Temporal verification
- **WriteLock** - Deterministic ordering

### Persistence
- ExportChunk / ExportIndex
- LoadChunk / LoadIndex
- SaveSnapshot / LoadSnapshot

### Search
- QueryChunk - Message query
- Full-Text Search - Snippet generation
- SearchWithPool - Pool-based search

### Replication (v6)
- Gossip Protocol
- Node Synchronization

---

## Architecture

```
┌─────────────────────────────────────────┐
│           Go Application                │
│         (github.com/zigojs/zigodb)      │
└──────────────────┬──────────────────────┘
                   │ CGO
                   ▼
┌─────────────────────────────────────────┐
│         ZigoDB Engine (Zig)             │
├─────────────────────────────────────────┤
│  ┌──────────┐    ┌──────────┐           │
│  │   RW_A   │ ←→ │   RW_B   │ (32MB)    │
│  └──────────┘    └──────────┘           │
│         │                │              │
│         ▼                ▼              │
│  ┌─────────────────────────┐            │
│  │      Dumper Thread      │            │
│  └───────────┬─────────────┘            │
│              ▼                          │
│  ┌──────────┐ ┌────────────┐            │
│  │ .rb file │ │ .idx file  │            │
│  └──────────┘ └────────────┘            │
└─────────────────────────────────────────┘
```

---

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| REGION_SIZE | 32 MB | Each region size |
| MAX_ENTRIES | 15625 | Entries per region |
| PAGE_SIZE | 64 KB | Memory alignment |
| SEGMENT_SIZE | 64 KB | Search segment |
| MAX_SEGMENTS | 1024 | Pool segments |

---

## Examples Location

All examples are in `go/examples/`:
- `01_init/` - Initialization
- `02_write/` - Writing messages
- `03_read/` - Reading
- `04_status/` - Status/checkpoint
- `05_drain/` - Drain mechanism
- `06_export_rb/` - Binary export
- `07_export_index/` - Index export
- `08_load_chunk/` - Load chunk
- `09_query_chunk/` - Query
- `10_search/` - Full-text search
- `11_temporal/` - Temporal rewind
- `12_search_pool/` - Search pool
- `13_chain_verification/` - Hash chain
- `14_snapshot/` - Snapshot save/load
- `15_rooms/` - Room messaging
- `16_gossip/` - Gossip protocol
- `17_mmap/` - Memory-mapped chunks
- `18_auto_drain/` - Auto drain trigger
- `19_replication/` - Replication
- `20_complete_chat/` - Complete chat example
- `21_concurrency/` - Concurrency tests
