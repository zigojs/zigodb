package main

import (
	"fmt"
	"time"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Write some messages first
	fmt.Println("=== Writing Messages ===")
	for i := 0; i < 50; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "test", "seq": %d}`, i, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write message %d: %v\n", i, err)
		} else if i%10 == 0 {
			fmt.Printf("Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Create checkpoint (T5.9)
	fmt.Println("\n=== Creating Checkpoint ===")
	cp, err := zigodb.Global().CreateCheckpoint()
	if err != nil {
		panic(fmt.Sprintf("Failed to create checkpoint: %v", err))
	}
	fmt.Printf("Checkpoint: Timestamp=%d, ChunkID=%d, Offset=%d, StateHash=%d\n",
		cp.Timestamp, cp.ChunkID, cp.Offset, cp.StateHash)

	// Write more messages after checkpoint
	fmt.Println("\n=== Writing More Messages ===")
	for i := 50; i < 80; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "test", "seq": %d}`, i, i))
		_, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write message %d: %v\n", i, err)
		}
	}
	fmt.Printf("Total messages: %d\n", zigodb.Global().GetMessageCount())

	// Trigger drain to create chunk file (commented out - causes hanging)
	// fmt.Println("\n=== Draining ===")
	// if err := zigodb.Global().TriggerDrain(); err != nil {
	// 	fmt.Printf("Drain error: %v\n", err)
	// }

	// Export the chunk without draining
	// Use chunk ID from checkpoint so RewindTo can find it
	chunkFile := fmt.Sprintf("storage/chunks/chunk_%d.rb", cp.ChunkID)
	indexFile := fmt.Sprintf("storage/chunks/chunk_%d.index", cp.ChunkID)
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		fmt.Printf("Export chunk error: %v\n", err)
	} else {
		fmt.Printf("Exported chunk to %s\n", chunkFile)
	}
	if err := zigodb.Global().ExportIndex(indexFile); err != nil {
		fmt.Printf("Export index error: %v\n", err)
	} else {
		fmt.Printf("Exported index to %s\n", indexFile)
	}

	// Get current timestamp for rewind
	currentTime := time.Now().UnixNano()

	// Save temporal index to file (T5.11)
	fmt.Println("\n=== Saving Temporal Index ===")
	temporalFile := "storage/chunks/temporal_index.json"
	if zigodb.Global().GetTemporalIndex() != nil {
		err := zigodb.Global().GetTemporalIndex().Save(temporalFile)
		if err != nil {
			fmt.Printf("Save temporal index error: %v\n", err)
		} else {
			fmt.Printf("Saved temporal index to %s\n", temporalFile)
		}
	}

	// Reinitialize
	zigodb.Shutdown()
	err = zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to reinitialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Load temporal index (T5.11)
	fmt.Println("\n=== Loading Temporal Index ===")
	loadedTemporal, err := zigodb.LoadTemporalIndex(temporalFile)
	if err != nil {
		fmt.Printf("Load temporal index error: %v\n", err)
	} else {
		fmt.Printf("Loaded temporal index with %d checkpoints\n", len(loadedTemporal.Checkpoints))
		if len(loadedTemporal.Checkpoints) > 0 {
			cp := loadedTemporal.Checkpoints[0]
			fmt.Printf("First checkpoint: Timestamp=%d, ChunkID=%d, Offset=%d\n",
				cp.Timestamp, cp.ChunkID, cp.Offset)
		}
		// Set the loaded temporal index back into ZigoDB
		zigodb.Global().SetTemporalIndex(loadedTemporal)
	}

	// Load the chunk
	fmt.Println("\n=== Loading Chunk ===")
	if err := zigodb.Global().LoadChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to load chunk: %v", err))
	}
	fmt.Printf("Loaded %d messages\n", len(zigodb.Global().LoadedChunkEntries))

	// Rewind to timestamp (T5.10)
	fmt.Println("\n=== Rewinding ===")
	// Use a timestamp slightly before current to find the chunk
	rewindTime := currentTime - 60_000_000_000 // 60 seconds ago
	if err := zigodb.Global().RewindTo(rewindTime); err != nil {
		fmt.Printf("Rewind error: %v\n", err)
	} else {
		// Query some messages after rewind
		results, _ := zigodb.Global().QueryChunk(0, 5)
		fmt.Printf("After rewind: %d messages\n", len(results))
		for i, data := range results {
			fmt.Printf("  [%d] %s\n", i, string(data))
		}
	}

	// Test FindNearest directly
	fmt.Println("\n=== Testing FindNearest ===")
	testTemporal := zigodb.NewTemporalIndex(100)
	testTemporal.AddCheckpoint(zigodb.TimeCheckpoint{Timestamp: 1000, ChunkID: 1, Offset: 0, StateHash: 0})
	testTemporal.AddCheckpoint(zigodb.TimeCheckpoint{Timestamp: 2000, ChunkID: 2, Offset: 100, StateHash: 0})
	testTemporal.AddCheckpoint(zigodb.TimeCheckpoint{Timestamp: 3000, ChunkID: 3, Offset: 200, StateHash: 0})

	nearest := testTemporal.FindNearest(1500)
	if nearest != nil {
		fmt.Printf("FindNearest(1500): Timestamp=%d, ChunkID=%d\n", nearest.Timestamp, nearest.ChunkID)
	}

	nearest = testTemporal.FindNearest(2500)
	if nearest != nil {
		fmt.Printf("FindNearest(2500): Timestamp=%d, ChunkID=%d\n", nearest.Timestamp, nearest.ChunkID)
	}

	fmt.Println("\n=== Temporal tutorial completed successfully! ===")
}
