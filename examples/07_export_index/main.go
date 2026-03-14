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

	// Write some messages
	for i := 0; i < 100; i++ {
		data := []byte(fmt.Sprintf(`{"msg": "message %d", "user": "test"}`, i))
		_, err := zigodb.Global().Write(data)
		if err != nil {
			fmt.Printf("Write error: %v\n", err)
			break
		}
	}

	fmt.Println("Wrote 100 messages")

	// Trigger drain
	if zigodb.Global().NeedsDrain() {
		err := zigodb.Global().TriggerDrain()
		if err != nil {
			fmt.Printf("Drain error: %v\n", err)
		}
	}

	// Ensure directory exists
	os.MkdirAll("storage/chunks", 0755)

	// Export chunk (not fully implemented)
	chunkFile := "storage/chunks/chat_001.rb"
	err = zigodb.Global().ExportChunk(chunkFile)
	if err != nil {
		fmt.Printf("Export chunk (expected - not fully implemented): %v\n", err)
	} else {
		fmt.Printf("Chunk exported to: %s\n", chunkFile)
	}

	// Export index (not fully implemented)
	indexFile := "storage/chunks/chat_001.index"
	err = zigodb.Global().ExportIndex(indexFile)
	if err != nil {
		fmt.Printf("Export index (expected - not fully implemented): %v\n", err)
	} else {
		fmt.Printf("Index exported to: %s\n", indexFile)
	}
}
