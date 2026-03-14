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

	// Write messages with proper hash chain
	fmt.Println("=== Writing Messages with Hash Chain ===")
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "seq": %d}`, i, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write message %d: %v\n", i, err)
		} else {
			fmt.Printf("Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Export chunk
	chunkFile := "storage/chunks/chain_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export chunk: %v", err))
	}
	fmt.Printf("Exported chunk to %s\n", chunkFile)

	// Reinitialize to load the chunk
	zigodb.Shutdown()
	err = zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to reinitialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Load the chunk
	fmt.Println("\n=== Loading Chunk ===")
	if err := zigodb.Global().LoadChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to load chunk: %v", err))
	}
	fmt.Printf("Loaded %d messages\n", len(zigodb.Global().LoadedChunkEntries))

	// Verify the hash chain (T5.16)
	fmt.Println("\n=== Verifying Hash Chain ===")
	isValid := zigodb.Global().VerifyChain()
	fmt.Printf("Chain verification: %v\n", isValid)

	// Compute state hash (T5.17)
	fmt.Println("\n=== Computing State Hash ===")
	stateHash := zigodb.Global().ComputeStateHash()
	fmt.Printf("State hash: %d\n", stateHash)

	// Show loaded entries for inspection
	fmt.Println("\n=== Loaded Entries ===")
	for i, entry := range zigodb.Global().LoadedChunkEntries {
		fmt.Printf("[%d] UUID=%d, PrevHash=%d, Data=%s\n",
			i, entry.UUID, entry.PrevHash, string(entry.Data[:30]))
	}

	// Test chain verification on loaded entries
	fmt.Println("\n=== Testing Chain Verification ===")
	if len(zigodb.Global().LoadedChunkEntries) > 1 {
		for i := 1; i < len(zigodb.Global().LoadedChunkEntries); i++ {
			prev := &zigodb.Global().LoadedChunkEntries[i-1]
			curr := &zigodb.Global().LoadedChunkEntries[i]
			fmt.Printf("Entry %d: PrevHash=%d, Expected=%d - %s\n",
				i, curr.PrevHash, hashData(prev.Data),
				map[bool]string{true: "OK", false: "FAIL"}[curr.PrevHash == hashData(prev.Data)])
		}
	}

	fmt.Println("\n=== Chain Verification tutorial completed successfully! ===")
}

// hashData computes a hash for verification
func hashData(data []byte) uint64 {
	var hash uint64 = 5381
	for _, b := range data {
		hash = hash*33 + uint64(b)
	}
	return hash
}
