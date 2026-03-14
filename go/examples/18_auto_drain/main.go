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

	// Enable auto-dump on drain (T4.14)
	fmt.Println("=== Enabling Auto-Dump on Drain ===")
	if err := zigodb.Global().AutoDumpOnDrain(true); err != nil {
		fmt.Printf("AutoDumpOnDrain error: %v\n", err)
	} else {
		fmt.Println("Auto-dump enabled: chunks will be exported automatically on drain")
	}

	// Start drain trigger (T4.14)
	// Parameters: sizeThreshold (0.9 = 90%), countThreshold, timeThreshold
	fmt.Println("\n=== Starting Drain Trigger ===")
	trigger := zigodb.Global().StartDrainTrigger(
		0.5,            // Size threshold: 50% (lower for demo)
		15,             // Count threshold: 15 messages (will trigger with 20)
		30*time.Second, // Time threshold: 30 seconds
	)
	if trigger != nil {
		fmt.Println("Drain trigger started with:")
		fmt.Println("  - Size threshold: 50%")
		fmt.Println("  - Count threshold: 15 messages")
		fmt.Println("  - Time threshold: 30 seconds")
	} else {
		fmt.Println("Note: Drain trigger not available in this implementation")
	}

	// Write messages to trigger drain
	fmt.Println("\n=== Writing Messages (will trigger drain at 15) ===")
	writeCount := 0
	for i := 0; i < 20; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "auto drain test %d", "seq": %d}`, i, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Failed to write: %v\n", err)
		} else {
			writeCount++
			fmt.Printf("Wrote message %d, UUID: %d\n", writeCount, uuid)
		}
	}

	// Wait for drain trigger to fire (checks every second)
	fmt.Println("\n=== Waiting for drain trigger... ===")
	time.Sleep(2 * time.Second)

	// Check if drain is needed (T4.5)
	fmt.Println("\n=== Checking Drain Status ===")
	needsDrain := zigodb.Global().NeedsDrain()
	fmt.Printf("Needs drain: %v\n", needsDrain)

	// Get current message count
	count := zigodb.Global().GetMessageCount()
	fmt.Printf("Current message count: %d\n", count)

	// Export remaining messages if any (alternative to drain)
	if count > 0 {
		fmt.Println("\n=== Exporting Messages (After Drain) ===")
		chunkFile := "storage/chunks/auto_drain_test.rb"
		if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
			fmt.Printf("Export error: %v\n", err)
		} else {
			fmt.Printf("Exported to %s\n", chunkFile)
		}
	}

	fmt.Println("\n=== Auto-Drain tutorial completed successfully! ===")
	fmt.Println("Auto-drain triggers automatic region switching when thresholds are met.")
	fmt.Println("Useful for production systems to prevent data loss from full regions.")
	fmt.Println("Note: Use ExportChunk as manual alternative to automatic drain.")
}
