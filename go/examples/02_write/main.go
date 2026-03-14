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

	// Write a simple JSON message (T4.2)
	data := []byte(`{"msg": "hello", "user": "test"}`)
	uuid, err := zigodb.Global().Write(data)
	if err != nil {
		panic(fmt.Sprintf("Failed to write: %v", err))
	}

	fmt.Printf("Message written successfully! UUID: %d\n", uuid)
}
