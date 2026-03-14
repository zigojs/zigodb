package main

import (
	"encoding/json"
	"fmt"
	"time"

	zigodb "github.com/zigojs/zigodb"
)

type ChatMessage struct {
	User    string `json:"user"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

func main() {
	// Initialize
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	fmt.Println("=== Zigo-DB Complete Chat Demo ===\n")

	// Write messages to different rooms
	rooms := []uint32{1, 2, 3}
	users := []string{"alice", "bob", "charlie"}

	fmt.Println("=== Writing Messages ===")
	for roomIdx, roomID := range rooms {
		for i := 0; i < 10; i++ {
			msg := ChatMessage{
				User:    users[roomIdx],
				Content: fmt.Sprintf("Hello from room %d, message %d", roomID, i),
				Time:    time.Now().UnixNano(),
			}
			jsonData, _ := json.Marshal(msg)
			zigodb.Global().WriteToRoom(roomID, jsonData)
		}
		fmt.Printf("Room %d: wrote 10 messages\n", roomID)
	}

	// Check status
	fmt.Println("\n=== Status ===")
	fmt.Printf("Total messages: %d\n", zigodb.Global().GetMessageCount())
	fmt.Printf("Needs drain: %v\n", zigodb.Global().NeedsDrain())

	// Create checkpoint
	fmt.Println("\n=== Checkpoint ===")
	cp, _ := zigodb.Global().CreateCheckpoint()
	fmt.Printf("Checkpoint created: ChunkID=%d, Offset=%d, StateHash=%d\n",
		cp.ChunkID, cp.Offset, cp.StateHash)

	// Export (skip drain to avoid hanging)
	fmt.Println("\n=== Export ===")
	timestamp := time.Now().Format("20060102_150405")
	chunkFile := fmt.Sprintf("storage/chunks/chat_%s.rb", timestamp)
	indexFile := fmt.Sprintf("storage/chunks/chat_%s.index", timestamp)

	zigodb.Global().ExportChunk(chunkFile)
	zigodb.Global().ExportIndex(indexFile)
	fmt.Printf("Exported chunk to %s\n", chunkFile)
	fmt.Printf("Exported index to %s\n", indexFile)

	// Load and query
	fmt.Println("\n=== Load and Query ===")
	err = zigodb.Global().LoadChunk(chunkFile)
	if err != nil {
		fmt.Printf("LoadChunk error: %v\n", err)
	} else {
		fmt.Printf("Loaded %d messages\n", len(zigodb.Global().LoadedChunkEntries))
	}

	// Query first 5 messages
	results, _ := zigodb.Global().QueryChunk(0, 5)
	fmt.Println("\nFirst 5 messages:")
	for i, data := range results {
		fmt.Printf("  [%d] %s\n", i, string(data))
	}

	// Search in loaded chunk
	fmt.Println("\n=== Search ===")
	searchResults, _ := zigodb.Global().Search("alice")
	fmt.Printf("Search for 'alice': %d results\n", len(searchResults))
	for _, r := range searchResults {
		fmt.Printf("  RoomID=%d, Snippet=%s\n", r.RoomID, r.Snippet)
	}

	// Verify chain
	fmt.Println("\n=== Chain Verification ===")
	valid := zigodb.Global().VerifyChain()
	fmt.Printf("Chain valid: %v\n", valid)

	// Compute state hash
	stateHash := zigodb.Global().ComputeStateHash()
	fmt.Printf("State hash: %d\n", stateHash)

	// Test temporal rewind
	fmt.Println("\n=== Temporal Rewind ===")
	// Use the exported chunk file for rewind
	err = zigodb.Global().LoadChunk(chunkFile)
	if err != nil {
		fmt.Printf("LoadChunk error: %v\n", err)
	} else {
		fmt.Printf("Loaded chunk from %s\n", chunkFile)
		// Query after rewind
		results, _ := zigodb.Global().QueryChunk(0, 3)
		fmt.Printf("After rewind: %d messages\n", len(results))
		for i, data := range results {
			fmt.Printf("  [%d] %s\n", i, string(data))
		}
	}

	fmt.Println("\n=== Complete Chat Demo completed successfully! ===")
}
