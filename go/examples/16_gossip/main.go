package main

import (
	"encoding/json"
	"fmt"
	"time"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// Initialize ZigoDB
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Write messages to generate event IDs
	fmt.Println("=== Writing Messages to Generate Events ===")
	for i := 0; i < 5; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "event %d", "content": "test data"}`, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write: %v\n", err)
		} else {
			fmt.Printf("Wrote message, UUID: %d\n", uuid)
		}
	}

	// Generate unique event ID (T4.26)
	// Event ID format: (timestamp_ns << 16) | node_id
	fmt.Println("\n=== Generating Event IDs ===")
	for i := 0; i < 3; i++ {
		eventID := zigodb.Global().GenerateEventID()
		fmt.Printf("Generated event ID: %d (0x%x)\n", eventID, eventID)

		// Parse event ID components
		timestamp := int64(eventID >> 16)
		nodeID := eventID & 0xFFFF
		fmt.Printf("  -> Timestamp: %d, Node ID: %d\n", timestamp, nodeID)

		// Small delay to get different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Get last hash (T4.27)
	fmt.Println("\n=== Getting Last Hash ===")
	lastHash := zigodb.Global().GetLastHash()
	fmt.Printf("Last hash in chain: %d (0x%x)\n", lastHash, lastHash)

	// Create gossip message for replication (T4.28)
	fmt.Println("\n=== Creating Gossip Message ===")
	gossipData := []byte(`{"type": "replication", "origin": "node-1", "events": [1, 2, 3]}`)
	secretKey := []byte("gossip_secret_key")
	gossip, err := zigodb.Global().CreateGossip(gossipData, secretKey)
	if err != nil {
		fmt.Printf("Failed to create gossip: %v\n", err)
	} else {
		fmt.Printf("Gossip message created:\n")
		fmt.Printf("  Event ID: %d\n", gossip.EventID)
		fmt.Printf("  Timestamp: %d\n", gossip.Timestamp)
		fmt.Printf("  Data: %s\n", string(gossip.Data))

		// Serialize gossip for network transmission
		gossipJSON, err := json.Marshal(gossip)
		if err != nil {
			fmt.Printf("Failed to serialize gossip: %v\n", err)
		} else {
			fmt.Printf("  Serialized: %s\n", string(gossipJSON))
		}
	}

	// Simulate gossip propagation between nodes
	fmt.Println("\n=== Simulating Gossip Propagation ===")

	// Simulate node 1 creates gossip
	node1Gossip, _ := zigodb.Global().CreateGossip([]byte(`{"from": "node-1", "seq": 1}`), secretKey)
	fmt.Printf("Node 1 sends: EventID=%d\n", node1Gossip.EventID)

	// Simulate node 2 creates gossip
	node2Gossip, _ := zigodb.Global().CreateGossip([]byte(`{"from": "node-2", "seq": 2}`), secretKey)
	fmt.Printf("Node 2 sends: EventID=%d\n", node2Gossip.EventID)

	// Simulate node 3 creates gossip
	node3Gossip, _ := zigodb.Global().CreateGossip([]byte(`{"from": "node-3", "seq": 3}`), secretKey)
	fmt.Printf("Node 3 sends: EventID=%d\n", node3Gossip.EventID)

	// Verify chain integrity
	fmt.Println("\n=== Chain Verification ===")
	chainValid := zigodb.Global().VerifyChain()
	fmt.Printf("Chain integrity: %v\n", map[bool]string{true: "VALID", false: "INVALID"}[chainValid])

	// Compute state hash
	stateHash := zigodb.Global().ComputeStateHash()
	fmt.Printf("Current state hash: %d (0x%x)\n", stateHash, stateHash)

	// Export chunk
	fmt.Println("\n=== Exporting Chunk ===")
	chunkFile := "storage/chunks/gossip_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export chunk: %v", err))
	}
	fmt.Printf("Chunk exported to %s\n", chunkFile)

	fmt.Println("\n=== Gossip/Replication tutorial completed successfully! ===")
	fmt.Println("Gossip protocol enables eventual consistency across distributed nodes.")
	fmt.Println("Each node generates unique event IDs based on timestamp and node ID.")
}
