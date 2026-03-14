package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

// MessageEntry represents a message in the chunk file
type MessageEntry struct {
	RoomID    uint32
	UUID      uint64
	Timestamp int64
	PrevHash  uint64
	StateHash uint64
	DataLen   uint32
	Checksum  uint32
	Data      []byte
}

func main() {
	// Initialize ZigoDB for LoadChunk test
	err := zigodb.Init()
	if err != nil {
		fmt.Printf("Failed to initialize ZigoDB: %v\n", err)
	}
	defer zigodb.Shutdown()

	// First, ensure we have a chunk to load
	// Run example 07 first to create the chunk

	chunkFile := "storage/chunks/chat_001.rb"
	indexFile := "storage/chunks/chat_001.index"

	// Load and read the index file using bridge
	fmt.Println("=== Loading Index ===")
	if err := zigodb.Global().LoadIndex(indexFile); err != nil {
		panic(fmt.Sprintf("Failed to load index: %v", err))
	}
	fmt.Printf("Loaded %d index entries\n", len(zigodb.Global().LoadedIndexEntries))

	// Print index entries
	for i, entry := range zigodb.Global().LoadedIndexEntries {
		fmt.Printf("Index %d: Offset=%d, UUID=%d, Timestamp=%d, RoomID=%d, DataLen=%d\n",
			i, entry.Offset, entry.UUID, entry.Timestamp, entry.RoomID, entry.DataLen)
	}

	// Load and read the chunk file
	fmt.Println("\n=== Loading Chunk ===")
	if err := zigodb.Global().LoadChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to load chunk: %v", err))
	}
	fmt.Printf("Loaded %d messages\n", len(zigodb.Global().LoadedChunkEntries))

	// Print messages
	for i, msg := range zigodb.Global().LoadedChunkEntries {
		fmt.Printf("Message %d: UUID=%d, RoomID=%d, Data=%s\n",
			i, msg.UUID, msg.RoomID, string(msg.Data))
	}

	// LoadChunk already tested above
	fmt.Println("\n=== Testing LoadChunk ===")
	fmt.Println("LoadChunk successful!")
}
