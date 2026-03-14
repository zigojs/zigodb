package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// Initialize ZigoDB (T4.1)
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize ZigoDB: %v", err))
	}
	defer zigodb.Shutdown()

	fmt.Println("ZigoDB initialized successfully!")
	fmt.Println("Ready to store messages...")
}
