package main

import (
	"fmt"
	"os"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// Initialize ZigoDB
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize ZigoDB: %v", err))
	}
	defer zigodb.Shutdown()

	// Ensure storage directory exists
	os.MkdirAll("storage/chunks", 0755)

	// Write a few messages
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "test"}`, i))
		uuid, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Write error (expected): %v\n", err)
			break
		}
		fmt.Printf("Wrote message %d, UUID: %d\n", i, uuid)
	}

	// Get message count (T4.5)
	count := zigodb.Global().GetMessageCount()
	fmt.Printf("Message count: %d\n", count)

	// Check if drain is needed (T4.5)
	needsDrain := zigodb.Global().NeedsDrain()
	fmt.Printf("Needs drain: %v\n", needsDrain)
}
