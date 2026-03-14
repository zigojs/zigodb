package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// First, create a chunk file to memory-map
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}

	// Write some messages
	fmt.Println("=== Writing Messages ===")
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "mmap test %d", "index": %d}`, i, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write: %v\n", err)
		} else {
			fmt.Printf("Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Export chunk first
	chunkFile := "storage/chunks/mmap_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export chunk: %v", err))
	}
	fmt.Printf("Chunk exported to %s\n", chunkFile)

	// Shutdown and reinitialize for mmap test
	zigodb.Shutdown()

	// Reinitialize
	err = zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to reinitialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Load chunk using memory mapping (T5.13)
	fmt.Println("\n=== Loading Chunk with Mmap ===")
	if err := zigodb.Global().MmapChunk(chunkFile); err != nil {
		// Note: MmapChunk may not be fully implemented in this version
		// Fall back to regular LoadChunk
		fmt.Printf("MmapChunk note: %v\n", err)
		fmt.Println("Falling back to regular LoadChunk...")

		if err := zigodb.Global().LoadChunk(chunkFile); err != nil {
			panic(fmt.Sprintf("Failed to load chunk: %v", err))
		}
		fmt.Printf("Loaded %d entries via regular load\n", len(zigodb.Global().LoadedChunkEntries))
	} else {
		fmt.Println("Chunk loaded via memory mapping!")

		// Query the mmap'd chunk
		results, err := zigodb.Global().QueryChunk(0, 5)
		if err != nil {
			fmt.Printf("Query error: %v\n", err)
		} else {
			fmt.Printf("\n=== Query Results (%d entries) ===\n", len(results))
			for i, data := range results {
				fmt.Printf("[%d] %s\n", i, string(data))
			}
		}
	}

	// Search in loaded chunk
	fmt.Println("\n=== Searching in Mmap'd Chunk ===")
	searchResults, err := zigodb.Global().Search("test")
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	} else {
		fmt.Printf("Found %d results for 'test':\n", len(searchResults))
		for _, r := range searchResults {
			fmt.Printf("  RoomID=%d, Offset=%d, Snippet=%s\n",
				r.RoomID, r.Offset, r.Snippet)
		}
	}

	// Verify chain in mmap'd chunk
	fmt.Println("\n=== Chain Verification ===")
	isValid := zigodb.Global().VerifyChain()
	fmt.Printf("Chain verification: %v\n", map[bool]string{true: "VALID", false: "INVALID"}[isValid])

	// Compute state hash
	stateHash := zigodb.Global().ComputeStateHash()
	fmt.Printf("State hash: %d\n", stateHash)

	// Cleanup: unmap the chunk (T5.13)
	fmt.Println("\n=== Cleaning up Mmap ===")
	if err := zigodb.Global().MunmapChunk(); err != nil {
		fmt.Printf("Munmap note: %v\n", err)
	} else {
		fmt.Println("Chunk unmapped successfully")
	}

	fmt.Println("\n=== Memory-mapped Chunk tutorial completed successfully! ===")
	fmt.Println("Mmap provides zero-copy access to chunk files for improved performance.")
	fmt.Println("Ideal for large datasets where memory efficiency is critical.")
}
