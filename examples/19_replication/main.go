package main

import (
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

	// Create gossip manager
	fmt.Println("=== Creating Gossip Manager ===")
	gossipMgr := zigodb.NewGossipManager("node_1")
	gossipMgr.AddPeer("node_2", "192.168.1.10:8080")
	gossipMgr.AddPeer("node_3", "192.168.1.11:8080")
	fmt.Printf("Gossip manager created for node_1 with 2 peers\n")

	// List peers
	peers := gossipMgr.GetPeers()
	fmt.Printf("Peers: %d\n", len(peers))
	for _, peer := range peers {
		fmt.Printf("  - %s at %s\n", peer.NodeID, peer.Address)
	}

	// Write some messages
	fmt.Println("\n=== Writing Messages ===")
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf(`{"event": "update", "id": %d, "data": "test"}`, i))
		uuid, err := zigodb.Global().Write(data)
		if err == nil {
			fmt.Printf("Wrote message %d (UUID: %d)\n", i, uuid)
		}
	}

	// Get last hash for gossip
	lastHash := zigodb.Global().GetLastHash()
	fmt.Printf("\nLast hash: %d\n", lastHash)

	// Create gossip message
	fmt.Println("\n=== Creating Gossip Message ===")
	secretKey := []byte("my_secret_key_12345")
	gossipMsg, err := zigodb.Global().CreateGossip(
		[]byte(`{"type": "state_update", "node": "node_1"}`),
		secretKey,
	)
	if err == nil {
		fmt.Printf("Gossip message created:\n")
		fmt.Printf("  EventID: %d\n", gossipMsg.EventID)
		fmt.Printf("  Timestamp: %d\n", gossipMsg.Timestamp)
		fmt.Printf("  NodeID: %d\n", gossipMsg.NodeID)
		fmt.Printf("  Data: %s\n", string(gossipMsg.Data))
		fmt.Printf("  Signature: %x (len=%d)\n", gossipMsg.Signature, len(gossipMsg.Signature))

		// Verify the signature
		valid := zigodb.VerifyGossip(gossipMsg.Data, gossipMsg.Signature, secretKey)
		fmt.Printf("  Signature valid: %v\n", valid)
	}

	// Broadcast to peers
	fmt.Println("\n=== Broadcasting to Peers ===")
	gossipMgr.Broadcast(gossipMsg)

	// Simulate message synchronization
	fmt.Println("\n=== Testing Message Synchronization ===")

	// Node 1: Create sync request from Node 2's perspective
	syncReq := zigodb.SyncRequest{
		FromNode:  "node_2",
		LastHash:  lastHash,
		Timestamp: time.Now().UnixNano() - 1000000000, // 1 second ago
	}

	// Node 1 handles the sync request
	syncResp, err := zigodb.Global().HandleSync(syncReq)
	if err != nil {
		fmt.Printf("Sync error: %v\n", err)
	} else {
		fmt.Printf("Sync response: %d entries, LastHash: %d\n",
			len(syncResp.Entries), syncResp.LastHash)
	}

	// Simulate applying sync (in real scenario, this would be from another node)
	fmt.Println("\n=== Applying Sync (Simulated) ===")
	if len(syncResp.Entries) > 0 {
		fmt.Printf("Would apply %d entries from peer\n", len(syncResp.Entries))
	} else {
		fmt.Println("No entries to sync (all messages are newer than request)")
	}

	// Verify chain
	fmt.Println("\n=== Chain Verification ===")
	valid := zigodb.Global().VerifyChain()
	fmt.Printf("Chain valid: %v\n", valid)

	// Export chunk
	fmt.Println("\n=== Exporting Chunk ===")
	chunkFile := "storage/chunks/replication_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		fmt.Printf("Export error: %v\n", err)
	} else {
		fmt.Printf("Chunk exported to %s\n", chunkFile)
	}

	fmt.Println("\n=== Replication tutorial completed successfully! ===")
	fmt.Println("Gossip protocol enables eventual consistency across distributed nodes.")
	fmt.Println("Message synchronization allows nodes to exchange missing messages.")
}
