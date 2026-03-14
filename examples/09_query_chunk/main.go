package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	// First write some messages to create a chunk
	fmt.Println("=== Writing messages ===")
	for i := 0; i < 20; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "testuser"}`, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write message %d: %v\n", i, err)
		} else {
			fmt.Printf("Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Export the chunk for loading
	chunkFile := "storage/chunks/chat_001.rb"
	indexFile := "storage/chunks/chat_001.index"

	fmt.Println("\n=== Exporting chunk ===")
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export chunk: %v", err))
	}
	fmt.Printf("Exported chunk to %s\n", chunkFile)

	if err := zigodb.Global().ExportIndex(indexFile); err != nil {
		panic(fmt.Sprintf("Failed to export index: %v", err))
	}
	fmt.Printf("Exported index to %s\n", indexFile)

	// Reinitialize to clear memory
	zigodb.Shutdown()
	err = zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to reinitialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Load the chunk
	fmt.Println("\n=== Loading chunk ===")
	if err := zigodb.Global().LoadChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to load chunk: %v", err))
	}
	fmt.Printf("Loaded %d messages\n", len(zigodb.Global().LoadedChunkEntries))

	// Query first 10 messages (T5.1)
	fmt.Println("\n=== QueryChunk: first 10 messages ===")
	results, err := zigodb.Global().QueryChunk(0, 10)
	if err != nil {
		panic(fmt.Sprintf("QueryChunk failed: %v", err))
	}

	fmt.Printf("Found %d messages:\n", len(results))
	for i, data := range results {
		fmt.Printf("  [%d] %s\n", i, string(data))
	}

	// Query with metadata (T5.2)
	fmt.Println("\n=== QueryChunkWithMeta: first 5 messages with metadata ===")
	metaResults, err := zigodb.Global().QueryChunkWithMeta(0, 5)
	if err != nil {
		panic(fmt.Sprintf("QueryChunkWithMeta failed: %v", err))
	}

	fmt.Printf("Found %d messages:\n", len(metaResults))
	for i, entry := range metaResults {
		fmt.Printf("  [%d] UUID=%d, RoomID=%d, Time=%d, Data=%s\n",
			i, entry.UUID, entry.RoomID, entry.Timestamp, string(entry.Data))
	}

	// Query with offset
	fmt.Println("\n=== QueryChunk: offset 10, count 5 ===")
	results, err = zigodb.Global().QueryChunk(10, 5)
	if err != nil {
		panic(fmt.Sprintf("QueryChunk failed: %v", err))
	}

	fmt.Printf("Found %d messages:\n", len(results))
	for i, data := range results {
		fmt.Printf("  [%d] %s\n", i, string(data))
	}

	fmt.Println("\n=== Query tutorial completed successfully! ===")
}
