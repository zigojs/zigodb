# ZigoDB - Core Layer (Zig)

## Overview

ZigoDB is a binary message database built with **Zig 0.16+** for high-performance applications:
- Real-time multiplayer games
- Scalable chat systems
- High-throughput structured logs

**Source Code:**
- `db/zigo_db.zig` - Main engine
- `db/types.zig` - Type definitions
- `tests/` - Test suite

---

## 1. Memory Architecture

### Double Buffer (RW_A / RW_B)

Two identical memory regions (16MB each). Only one receives active writes:

```
RW_A (16MB) ←→ RW_B (16MB)
    ↑              ↑
region_bit (atomic u32)
```

Control:
- `region_bit = 0` → RW_A active
- `region_bit = 1` → RW_B active

**Implementation:** `db/types.zig:72-92`

### Atomic Cursor (Multi-Core Writers)

Each writer gets a unique slot via atomic fetch-add:

```zig
// 1. Increment cursor and get unique index
const my_idx = @atomicRmw(u32, &region.cursor, .Add, 1, .monotonic);

// 2. Mark presence (for deterministic drain)
_ = @atomicRmw(i32, &region.active_writers, .Add, 1, .monotonic);

// 3. Write to slot my_idx
```

**Impossible to collide**: Hardware guarantees unique numbers per thread.

### Drain Mechanism

Triggers: 90% capacity, 10k events, 30s, or forced:

```zig
// 1. Flip the bit
@atomicStore(u32, &db.region_bit, new_bit, .release);

// 2. Wait for writers to finish (deterministic spin-lock)
while (@atomicLoad(i32, &old_region.active_writers, .acquire) > 0) {
    std.Thread.yield();
}

// 3. Dump region to disk
dump_region(old_region, filename);
```

### Search Pool (R_S)

Read-only segment pool (64MB total):
- 1024 segments × 64KB each
- Atomic bitmap for reservation
- Direct mmap from .rb file (zero-copy)
- Immediate release after use

---

## 2. Data Structures

### MessageEntry (1072 bytes)

```zig
pub const MessageEntry = extern struct {
    room_id:    u32 align(1),    // Room/chatroom ID
    uuid:       u64 align(1),    // Unique ID (monotonic)
    timestamp:  i64 align(1),    // Server timestamp (ns)
    prev_hash:  u64 align(1),    // Previous message hash
    state_hash: u64 align(1),    // State verification (v5)
    data_len:   u32 align(1),    // Actual JSON size
    checksum:   u32 align(1),    // Integrity check
    json_data:  [1024]u8 align(1), // Payload
};
```

### Region

```zig
pub const Region = struct {
    cursor:          u32,           // Next free slot
    active_writers:  i32,           // Active writers (drain)
    entries:         []MessageEntry, // Array of messages
    base_ptr:        ?*anyopaque,    // Aligned memory
    allocated_slice: []u8,           // Backing memory
};
```

### ZigoDB

```zig
pub const ZigoDB = struct {
    rw_a:            Region,
    rw_b:            Region,
    region_bit:      u32,           // 0 = A, 1 = B
    search_pool:     SearchPool,
    temporal_index:  ?*TemporalIndex,
};
```

### SearchPool

```zig
pub const SearchPool = struct {
    segments:    [MAX_SEGMENTS]SearchSegment,
    bitmap:      [16]u64,          // 1024 bits
    mmap_ptr:    ?*anyopaque,
};
```

### TemporalIndex (v5)

```zig
pub const TimeCheckpoint = struct {
    timestamp:   i64,    // Unix timestamp (ns)
    chunk_id:    u64,
    offset:      u64,
    state_hash:  u64,
};
```

---

## 3. CGO Exported API

### Initialization

```c
ZigoDB* db_init(size_t messages_per_region);
void db_shutdown(ZigoDB* db);
```

### Writing

```c
MessageEntry* db_reserve_slot(ZigoDB* db);
void db_release_slot(Region* region);
uint32_t db_fill_entry(MessageEntry* entry, 
    uint32_t room_id, uint64_t uuid, int64_t timestamp,
    uint64_t prev_hash, uint64_t state_hash,
    const uint8_t* data, size_t data_len);
```

### Drain

```c
Region* db_switch_and_drain(ZigoDB* db);
bool db_validate_drain(Region* region);
bool db_needs_drain(ZigoDB* db);
```

### Persistence

```c
int db_dump_region(Region* region, const char* path);
int db_dump_index(Region* region, const char* path);
```

### Search

```c
void* search_reserve_segment(uint32_t chunk_id);
void* search_reserve_segment_mmap(const char* filename, uint32_t chunk_id);
void search_release_segment(uint32_t segment_id);
uint32_t search_query(void* segment, const char* json_query, 
    void* results, uint32_t max_results);
```

### Temporal 

```c
int temporal_create_checkpoint(ZigoDB* db, TimeCheckpoint* checkpoint);
int temporal_find_nearest(TemporalIndex* index, int64_t timestamp, 
    TimeCheckpoint* result);
int temporal_save(TemporalIndex* index, const char* path);
int temporal_load(TemporalIndex* index, const char* path);
```

---

## 4. Compilation

### Build Library

```bash
# Native (Windows)
make build

# Shared library
make build-shared

# Static library  
make build-static

# Cross-compilation
make build-linux      # Linux x64
make build-macos      # macOS x64
make build-linux-arm64
make build-macos-arm64
```

**Output:** `db/zigo_db.dll` (Windows), `db/zigo_db.lib`

### Run Tests

```bash
# All tests
make test

# Specific tests
make test-core      # Core data structures
make test-write     # Write path
make test-drain     # Drain mechanism
make test-persistence
make test-search
make test-temporal
make test-replication
```

---

## 5. Data Flow

```
┌─────────────────────────────────────────────────────┐
│                    Go Application                   │
└─────────────────────┬───────────────────────────────┘
                      │ CGO
                      ▼
┌─────────────────────────────────────────────────────┐
│                   Zigo Engine (Zig)                 │
├─────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌───────────────┐              │
│  │    RW_A      │    │    RW_B       │  (32MB each) │
│  │   (active)   │←──→│   (active)    │              │
│  └──────┬───────┘    └───────┬───────┘              │
│         │ region_bit (atomic)│                      │
│         ▼                    ▼                      │
│  ┌─────────────────────────────────────┐            │
│  │         Dumper Thread               │            │
│  │   (switch + dump + reset)           │            │
│  └──────────────────┬──────────────────┘            │
│                     ▼                               │
│  ┌──────────────┐  ┌─────────────┐                  │
│  │ chunk_xxx.rb │  │chunk_xxx.idx│                  │
│  └──────────────┘  └─────────────┘                  │
│                     │                               │
│                     ▼                               │
│  ┌─────────────────────────────────────┐            │
│  │        Search Pool (R_S)            │            │
│  │    1024 × 64KB segments             │            │
│  └─────────────────────────────────────┘            │
└─────────────────────────────────────────────────────┘
```

---

## 6. Features

### Core Features
- **Double Buffer** - RW_A / RW_B dual 16MB regions
- **Atomic Cursor** - Multi-core write safety via fetch-add
- **Drain Mechanism** - 90% capacity, 10k events, 30s triggers
- **Search Pool** - 1024 x 64KB segments for caching
- **Memory-mapped chunks** - Zero-copy file loading

### Data Features
- **Room ID** - Multi-room/chat support
- **State Hash** - Temporal verification and reconstruction
- **WriteLock** - Ticket mechanism for deterministic ordering
- **Hash Chain** - Entry verification

### Temporal Features
- **TimeCheckpoint** - Instant state rewind
- **TemporalIndex** - Checkpoint management
- **RewindTo** - Time-based rewind

### Persistence Features
- **ExportChunk** - Binary .rb export
- **ExportIndex** - Index export  
- **LoadChunk/LoadIndex** - Load saved data
- **SaveSnapshot/LoadSnapshot** - Full state backup

### Search Features
- **QueryChunk** - Message query with offset/count
- **Full-Text Search** - Snippet generation
- **SearchInRoom** - Room-filtered search
- **SearchWithPool** - Pool-based search

### Replication Features
- **Gossip Protocol** - P2P messaging between nodes
- **Gossip Signing** - HMAC-SHA256 verification
- **SyncRequest/SyncResponse** - Node synchronization
- **Global Event ID** - (timestamp << 16) | node_id

---

## 7. Constants

| Constant | Value | Description |
|----------|-------|-------------|
| REGION_SIZE | 32 MB | Size of each region |
| MAX_ENTRIES_PER_REGION | 15625 | Entries per region (~1072 bytes) |
| PAGE_SIZE | 64 KB | Page alignment |
| SEGMENT_SIZE | 64 KB | Search segment size |
| MAX_SEGMENTS | 1024 | Segments in search pool |
