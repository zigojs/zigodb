# ZigoDB
[![Build and Release](https://github.com/zigojs/zigodb/actions/workflows/build.yml/badge.svg)](https://github.com/zigojs/zigodb/actions/workflows/build.yml)

A high-performance embedded database engine built with Zig and Go. 
Designed for ultra-low latency event streaming, log systems, MMO state management, and real-time AI agent memory.

## 🚀 Performance (Hardcore Benchmarks)
Tested on **28 CPU Cores** (E5-2680 V4) | Go 1.24.5 | **32MB Double-Buffer**

### Writing (W_L / W_R Switch)
| Cores | Throughput | Latency |
| :--- | :--- | :--- |
| 1 | 2,535,099 msg/sec | 0.25 µs |
| 8 | 3,106,754 msg/sec | 0.45 µs |
| 28 | 2,781,463 msg/sec | 0.45 µs |

### Reading (Search Pool)
| Cores | Throughput | Latency |
| :--- | :--- | :--- |
| 1 | 4,095,236 msg/sec | 0.21 µs |
| 8 | 12,102,967 msg/sec | 0.27 µs |
| 28 | 12,990,739 msg/sec | 0.27 µs |

**RAM Footprint:** ~64MB (Static allocation) | **Failed Writes:** 0 (Under full load)

## Features
- **Locker Writer Architecture**: Atomic slot reservation for zero-lock multicore writes.
- **Cache-Line Isolated**: Optimized with `align(64)` to prevent False Sharing across 28+ cores.
- **Fixed-Slot Static Paging**: O(1) complexity for write/read operations.
- **Temporal Indexing**: Time-based indexing for instant historical queries.
- **Hybrid Engine**: Zig for the high-performance atomic core; Go for high-level orchestration.
- **Chain Verification**: Cryptographic integrity verification
- **Replication Support**: Gossip-based replication protocol
- **Embedded**: No external database server required

## 🛠 Architecture Basics
ZigoDB treats memory as a high-speed tunnel:
1. **Writers** grab an atomic "presence ticket" and a slot index.
2. **Data** is copied directly via SIMD-optimized `memcpy` in Zig.
3. **Double-Buffering** allows the "Leader Writer" to swap regions and drain data to disk without stopping the world.

## Building from Source

### Prerequisites

- Go 1.21+
- Zig compiler

### Download and Build

```bash
mkdir deps
cd deps
git clone https://github.com/zigojs/zigodb.git
cd zigodb
make build
cd ../..
```

## Quick Start

```bash
# Initialize your module
go mod init example.com/myproject

# Copy the required binary and header to your root
# Windows:
copy deps\zigodb\db\zigo_db.dll .
# Linux/macOS:
cp deps/zigodb/db/zigo_db.so . 2>/dev/null || cp deps/zigodb/db/zigo_db.dylib .

# Copy an example to test
copy deps\zigodb\examples\01_init\main.go .

# Link Go to the local repository
go mod edit -replace github.com/zigojs/zigodb=./deps/zigodb
go mod tidy

# Run it!
go run main.go 
```

## Examples

See the [`go/examples`](go/examples) directory for more examples:

- `01_init` - Basic initialization
- `02_write` - Writing messages
- `03_read` - Reading messages
- `04_status` - Database status
- `05_drain` - Draining messages to disk
- `06_export_rb` - Exporting to Ruby format
- `07_export_index` - Exporting with index
- `08_load_chunk` - Loading chunks
- `09_query_chunk` - Querying chunks
- `10_search` - Full-text search
- `11_temporal` - Temporal indexing
- `12_search_pool` - Search with pool
- `13_chain_verification` - Chain verification
- `14_snapshot` - Snapshots
- `15_rooms` - Room-based storage
- `16_gossip` - Gossip protocol
- `17_mmap` - Memory mapping
- `18_auto_drain` - Auto-drain
- `19_replication` - Replication
- `20_complete_chat` - Complete chat example
- `21_concurrency` - Concurrency test

## API

### Initialization

```go
err := zigodb.Init()
defer zigodb.Shutdown()
```

### Write

```go
uuid, err := zigodb.Global().Write(data)
```

### Read

```go
data, err := zigodb.Global().ReadLast()
```

### Search

```go
results, err := zigodb.Global().Search("query")
```

### Export/Import

```go
err := zigodb.Global().ExportChunk("file.rb")
err := zigodb.Global().LoadChunk("file.rb")
```
 
### Run Tests

```bash
make test
```

## License

MIT
