package main

import (
	"fmt"

	zigodb "github.com/zigojs/zigodb"
)

func main() {
	// Initialize ZigoDB
	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	// Define room IDs for different chat rooms
	const (
		RoomGeneral = 1
		RoomTech    = 2
		RoomRandom  = 3
	)

	// Write messages to different rooms (T4.10)
	fmt.Println("=== Writing Messages to Different Rooms ===")

	// General room messages
	for i := 0; i < 3; i++ {
		data := []byte(fmt.Sprintf(`{"user": "admin", "msg": "Welcome to general chat %d"}`, i))
		uuid, err := zigodb.Global().WriteToRoom(RoomGeneral, data)
		if err != nil {
			fmt.Printf("Failed to write to general room: %v\n", err)
		} else {
			fmt.Printf("General room: Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Tech room messages
	for i := 0; i < 3; i++ {
		data := []byte(fmt.Sprintf(`{"user": "dev", "msg": "Tech discussion %d", "language": "go"}`, i))
		uuid, err := zigodb.Global().WriteToRoom(RoomTech, data)
		if err != nil {
			fmt.Printf("Failed to write to tech room: %v\n", err)
		} else {
			fmt.Printf("Tech room: Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Random room messages
	for i := 0; i < 3; i++ {
		data := []byte(fmt.Sprintf(`{"user": "guest", "msg": "Random thought %d"}`, i))
		uuid, err := zigodb.Global().WriteToRoom(RoomRandom, data)
		if err != nil {
			fmt.Printf("Failed to write to random room: %v\n", err)
		} else {
			fmt.Printf("Random room: Wrote message %d, UUID: %d\n", i, uuid)
		}
	}

	// Get total message count
	totalCount := zigodb.Global().GetMessageCount()
	fmt.Printf("\nTotal message count: %d\n", totalCount)

	// Get all rooms (T4.12)
	fmt.Println("\n=== Getting Room List ===")
	rooms, err := zigodb.Global().GetRooms()
	if err != nil {
		fmt.Printf("GetRooms error: %v\n", err)
	} else {
		fmt.Printf("Active rooms: %v\n", rooms)
	}

	// Read last message from each room (T4.11)
	fmt.Println("\n=== Reading Last Message from Each Room ===")

	// Read from General room
	lastGeneral, err := zigodb.Global().ReadLastFromRoom(RoomGeneral)
	if err != nil {
		fmt.Printf("Error reading from General room: %v\n", err)
	} else {
		fmt.Printf("General room last message: %s\n", string(lastGeneral))
	}

	// Read from Tech room
	lastTech, err := zigodb.Global().ReadLastFromRoom(RoomTech)
	if err != nil {
		fmt.Printf("Error reading from Tech room: %v\n", err)
	} else {
		fmt.Printf("Tech room last message: %s\n", string(lastTech))
	}

	// Read from Random room
	lastRandom, err := zigodb.Global().ReadLastFromRoom(RoomRandom)
	if err != nil {
		fmt.Printf("Error reading from Random room: %v\n", err)
	} else {
		fmt.Printf("Random room last message: %s\n", string(lastRandom))
	}

	// Export chunk for later analysis
	fmt.Println("\n=== Exporting Chunk ===")
	chunkFile := "storage/chunks/rooms_test.rb"
	if err := zigodb.Global().ExportChunk(chunkFile); err != nil {
		panic(fmt.Sprintf("Failed to export chunk: %v", err))
	}
	fmt.Printf("Chunk exported to %s\n", chunkFile)

	fmt.Println("\n=== Room-based messaging tutorial completed successfully! ===")
	fmt.Println("Rooms allow separating messages by chat room or topic.")
	fmt.Println("Each room has independent message streams with unique UUIDs.")
}
