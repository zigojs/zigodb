# ZigoDB Go API

## Installation

```bash
go get github.com/zigojs/zigodb
```

## Initialization

```go
import zigodb "github.com/zigojs/zigodb"

func main() {
    // Initialize database
    err := zigodb.Init()
    if err != nil {
        panic(err)
    }
    defer zigodb.Shutdown()
    
    // ...
}
```

## API Reference

### Init()

Initializes the ZigoDB engine. Must be called before any other operations.

```go
err := zigodb.Init()
```

### Shutdown()

Gracefully shuts down the database.

```go
zigodb.Shutdown()
```

### Global() *ZigoDB

Returns the global ZigoDB instance.

```go
db := zigodb.Global()
```

### Write(data []byte) (uint64, error)

Writes data to the database. Returns the UUID of the message.

```go
uuid, err := db.Write([]byte(`{"msg": "hello"}`))
```

### GetMessageCount() uint32

Returns the current number of messages in the active region.

```go
count := db.GetMessageCount()
```

### NeedsDrain() bool

Returns true if the active region needs to be drained (90% capacity).

```go
if db.NeedsDrain() {
    // Trigger drain
}
```

### TriggerDrain() error

Switches to the inactive region and prepares the old region for export.

```go
err := db.TriggerDrain()
```

### ExportChunk(filename string) error

Exports the current region to a binary .rb file.

```go
err := db.ExportChunk("storage/chunks/chat_001.rb")
```

### ExportIndex(filename string) error

Exports the index to a .index file.

```go
err := db.ExportIndex("storage/chunks/chat_001.index")
```

### ReadLast() ([]byte, error)

Reads the last message directly from memory.

```go
data, err := db.ReadLast()
```

### ReadLastFromRoom(roomID uint32) ([]byte, error)

Reads the last message from a specific room.

```go
data, err := db.ReadLastFromRoom(roomID)
```

### LoadChunk(filename string) error

Loads a chunk file into memory.

```go
err := db.LoadChunk("storage/chunks/chat_001.rb")
```

### LoadIndex(filename string) error

Loads an index file into memory.

```go
err := db.LoadIndex("storage/chunks/chat_001.index")
```

### QueryChunk(offset, count int) ([]MessageEntryData, error)

Queries messages from loaded chunks with offset/count.

```go
entries, err := db.QueryChunk(0, 100)
```

### QueryChunkWithMeta(offset, count int) ([]MessageEntryData, error)

Queries messages with full metadata (UUID, RoomID, Timestamp).

```go
entries, err := db.QueryChunkWithMeta(0, 100)
```

### Search(query string) ([]SearchResult, error)

Full-text search in loaded chunks with snippet generation.

```go
results, err := db.Search("hello world")
```

### SearchPaged(query string, page, pageSize int) ([]SearchResult, error)

Paginated search results.

```go
results, err := db.SearchPaged("hello", 1, 10)
```

### SearchInRoom(roomID uint32, query string) ([]SearchResult, error)

Room-filtered search.

```go
results, err := db.SearchInRoom(1, "hello")
```

### MmapChunk(filename string) error

Memory-mapped chunk loading (zero-copy).

```go
err := db.MmapChunk("storage/chunks/chat_001.rb")
```

### MunmapChunk()

Unmaps the chunk file.

```go
db.MunmapChunk()
```

### SaveSnapshot(filename string) error

Saves database snapshot to file.

```go
err := db.SaveSnapshot("snapshot.bin")
```

### LoadSnapshot(filename string) error

Loads database snapshot from file.

```go
err := db.LoadSnapshot("snapshot.bin")
```

---

## Temporal Layer (v5)

### TemporalIndex

Temporal index for time-based lookups.

```go
temporal := db.GetTemporalIndex()
```

### FindNearest(timestamp int64) (*TimeCheckpoint, error)

Finds nearest checkpoint to a timestamp.

```go
checkpoint, err := temporal.FindNearest(1234567890)
```

### RewindTo(timestamp int64) error

Time-based rewind using temporal checkpoints.

```go
err := db.RewindTo(1234567890)
```

### SaveTemporalIndex(filename string) error

Saves temporal index to JSON file.

```go
err := db.SaveTemporalIndex("temporal.json")
```

### LoadTemporalIndex(filename string) error

Loads temporal index from JSON file.

```go
err := db.LoadTemporalIndex("temporal.json")
```

### CreateCheckpoint() (*TimeCheckpoint, error)

Creates temporal checkpoint with actual state hash from Zig.

```go
checkpoint, err := db.CreateCheckpoint()
```

---

## Search Pool (v5)

### SearchWithPool(query string) ([]SearchResult, error)

Search using Search Pool across chunks.

```go
results, err := db.SearchWithPool("hello")
```

### VerifyChain() (bool, error)

Hash chain verification for loaded chunks.

```go
valid, err := db.VerifyChain()
```

### ComputeStateHash() (uint64, error)

Compute state hash from entries.

```go
hash, err := db.ComputeStateHash()
```

---

## Room Support (v4)

### WriteRoom(roomID uint32, data []byte) (uint64, error)

Writes data to a specific room.

```go
uuid, err := db.WriteRoom(1, []byte(`{"msg": "hello"}`))
```

---

## Examples

### Example 1: Initialization

```go
package main

import "fmt"
import zigodb "github.com/zigojs/zigodb"

func main() {
    err := zigodb.Init()
    if err != nil {
        panic(err)
    }
    defer zigodb.Shutdown()
    
    fmt.Println("ZigoDB initialized!")
}
```

### Example 2: Writing

```go
package main

import "fmt"
import zigodb "github.com/zigojs/zigodb"

func main() {
    zigodb.Init()
    defer zigodb.Shutdown()
    
    data := []byte(`{"msg": "hello", "user": "test"}`)
    uuid, err := zigodb.Global().Write(data)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Message written! UUID: %d\n", uuid)
}
```

### Example 3: Query and Search

```go
package main

import (
    "fmt"
    zigodb "github.com/zigojs/zigodb"
)

func main() {
    zigodb.Init()
    defer zigodb.Shutdown()
    
    // Load chunk
    zigodb.Global().LoadChunk("chunk.rb")
    
    // Query
    entries, _ := zigodb.Global().QueryChunk(0, 100)
    for _, e := range entries {
        fmt.Printf("Message: %s\n", string(e.Data))
    }
    
    // Search
    results, _ := zigodb.Global().Search("hello")
    for _, r := range results {
        fmt.Printf("Found: %s\n", r.Snippet)
    }
}
```

### Example 4: Temporal Rewind

```go
package main

import (
    "fmt"
    zigodb "github.com/zigojs/zigodb"
)

func main() {
    zigodb.Init()
    defer zigodb.Shutdown()
    
    // Create checkpoint
    cp, _ := zigodb.Global().CreateCheckpoint()
    fmt.Printf("Checkpoint: %+v\n", cp)
    
    // Find nearest to timestamp
    nearest, _ := zigodb.Global().GetTemporalIndex().FindNearest(cp.Timestamp)
    fmt.Printf("Nearest: %+v\n", nearest)
    
    // Rewind
    zigodb.Global().RewindTo(cp.Timestamp)
}
```

## Running Examples

```bash
cd go/examples/01_init
go run main.go
```
