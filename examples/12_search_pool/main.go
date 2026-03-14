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

	// Write messages for multiple chunks
	fmt.Println("=== Creating Test Data ===")
	for chunk := 0; chunk < 3; chunk++ {
		for i := 0; i < 20; i++ {
			data := []byte(fmt.Sprintf(`{"msg": "test message %d in chunk %d", "id": %d}`, i, chunk, chunk*20+i))
			zigodb.Global().Write(data)
		}
		fmt.Printf("Wrote 20 messages for chunk %d\n", chunk)

		// Export chunk
		filename := fmt.Sprintf("storage/chunks/test_chunk_%d.rb", chunk)
		if err := zigodb.Global().ExportChunk(filename); err != nil {
			fmt.Printf("Export error for chunk %d: %v\n", chunk, err)
		} else {
			fmt.Printf("Exported %s\n", filename)
		}
	}

	// Search across all chunks using search pool (T5.14)
	fmt.Println("\n=== Searching Across Chunks ===")
	chunks := []string{
		"storage/chunks/test_chunk_0.rb",
		"storage/chunks/test_chunk_1.rb",
		"storage/chunks/test_chunk_2.rb",
	}

	for _, chunk := range chunks {
		fmt.Printf("\nSearching in %s:\n", chunk)
		results, err := zigodb.Global().SearchWithPool("test", chunk)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		fmt.Printf("  Found %d results\n", len(results))
		for _, r := range results {
			snippet := r.Snippet
			if len(snippet) > 50 {
				snippet = snippet[:50]
			}
			fmt.Printf("    RoomID=%d, Snippet=%s\n", r.RoomID, snippet)
		}
	}

	// Test search pool with different query
	fmt.Println("\n=== Searching for 'message' ===")
	for _, chunk := range chunks {
		results, err := zigodb.Global().SearchWithPool("message", chunk)
		if err != nil {
			fmt.Printf("Error searching %s: %v\n", chunk, err)
			continue
		}
		fmt.Printf("%s: %d results\n", chunk, len(results))
	}

	fmt.Println("\n=== Search Pool tutorial completed successfully! ===")
}
