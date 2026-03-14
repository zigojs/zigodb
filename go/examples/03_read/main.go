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

	// Write a message first
	data := []byte(`{"msg": "hello", "user": "test"}`)
	uuid, err := zigodb.Global().Write(data)
	if err != nil {
		panic(fmt.Sprintf("Failed to write: %v", err))
	}
	fmt.Printf("Message written! UUID: %d\n", uuid)

	// Read the last message (T4.3)
	// Note: This is a placeholder - actual implementation requires memory access
	lastData, err := zigodb.Global().ReadLast()
	if err != nil {
		fmt.Printf("ReadLast not available yet: %v\n", err)
	} else {
		fmt.Printf("Last message: %s\n", string(lastData))
	}
}
