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

	// First write some messages with different content to test search
	fmt.Println("=== Writing messages ===")
	messages := []struct {
		roomID uint32
		data   string
	}{
		{0, `{"msg": "hello world", "user": "alice"}`},
		{0, `{"msg": "good morning", "user": "bob"}`},
		{0, `{"msg": "hello there", "user": "charlie"}`},
		{1, `{"msg": "hello from room 1", "user": "dave"}`},
		{1, `{"msg": "testing search", "user": "eve"}`},
		{0, `{"msg": "another hello message", "user": "frank"}`},
		{1, `{"msg": "hello everyone in room 1", "user": "grace"}`},
		{0, `{"msg": "search is working", "user": "henry"}`},
		{0, `{"msg": "found it", "user": "iris"}`},
		{1, `{"msg": "room 1 test message", "user": "jack"}`},
	}

	for i, msg := range messages {
		uuid, err := zigodb.Global().WriteToRoom(msg.roomID, []byte(msg.data))
		if err != nil {
			fmt.Printf("Failed to write message %d: %v\n", i, err)
		} else {
			fmt.Printf("Wrote message %d to room %d, UUID: %d\n", i, msg.roomID, uuid)
		}
	}

	// Export the chunk for loading
	chunkFile := "storage/chunks/search_test.rb"
	indexFile := "storage/chunks/search_test.index"

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

	// Basic search (T5.4)
	fmt.Println("\n=== Search: 'hello' ===")
	results, err := zigodb.Global().Search("hello")
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	} else {
		fmt.Printf("Found %d results:\n", len(results))
		for _, r := range results {
			fmt.Printf("  RoomID=%d, Time=%d, Snippet=%s\n",
				r.RoomID, r.Timestamp, r.Snippet)
		}
	}

	// Paginated search (T5.5)
	fmt.Println("\n=== Search Paged: 'hello' (page 1, 5 per page) ===")
	pagedResults, err := zigodb.Global().SearchPaged("hello", 1, 5)
	if err != nil {
		fmt.Printf("SearchPaged error: %v\n", err)
	} else {
		fmt.Printf("Page 1: %d results\n", len(pagedResults))
		for i, r := range pagedResults {
			fmt.Printf("  [%d] RoomID=%d, Snippet=%s\n", i, r.RoomID, r.Snippet)
		}
	}

	// Search in specific room (T5.6)
	fmt.Println("\n=== Search in Room 1: 'hello' ===")
	roomResults, err := zigodb.Global().SearchInRoom("hello", 1)
	if err != nil {
		fmt.Printf("SearchInRoom error: %v\n", err)
	} else {
		fmt.Printf("Found %d results in room 1\n", len(roomResults))
		for _, r := range roomResults {
			fmt.Printf("  Offset=%d, Snippet=%s\n", r.Offset, r.Snippet)
		}
	}

	// Search for another term
	fmt.Println("\n=== Search: 'test' ===")
	results, err = zigodb.Global().Search("test")
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	} else {
		fmt.Printf("Found %d results:\n", len(results))
		for _, r := range results {
			fmt.Printf("  RoomID=%d, Snippet=%s\n", r.RoomID, r.Snippet)
		}
	}

	// Search with no results
	fmt.Println("\n=== Search: 'nonexistent' ===")
	results, err = zigodb.Global().Search("nonexistent")
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	} else {
		fmt.Printf("Found %d results\n", len(results))
	}

	fmt.Println("\n=== Search tutorial completed successfully! ===")
}
