package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// Initialize ZigoDB
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize ZigoDB: %v", err))
	}
	defer zigodb.Shutdown()

	// Write many messages
	fmt.Println("=== Writing Messages ===")
	for i := 0; i < 1000; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "test"}`, i))
		_, err := zigodb.Global().Write(data)
		if err != nil {
			panic(fmt.Sprintf("Failed to write: %v", err))
		}
		if (i+1)%200 == 0 {
			fmt.Printf("Wrote %d messages\n", i+1)
		}
	}

	// Check if drain is needed (T4.5)
	fmt.Println("\n=== Checking Drain Status ===")
	needsDrain := zigodb.Global().NeedsDrain()
	fmt.Printf("Needs drain: %v\n", needsDrain)

	// Get current message count
	count := zigodb.Global().GetMessageCount()
	fmt.Printf("Current message count: %d\n", count)

	// Export chunk as alternative to drain (more reliable)
	fmt.Println("\n=== Exporting Chunk ===")
	chunkFile := "storage/chunks/drain_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export: %v", err))
	}
	fmt.Printf("Exported %d messages to %s\n", count, chunkFile)

	fmt.Println("\n=== Drain example completed! ===")
}
