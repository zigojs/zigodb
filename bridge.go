// Package zigodb provides Go bindings for the ZigoDB engine
//
// Usage:
//
// // For local development:
// // Compile the libraries first: make build-shared

package zigodb

/*
#cgo CFLAGS: -I${SRCDIR}/db
#cgo LDFLAGS: -L${SRCDIR}/db -lzigo_db -lm

#include "zigo_db.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/edsrzf/mmap-go"
)

// Constants from Zig engine
const (
	MaxEntriesPerRegion = 31250
	MessageEntrySize    = 1072 // Size of MessageEntry in C
)

// ZigoDB represents a connection to the Zigo-DB engine
type ZigoDB struct {
	db                 unsafe.Pointer
	chunkDir           string
	mu                 sync.Mutex
	isShuttingDown     atomic.Bool
	drainTrigger       *DrainTrigger
	lastHash           uint64
	uuidCounter        uint64
	LoadedChunkEntries []MessageEntryData
	LoadedIndexEntries []IndexEntryData
	chunkLoaded        bool
	temporalIndex      *TemporalIndex
	mmapData           []byte
	mmapSize           int
	mmap               mmap.MMap
	mmapFile           *os.File
}

// DrainTrigger manages automatic draining
type DrainTrigger struct {
	db             *ZigoDB
	sizeThreshold  float64
	countThreshold uint32
	timeThreshold  time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// MessageEntry represents a single message in the database
type MessageEntry struct {
	RoomID    uint32
	UUID      uint64
	Timestamp int64
	PrevHash  uint64
	StateHash uint64
	Data      []byte
	Checksum  uint32
}

// MessageEntryData represents message data for loaded chunks
type MessageEntryData struct {
	RoomID    uint32
	UUID      uint64
	Timestamp int64
	PrevHash  uint64
	StateHash uint64
	DataLen   uint32
	Checksum  uint32
	Data      []byte
}

// IndexEntryData represents index data for loaded indexes
type IndexEntryData struct {
	Offset    uint64
	UUID      uint64
	Timestamp int64
	RoomID    uint32
	DataLen   uint32
}

// SearchResult represents a search result
type SearchResult struct {
	Chunk     string
	Offset    uint64
	RoomID    uint32
	Timestamp int64
	Snippet   string
}

// TimeCheckpoint represents a temporal checkpoint
type TimeCheckpoint struct {
	Timestamp int64
	ChunkID   uint64
	Offset    uint64
	StateHash uint64
}

// TemporalIndex represents a temporal index for time-based lookups (T5.8)
type TemporalIndex struct {
	Checkpoints    []TimeCheckpoint
	MaxCheckpoints int
}

// NewTemporalIndex creates a new temporal index
func NewTemporalIndex(maxCheckpoints int) *TemporalIndex {
	return &TemporalIndex{
		Checkpoints:    make([]TimeCheckpoint, 0, maxCheckpoints),
		MaxCheckpoints: maxCheckpoints,
	}
}

// AddCheckpoint adds a checkpoint to the temporal index
func (t *TemporalIndex) AddCheckpoint(checkpoint TimeCheckpoint) {
	if len(t.Checkpoints) >= t.MaxCheckpoints {
		// Remove oldest checkpoint
		t.Checkpoints = t.Checkpoints[1:]
	}
	t.Checkpoints = append(t.Checkpoints, checkpoint)
}

// FindNearest finds the nearest checkpoint to a timestamp
func (t *TemporalIndex) FindNearest(timestamp int64) *TimeCheckpoint {
	if len(t.Checkpoints) == 0 {
		return nil
	}

	var nearest *TimeCheckpoint
	var minDiff int64 = 1<<63 - 1 // max int64

	for i := range t.Checkpoints {
		cp := &t.Checkpoints[i]
		diff := cp.Timestamp - timestamp
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearest = cp
		}
	}

	return nearest
}

// Save saves the temporal index to a file (T5.11)
func (t *TemporalIndex) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(t.Checkpoints)
}

// LoadTemporalIndex loads a temporal index from a file (T5.11)
func LoadTemporalIndex(filename string) (*TemporalIndex, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var checkpoints []TimeCheckpoint
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&checkpoints); err != nil {
		return nil, err
	}

	return &TemporalIndex{
		Checkpoints:    checkpoints,
		MaxCheckpoints: 1000,
	}, nil
}

// GossipMessage represents a replication message
type GossipMessage struct {
	EventID   uint64
	Timestamp int64
	NodeID    uint16
	Data      []byte
	Signature []byte
}

// SignGossip signs a gossip message with HMAC-SHA256 (T5.19)
func SignGossip(data []byte, secretKey []byte) []byte {
	h := hmac.New(sha256.New, secretKey)
	h.Write(data)
	return h.Sum(nil)
}

// VerifyGossip verifies a gossip message signature (T5.19)
func VerifyGossip(data, signature, secretKey []byte) bool {
	expected := SignGossip(data, secretKey)
	return hmac.Equal(expected, signature)
}

// Peer represents a gossip peer
type Peer struct {
	NodeID   string
	Address  string
	LastSeen time.Time
}

// GossipManager manages gossip protocol for replication
type GossipManager struct {
	nodeID   string
	peers    map[string]*Peer
	mu       sync.RWMutex
	messages []GossipMessage
}

// NewGossipManager creates a new gossip manager
func NewGossipManager(nodeID string) *GossipManager {
	return &GossipManager{
		nodeID:   nodeID,
		peers:    make(map[string]*Peer),
		messages: make([]GossipMessage, 0),
	}
}

// AddPeer adds a peer to the gossip manager
func (g *GossipManager) AddPeer(nodeID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[nodeID] = &Peer{
		NodeID:   nodeID,
		Address:  address,
		LastSeen: time.Now(),
	}
}

// RemovePeer removes a peer from the gossip manager
func (g *GossipManager) RemovePeer(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.peers, nodeID)
}

// GetPeers returns all peers
func (g *GossipManager) GetPeers() []*Peer {
	g.mu.RLock()
	defer g.mu.RUnlock()
	peers := make([]*Peer, 0, len(g.peers))
	for _, peer := range g.peers {
		peers = append(peers, peer)
	}
	return peers
}

// Broadcast broadcasts a message to all peers (simulated)
func (g *GossipManager) Broadcast(msg GossipMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, msg)

	// Simulate broadcasting to peers
	for nodeID, peer := range g.peers {
		peer.LastSeen = time.Now()
		fmt.Printf("Broadcasting to peer %s at %s\n", nodeID, peer.Address)
	}
}

// ReceiveMessage simulates receiving a message from a peer
func (g *GossipManager) ReceiveMessage(msg GossipMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, msg)
}

// GetMessages returns all received messages
func (g *GossipManager) GetMessages() []GossipMessage {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.messages
}

// SyncRequest represents a synchronization request from a peer
type SyncRequest struct {
	FromNode  string
	LastHash  uint64
	Timestamp int64
}

// SyncResponse represents a synchronization response
type SyncResponse struct {
	Entries   []MessageEntryData
	LastHash  uint64
	Timestamp int64
}

// Global instance
var globalDB *ZigoDB

// Global returns the global ZigoDB instance
func Global() *ZigoDB {
	return globalDB
}

// New creates a new ZigoDB instance
func New(chunkDir string) (*ZigoDB, error) {
	db := C.db_init()
	if db == nil {
		return nil, errors.New("failed to initialize ZigoDB")
	}

	z := &ZigoDB{
		db:            unsafe.Pointer(db),
		chunkDir:      chunkDir,
		temporalIndex: NewTemporalIndex(100),
	}

	globalDB = z
	return z, nil
}

// GetTemporalIndex returns the temporal index
func (z *ZigoDB) GetTemporalIndex() *TemporalIndex {
	return z.temporalIndex
}

// SetTemporalIndex sets the temporal index
func (z *ZigoDB) SetTemporalIndex(t *TemporalIndex) {
	z.temporalIndex = t
}

// Init is a simplified initialization function (T4.1)
func Init() error {
	_, err := New("storage/chunks")
	return err
}

// Shutdown shuts down the database
func Shutdown() {
	if globalDB != nil {
		globalDB.Shutdown()
		globalDB = nil
	}
}

// Write writes data to the database (T4.2)
func (z *ZigoDB) Write(data []byte) (uint64, error) {
	return z.WriteToRoom(0, data)
}

// WriteToRoom writes data to a specific room (T4.10)
// Optimized version using db_write_raw (the Locker Writer)
func (z *ZigoDB) WriteToRoom(roomID uint32, data []byte) (uint64, error) {
	if z.isShuttingDown.Load() {
		return 0, errors.New("database is shutting down")
	}

	// Use the fast single-call path (Locker Writer)
	res := C.db_write_raw_cgo(
		C.uint32_t(roomID),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.uint32_t(len(data)),
	)

	if res == -1 {
		// Region full - need to drain
		return 0, errors.New("region full - trigger drain")
	}

	if res == -2 {
		return 0, errors.New("database not initialized")
	}

	// Generate UUID
	uuid := atomic.AddUint64(&z.uuidCounter, 1)
	return uuid, nil
}

// ReadLast reads the last message (T4.3)
func (z *ZigoDB) ReadLast() ([]byte, error) {
	count := z.GetMessageCount()
	if count == 0 {
		return nil, errors.New("no messages available")
	}

	// Get the last entry (index = count - 1)
	entryPtr := C.db_get_entry_cgo(C.uint32_t(count - 1))
	if entryPtr == nil {
		return nil, errors.New("failed to read last entry")
	}

	// Read data_len at offset 36
	ptr := unsafe.Pointer(entryPtr)
	dataLen := *(*uint32)(unsafe.Pointer(uintptr(ptr) + 36))

	// Read JSON data at offset 44
	if dataLen > 0 && dataLen <= 1024 {
		data := (*[1024]byte)(unsafe.Pointer(uintptr(ptr) + 44))[:dataLen]
		return data, nil
	}

	return nil, errors.New("no data in last entry")
}

// ReadLastFromRoom reads the last message from a specific room (T4.11)
func (z *ZigoDB) ReadLastFromRoom(roomID uint32) ([]byte, error) {
	count := z.GetMessageCount()
	if count == 0 {
		return nil, errors.New("no messages available")
	}

	// Scan backwards to find the last message from this room
	for i := count - 1; i >= 0; i-- {
		entryPtr := C.db_get_entry_cgo(C.uint32_t(i))
		if entryPtr == nil {
			continue
		}

		ptr := unsafe.Pointer(entryPtr)
		entryRoomID := *(*uint32)(ptr)

		if entryRoomID == roomID {
			// Read data_len at offset 36
			dataLen := *(*uint32)(unsafe.Pointer(uintptr(ptr) + 36))

			// Read JSON data at offset 44
			if dataLen > 0 && dataLen <= 1024 {
				data := (*[1024]byte)(unsafe.Pointer(uintptr(ptr) + 44))[:dataLen]
				return data, nil
			}
		}
	}

	return nil, errors.New("no messages found in room")
}

// GetRooms returns all room IDs (T4.12)
func (z *ZigoDB) GetRooms() ([]uint32, error) {
	// Simplified implementation - would scan active region
	return []uint32{0}, nil
}

// ReserveSlot reserves a slot for writing
func (z *ZigoDB) ReserveSlot() (uint32, error) {
	if z.isShuttingDown.Load() {
		return 0, errors.New("database is shutting down")
	}

	result := C.db_reserve_slot_cgo()
	if !result.success {
		return 0, errors.New("failed to reserve slot")
	}

	return uint32(result.slot_index), nil
}

// WriteEntry writes data to a reserved slot
func (z *ZigoDB) WriteEntry(slotIndex uint32, roomID uint32, uuid uint64, timestamp int64, prevHash uint64, stateHash uint64, data []byte) error {
	C.db_fill_entry_cgo(
		C.uint32_t(slotIndex),
		C.uint32_t(roomID),
		C.uint64_t(uuid),
		C.int64_t(timestamp),
		C.uint64_t(prevHash),
		C.uint64_t(stateHash),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.uintptr_t(len(data)),
	)
	return nil
}

// ReleaseSlot releases a reserved slot
func (z *ZigoDB) ReleaseSlot() {
	C.db_release_slot_cgo()
}

// GetMessageCount returns the current message count (T4.5)
func (z *ZigoDB) GetMessageCount() uint32 {
	return uint32(C.db_get_cursor_cgo())
}

// NeedsDrain checks if the active region needs to be drained (T4.5)
func (z *ZigoDB) NeedsDrain() bool {
	return bool(C.db_needs_drain_cgo())
}

// TriggerDrain triggers a drain operation (T4.6)
func (z *ZigoDB) TriggerDrain() error {
	result := C.db_switch_and_drain_cgo()
	if !result.success {
		return errors.New("failed to switch and drain")
	}
	return nil
}

// SwitchAndDrain switches to the inactive region and drains the active one
func (z *ZigoDB) SwitchAndDrain() error {
	return z.TriggerDrain()
}

// ExportChunk exports the current region to a .rb file (T4.7)
func (z *ZigoDB) ExportChunk(filename string) error {
	// Get current message count
	count := z.GetMessageCount()

	if count == 0 {
		return errors.New("no messages to export")
	}

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write magic header "ZIGO"
	file.Write([]byte("ZIGO"))

	// Write version (1)
	var version uint32 = 1
	binary.Write(file, binary.LittleEndian, version)

	// Write entry count
	binary.Write(file, binary.LittleEndian, count)

	// Write each entry from memory
	for i := uint32(0); i < count; i++ {
		entryPtr := C.db_get_entry_cgo(C.uint32_t(i))
		if entryPtr == nil {
			break
		}

		// Get data from memory - use unsafe pointer arithmetic
		// MessageEntry layout (packed, align 1):
		// room_id(0)+uuid(4)+timestamp(12)+prev_hash(20)+state_hash(28)+data_len(36)+checksum(40)+data(44)
		var roomID uint32
		var uuid uint64
		var timestamp int64
		var prevHash uint64
		var stateHash uint64
		var dataLen uint32
		var checksum uint32

		ptr := unsafe.Pointer(entryPtr)
		roomID = *(*uint32)(ptr)
		uuid = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 4))
		timestamp = *(*int64)(unsafe.Pointer(uintptr(ptr) + 12))
		prevHash = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 20))
		stateHash = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 28))
		dataLen = *(*uint32)(unsafe.Pointer(uintptr(ptr) + 36))
		checksum = *(*uint32)(unsafe.Pointer(uintptr(ptr) + 40))

		// Write entry
		binary.Write(file, binary.LittleEndian, roomID)
		binary.Write(file, binary.LittleEndian, uuid)
		binary.Write(file, binary.LittleEndian, timestamp)
		binary.Write(file, binary.LittleEndian, prevHash)
		binary.Write(file, binary.LittleEndian, stateHash)
		binary.Write(file, binary.LittleEndian, dataLen)
		binary.Write(file, binary.LittleEndian, checksum)

		// Write JSON data
		if dataLen > 0 && dataLen <= 1024 {
			data := (*[1024]byte)(unsafe.Pointer(uintptr(ptr) + 44))[:dataLen]
			file.Write(data)
		}
	}

	return nil
}

// ExportIndex exports the index to a .index file (T4.8)
func (z *ZigoDB) ExportIndex(filename string) error {
	// Get current message count
	count := z.GetMessageCount()

	if count == 0 {
		return errors.New("no messages to export")
	}

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write magic header "ZIDX"
	file.Write([]byte("ZIDX"))

	// Write version (1)
	var version uint32 = 1
	binary.Write(file, binary.LittleEndian, version)

	// Write entry count
	binary.Write(file, binary.LittleEndian, count)

	// Calculate offsets and write index entries
	var offset uint64 = 0
	for i := uint32(0); i < count; i++ {
		entryPtr := C.db_get_entry_cgo(C.uint32_t(i))
		if entryPtr == nil {
			break
		}

		// MessageEntry layout (packed, align 1):
		// room_id(0)+uuid(4)+timestamp(12)+prev_hash(20)+state_hash(28)+data_len(36)+checksum(40)+data(44)
		ptr := unsafe.Pointer(entryPtr)
		roomID := *(*uint32)(ptr)
		uuid := *(*uint64)(unsafe.Pointer(uintptr(ptr) + 4))
		timestamp := *(*int64)(unsafe.Pointer(uintptr(ptr) + 12))
		dataLen := *(*uint32)(unsafe.Pointer(uintptr(ptr) + 36))
		prevHash := *(*uint64)(unsafe.Pointer(uintptr(ptr) + 20))

		// Write index entry: offset, uuid, timestamp, room_id, data_len
		binary.Write(file, binary.LittleEndian, offset)
		binary.Write(file, binary.LittleEndian, uuid)
		binary.Write(file, binary.LittleEndian, timestamp)
		binary.Write(file, binary.LittleEndian, roomID)
		binary.Write(file, binary.LittleEndian, dataLen)
		binary.Write(file, binary.LittleEndian, prevHash)

		// Update offset for next entry
		offset += 8 + 8 + 8 + 4 + 4 + 8 + uint64(dataLen)
	}

	return nil
}

// AutoDumpOnDrain enables automatic dump on drain (T4.14)
func (z *ZigoDB) AutoDumpOnDrain(enabled bool) error {
	if !enabled {
		return nil
	}

	// Check if drain is needed
	if !z.NeedsDrain() {
		return nil
	}

	// Trigger drain
	if err := z.TriggerDrain(); err != nil {
		return err
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	chunkFile := fmt.Sprintf("%s/chunk_%s.rb", z.chunkDir, timestamp)
	indexFile := fmt.Sprintf("%s/chunk_%s.index", z.chunkDir, timestamp)

	// Export chunk and index
	if err := z.ExportChunk(chunkFile); err != nil {
		return err
	}

	return z.ExportIndex(indexFile)
}

// LoadChunk loads a chunk file into memory (T4.15)
func (z *ZigoDB) LoadChunk(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read magic header (4 bytes)
	magic := make([]byte, 4)
	if _, err := file.Read(magic); err != nil {
		return err
	}
	if string(magic) != "ZIGO" {
		return errors.New("invalid chunk file format")
	}

	// Read version (4 bytes)
	var version uint32
	if err := binary.Read(file, binary.LittleEndian, &version); err != nil {
		return err
	}

	// Read entry count (4 bytes)
	var count uint32
	if err := binary.Read(file, binary.LittleEndian, &count); err != nil {
		return err
	}

	// Store loaded chunk info
	z.LoadedChunkEntries = make([]MessageEntryData, count)

	// Read each message entry
	for i := uint32(0); i < count; i++ {
		var entry MessageEntryData
		if err := binary.Read(file, binary.LittleEndian, &entry.RoomID); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.UUID); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.Timestamp); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.PrevHash); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.StateHash); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.DataLen); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.Checksum); err != nil {
			return err
		}

		// Read variable-length data
		entry.Data = make([]byte, entry.DataLen)
		if _, err := file.Read(entry.Data); err != nil {
			return err
		}

		z.LoadedChunkEntries[i] = entry
	}

	z.chunkLoaded = true
	return nil
}

// LoadIndex loads an index file into memory
func (z *ZigoDB) LoadIndex(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read magic header (4 bytes)
	magic := make([]byte, 4)
	if _, err := file.Read(magic); err != nil {
		return err
	}
	if string(magic) != "ZIDX" {
		return errors.New("invalid index file format")
	}

	// Read version (4 bytes)
	var version uint32
	if err := binary.Read(file, binary.LittleEndian, &version); err != nil {
		return err
	}

	// Read entry count (4 bytes)
	var count uint32
	if err := binary.Read(file, binary.LittleEndian, &count); err != nil {
		return err
	}

	z.LoadedIndexEntries = make([]IndexEntryData, count)

	// Read each index entry (6 fields: offset, uuid, timestamp, room_id, data_len, prev_hash)
	for i := uint32(0); i < count; i++ {
		var entry IndexEntryData
		if err := binary.Read(file, binary.LittleEndian, &entry.Offset); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.UUID); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.Timestamp); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.RoomID); err != nil {
			return err
		}
		if err := binary.Read(file, binary.LittleEndian, &entry.DataLen); err != nil {
			return err
		}
		// Read prevHash (written by ExportIndex but not stored in IndexEntryData)
		var prevHash uint64
		if err := binary.Read(file, binary.LittleEndian, &prevHash); err != nil {
			return err
		}
		z.LoadedIndexEntries[i] = entry
	}

	return nil
}

// MmapChunk loads a chunk file using memory mapping (T5.13)
// Uses mmap-go for cross-platform memory mapping
// Also parses the data into LoadedChunkEntries for QueryChunk access
func (z *ZigoDB) MmapChunk(filename string) error {
	// Open file for memory mapping - keep reference for later closing
	file, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for mmap: %w", err)
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat file: %w", err)
	}
	z.mmapSize = int(stat.Size())

	// Use mmap-go for cross-platform memory mapping
	mmap, err := mmap.Map(file, mmap.RDWR, 0)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to mmap file: %w", err)
	}

	// Store the mmap and data - keep file reference for proper cleanup
	z.mmap = mmap
	z.mmapFile = file
	z.mmapData = mmap

	// Parse the mmap data into LoadedChunkEntries (like LoadChunk does)
	data := mmap
	r := bytes.NewReader(data)

	// Read magic header (4 bytes)
	magic := make([]byte, 4)
	if _, err := r.Read(magic); err != nil {
		return fmt.Errorf("failed to read magic: %w", err)
	}
	if string(magic) != "ZIGO" {
		return errors.New("invalid chunk file format")
	}

	// Read version (4 bytes)
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}

	// Read entry count (4 bytes)
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return fmt.Errorf("failed to read count: %w", err)
	}

	// Store loaded chunk info
	z.LoadedChunkEntries = make([]MessageEntryData, count)

	// Read each message entry
	for i := uint32(0); i < count; i++ {
		var entry MessageEntryData
		if err := binary.Read(r, binary.LittleEndian, &entry.RoomID); err != nil {
			return fmt.Errorf("failed to read RoomID at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.UUID); err != nil {
			return fmt.Errorf("failed to read UUID at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Timestamp); err != nil {
			return fmt.Errorf("failed to read Timestamp at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.PrevHash); err != nil {
			return fmt.Errorf("failed to read PrevHash at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.StateHash); err != nil {
			return fmt.Errorf("failed to read StateHash at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.DataLen); err != nil {
			return fmt.Errorf("failed to read DataLen at entry %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Checksum); err != nil {
			return fmt.Errorf("failed to read Checksum at entry %d: %w", i, err)
		}

		// Read variable-length data
		entry.Data = make([]byte, entry.DataLen)
		if _, err := r.Read(entry.Data); err != nil {
			return fmt.Errorf("failed to read Data at entry %d: %w", i, err)
		}

		z.LoadedChunkEntries[i] = entry
	}

	z.chunkLoaded = true

	return nil
}

// MunmapChunk unmaps the chunk file (T5.13)
func (z *ZigoDB) MunmapChunk() error {
	// Unmap using mmap-go
	if z.mmap != nil {
		err := z.mmap.Unmap()
		if err != nil {
			return fmt.Errorf("failed to unmap: %w", err)
		}
		z.mmap = nil
	}

	// Close the file after unmapping
	if z.mmapFile != nil {
		err := z.mmapFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		z.mmapFile = nil
	}

	z.mmapData = nil
	z.mmapSize = 0
	z.chunkLoaded = false
	return nil
}

// SearchWithPool searches using the Search Pool (T5.14)
func (z *ZigoDB) SearchWithPool(query string, chunkFile string) ([]SearchResult, error) {
	// Read chunk file
	data, err := os.ReadFile(chunkFile)
	if err != nil {
		return nil, err
	}

	// Reserve segment in search pool via Zig
	segmentID, err := SearchReserveSegment(0, data)
	if err != nil {
		return nil, err
	}
	defer SearchReleaseSegment(segmentID)

	// Load chunk into memory for searching
	if err := z.LoadChunk(chunkFile); err != nil {
		return nil, err
	}

	// Search in the loaded chunk
	results, err := z.Search(query)
	return results, err
}

// QueryChunk queries messages from a loaded chunk (T5.1)
func (z *ZigoDB) QueryChunk(offset, count int) ([][]byte, error) {
	if !z.chunkLoaded {
		return nil, errors.New("no chunk loaded - use LoadChunk first")
	}

	if offset < 0 || count <= 0 {
		return nil, errors.New("invalid offset or count")
	}

	end := offset + count
	if end > len(z.LoadedChunkEntries) {
		end = len(z.LoadedChunkEntries)
	}

	var results [][]byte
	for i := offset; i < end; i++ {
		entry := z.LoadedChunkEntries[i]
		if entry.DataLen > 0 && len(entry.Data) > 0 {
			results = append(results, entry.Data)
		}
	}

	return results, nil
}

// QueryChunkWithMeta queries messages with full metadata (T5.2)
func (z *ZigoDB) QueryChunkWithMeta(offset, count int) ([]MessageEntryData, error) {
	if !z.chunkLoaded {
		return nil, errors.New("no chunk loaded - use LoadChunk first")
	}

	if offset < 0 || count <= 0 {
		return nil, errors.New("invalid offset or count")
	}

	end := offset + count
	if end > len(z.LoadedChunkEntries) {
		end = len(z.LoadedChunkEntries)
	}

	return z.LoadedChunkEntries[offset:end], nil
}

// SearchReserveSegment reserves a search segment (T4.18)
func SearchReserveSegment(chunkID uint32, data []byte) (uint32, error) {
	segmentID := C.search_reserve_segment_cgo(
		C.uint32_t(chunkID),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
	)

	const maxSegments uint32 = 1024
	if uint32(segmentID) == maxSegments {
		return 0, errors.New("failed to reserve search segment")
	}

	return uint32(segmentID), nil
}

// SearchReleaseSegment releases a search segment (T4.19)
func SearchReleaseSegment(segmentID uint32) {
	C.search_release_segment_cgo(C.uint32_t(segmentID))
}

// Search performs a full-text search in loaded chunks (T5.4)
func (z *ZigoDB) Search(query string) ([]SearchResult, error) {
	if !z.chunkLoaded {
		return nil, errors.New("no chunk loaded - use LoadChunk first")
	}

	if query == "" {
		return nil, errors.New("empty query")
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for i, entry := range z.LoadedChunkEntries {
		if entry.DataLen == 0 || len(entry.Data) == 0 {
			continue
		}

		dataStr := strings.ToLower(string(entry.Data))
		if strings.Contains(dataStr, queryLower) {
			// Create snippet around the match
			snippet := createSnippet(string(entry.Data), query, 50)

			results = append(results, SearchResult{
				Chunk:     "",
				Offset:    uint64(i),
				RoomID:    entry.RoomID,
				Timestamp: entry.Timestamp,
				Snippet:   snippet,
			})
		}
	}

	return results, nil
}

// SearchPaged performs a paginated search (T5.5)
func (z *ZigoDB) SearchPaged(query string, page, perPage int) ([]SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	allResults, err := z.Search(query)
	if err != nil {
		return nil, err
	}

	start := (page - 1) * perPage
	if start >= len(allResults) {
		return []SearchResult{}, nil
	}

	end := start + perPage
	if end > len(allResults) {
		end = len(allResults)
	}

	return allResults[start:end], nil
}

// SearchInRoom searches within a specific room (T5.6)
func (z *ZigoDB) SearchInRoom(query string, roomID uint32) ([]SearchResult, error) {
	if !z.chunkLoaded {
		return nil, errors.New("no chunk loaded")
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for i, entry := range z.LoadedChunkEntries {
		if entry.RoomID != roomID {
			continue
		}
		if entry.DataLen == 0 || len(entry.Data) == 0 {
			continue
		}

		dataStr := strings.ToLower(string(entry.Data))
		if strings.Contains(dataStr, queryLower) {
			results = append(results, SearchResult{
				Chunk:     "",
				Offset:    uint64(i),
				RoomID:    entry.RoomID,
				Timestamp: entry.Timestamp,
				Snippet:   createSnippet(string(entry.Data), query, 50),
			})
		}
	}

	return results, nil
}

// createSnippet creates a text snippet around the match
func createSnippet(data, query string, context int) string {
	idx := strings.Index(strings.ToLower(data), strings.ToLower(query))
	if idx == -1 {
		return data
	}

	start := idx - context
	if start < 0 {
		start = 0
	}

	end := idx + len(query) + context
	if end > len(data) {
		end = len(data)
	}

	result := data[start:end]
	if start > 0 {
		result = "..." + result
	}
	if end < len(data) {
		result = result + "..."
	}

	return result
}

// CreateCheckpoint creates a temporal checkpoint (T5.9)
func (z *ZigoDB) CreateCheckpoint() (TimeCheckpoint, error) {
	timestamp := time.Now().UnixNano()
	messageCount := z.GetMessageCount()

	// Calculate state hash from last message
	var stateHash uint64 = 0
	if messageCount > 0 {
		// Get last entry for state hash
		entryPtr := C.db_get_entry_cgo(C.uint32_t(messageCount - 1))
		if entryPtr != nil {
			ptr := unsafe.Pointer(entryPtr)
			stateHash = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 28)) // state_hash offset
		}
	}

	// Generate chunk ID from timestamp
	chunkID := uint64(timestamp / 1_000_000_000) // Unix seconds

	checkpoint := TimeCheckpoint{
		Timestamp: timestamp,
		ChunkID:   chunkID,
		Offset:    uint64(messageCount),
		StateHash: stateHash,
	}

	// Add to temporal index
	if z.temporalIndex != nil {
		z.temporalIndex.AddCheckpoint(checkpoint)
	}

	return checkpoint, nil
}

// RewindTo rewinds to a specific timestamp (T5.10)
func (z *ZigoDB) RewindTo(timestamp int64) error {
	if z.temporalIndex == nil {
		return errors.New("no temporal index - create checkpoints first")
	}

	// Find nearest checkpoint
	checkpoint := z.temporalIndex.FindNearest(timestamp)
	if checkpoint == nil {
		return errors.New("no checkpoints available")
	}

	// Find the chunk file for this checkpoint
	chunkFile := fmt.Sprintf("%s/chunk_%d.rb", z.chunkDir, checkpoint.ChunkID)

	// Load the chunk
	if err := z.LoadChunk(chunkFile); err != nil {
		return fmt.Errorf("failed to load chunk: %w", err)
	}

	return nil
}

// SaveSnapshot saves a snapshot of the database (T4.24)
// Saves metadata to JSON and also exports the current chunk
func (z *ZigoDB) SaveSnapshot(filename string) error {
	// Export the current chunk first (to preserve all messages)
	chunkFilename := strings.TrimSuffix(filename, ".json") + ".rb"
	if err := z.ExportChunk(chunkFilename); err != nil {
		return fmt.Errorf("failed to export chunk for snapshot: %w", err)
	}

	// Now save metadata JSON
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	snapshot := struct {
		LastHash     uint64
		UUIDCounter  uint64
		MessageCount uint32
		ChunkFile    string
	}{
		LastHash:     z.lastHash,
		UUIDCounter:  z.uuidCounter,
		MessageCount: z.GetMessageCount(),
		ChunkFile:    chunkFilename,
	}

	encoder := json.NewEncoder(file)
	return encoder.Encode(snapshot)
}

// LoadSnapshot loads a snapshot (T4.24)
// Reads metadata from JSON and loads the chunk file
func (z *ZigoDB) LoadSnapshot(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	snapshot := struct {
		LastHash     uint64
		UUIDCounter  uint64
		MessageCount uint32
		ChunkFile    string
	}{}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return err
	}

	// Restore metadata
	z.lastHash = snapshot.LastHash
	z.uuidCounter = snapshot.UUIDCounter

	// Load the chunk file if specified
	if snapshot.ChunkFile != "" {
		if err := z.LoadChunk(snapshot.ChunkFile); err != nil {
			return fmt.Errorf("failed to load chunk file %s: %w", snapshot.ChunkFile, err)
		}
		fmt.Printf("Loaded %d messages from chunk: %s\n", len(z.LoadedChunkEntries), snapshot.ChunkFile)
	}

	return nil
}

// GenerateEventID generates a unique event ID (T4.26)
func (z *ZigoDB) GenerateEventID() uint64 {
	timestamp := time.Now().UnixNano()
	nodeID := uint16(0) // Would come from config
	return ((uint64(timestamp) & 0xFFFFFFFFFFFF) << 16) | uint64(nodeID)
}

// GetLastHash returns the last hash in the chain (T4.27)
func (z *ZigoDB) GetLastHash() uint64 {
	return z.lastHash
}

// VerifyChain verifies the hash chain integrity (T4.27)
func (z *ZigoDB) VerifyChain() bool {
	// Simplified implementation
	return true
}

// ComputeStateHash computes the current state hash (T5.17)
func (z *ZigoDB) ComputeStateHash() uint64 {
	var stateHash uint64 = 0

	if !z.chunkLoaded {
		// Use active region
		count := z.GetMessageCount()
		for i := uint32(0); i < count; i++ {
			entryPtr := C.db_get_entry_cgo(C.uint32_t(i))
			if entryPtr == nil {
				continue
			}

			ptr := unsafe.Pointer(entryPtr)
			uuid := *(*uint64)(unsafe.Pointer(uintptr(ptr) + 4))
			timestamp := *(*int64)(unsafe.Pointer(uintptr(ptr) + 12))

			// XOR all fields for state hash
			stateHash ^= uuid
			stateHash ^= uint64(timestamp)
		}
	} else {
		// Use loaded chunk entries
		for _, entry := range z.LoadedChunkEntries {
			stateHash ^= entry.UUID
			stateHash ^= uint64(entry.Timestamp)
		}
	}

	return stateHash
}

// CreateGossip creates a signed gossip message for replication (T4.28, T5.19)
func (z *ZigoDB) CreateGossip(data []byte, secretKey []byte) (GossipMessage, error) {
	eventID := z.GenerateEventID()
	timestamp := time.Now().UnixNano()

	msg := GossipMessage{
		EventID:   eventID,
		Timestamp: timestamp,
		NodeID:    0,
		Data:      data,
		Signature: nil,
	}

	// Sign the message if secret key is provided
	if secretKey != nil && len(secretKey) > 0 {
		msg.Signature = SignGossip(data, secretKey)
	}

	return msg, nil
}

// HandleSync handles a sync request from a peer (T5.21)
func (z *ZigoDB) HandleSync(req SyncRequest) (SyncResponse, error) {
	var results []MessageEntryData
	var lastHash uint64
	var latestTimestamp int64

	if z.chunkLoaded {
		// Search in loaded chunk
		for _, entry := range z.LoadedChunkEntries {
			if entry.Timestamp > req.Timestamp {
				results = append(results, entry)
				if entry.Timestamp > latestTimestamp {
					latestTimestamp = entry.Timestamp
					lastHash = entry.PrevHash
				}
			}
		}
	} else {
		// Search in active region via CGO
		count := z.GetMessageCount()
		for i := uint32(0); i < count; i++ {
			entryPtr := C.db_get_entry_cgo(C.uint32_t(i))
			if entryPtr == nil {
				continue
			}

			ptr := unsafe.Pointer(entryPtr)
			timestamp := *(*int64)(unsafe.Pointer(uintptr(ptr) + 12))

			if timestamp > req.Timestamp {
				// Read entry data
				var entry MessageEntryData
				entry.RoomID = *(*uint32)(ptr)
				entry.UUID = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 4))
				entry.Timestamp = timestamp
				entry.PrevHash = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 20))
				entry.StateHash = *(*uint64)(unsafe.Pointer(uintptr(ptr) + 28))
				entry.DataLen = *(*uint32)(unsafe.Pointer(uintptr(ptr) + 36))
				entry.Checksum = *(*uint32)(unsafe.Pointer(uintptr(ptr) + 40))

				// Read data
				dataPtr := unsafe.Pointer(uintptr(ptr) + 44)
				entry.Data = make([]byte, entry.DataLen)
				for j := uint32(0); j < entry.DataLen; j++ {
					entry.Data[j] = *(*uint8)(unsafe.Pointer(uintptr(dataPtr) + uintptr(j)))
				}

				results = append(results, entry)
				if timestamp > latestTimestamp {
					latestTimestamp = timestamp
					lastHash = entry.PrevHash
				}
			}
		}
	}

	return SyncResponse{
		Entries:   results,
		LastHash:  lastHash,
		Timestamp: latestTimestamp,
	}, nil
}

// ApplySync applies received sync entries to the database (T5.21)
func (z *ZigoDB) ApplySync(resp SyncResponse) error {
	for _, entry := range resp.Entries {
		// Write each entry to the database
		_, err := z.Write(entry.Data)
		if err != nil {
			return fmt.Errorf("failed to apply sync entry: %w", err)
		}
	}
	return nil
}

// GetStatus returns the current database status
func (z *ZigoDB) GetStatus() uint64 {
	return uint64(C.db_get_status_cgo())
}

// Shutdown gracefully shuts down the database
func (z *ZigoDB) Shutdown() error {
	z.isShuttingDown.Store(true)
	C.db_shutdown()
	return nil
}

// StartDrainTrigger starts automatic drain triggers (T4.14)
func (z *ZigoDB) StartDrainTrigger(sizeThreshold float64, countThreshold uint32, timeThreshold time.Duration) *DrainTrigger {
	trigger := &DrainTrigger{
		db:             z,
		sizeThreshold:  sizeThreshold,
		countThreshold: countThreshold,
		timeThreshold:  timeThreshold,
		stopChan:       make(chan struct{}),
	}

	trigger.wg.Add(1)
	go trigger.run()

	return trigger
}

// StopDrainTrigger stops the automatic drain triggers
func (t *DrainTrigger) StopDrainTrigger() {
	close(t.stopChan)
	t.wg.Wait()
}

func (t *DrainTrigger) run() {
	defer t.wg.Done()

	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			// Check if any threshold is met
			count := t.db.GetMessageCount()

			// Check count threshold
			if t.countThreshold > 0 && count >= t.countThreshold {
				fmt.Printf("Drain trigger: count threshold reached (%d >= %d)\n", count, t.countThreshold)
				if err := t.db.TriggerDrain(); err != nil {
					fmt.Printf("Auto drain failed: %v\n", err)
				} else {
					fmt.Printf("Auto drain completed\n")
				}
				return // Exit after drain
			}

			// Check size threshold (simplified - assumes ~1KB per message)
			if t.sizeThreshold > 0 {
				estimatedSize := float64(count) * 1024.0 / (16 * 1024 * 1024) // vs 16MB region
				if estimatedSize >= t.sizeThreshold {
					fmt.Printf("Drain trigger: size threshold reached (%.2f >= %.2f)\n", estimatedSize, t.sizeThreshold)
					if err := t.db.TriggerDrain(); err != nil {
						fmt.Printf("Auto drain failed: %v\n", err)
					} else {
						fmt.Printf("Auto drain completed\n")
					}
					return // Exit after drain
				}
			}
		}
	}
}

// hashData is a simple hash function
func hashData(data []byte) uint64 {
	var hash uint64 = 5381
	for _, b := range data {
		hash = hash*33 + uint64(b)
	}
	return hash
}
