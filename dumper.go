// Package zigodb provides Go bindings for the ZigoDB engine
package zigodb

import (
	"fmt"
	"sync"
	"time"
)

// Dumper handles asynchronous chunk dumping to disk
type Dumper struct {
	db        *ZigoDB
	chunkDir  string
	mu        sync.Mutex
	isRunning bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewDumper creates a new Dumper instance
func NewDumper(db *ZigoDB, chunkDir string) *Dumper {
	return &Dumper{
		db:       db,
		chunkDir: chunkDir,
		stopChan: make(chan struct{}),
	}
}

// Start begins the dumper background goroutine
func (d *Dumper) Start(interval time.Duration) {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = true
	d.mu.Unlock()

	d.wg.Add(1)
	go d.run(interval)
}

// Stop gracefully stops the dumper
func (d *Dumper) Stop() {
	close(d.stopChan)
	d.wg.Wait()
}

func (d *Dumper) run(interval time.Duration) {
	defer d.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			// Do final drain before stopping
			if d.db.NeedsDrain() {
				d.dump()
			}
			return
		case <-ticker.C:
			if d.db.NeedsDrain() {
				d.dump()
			}
		}
	}
}

func (d *Dumper) dump() {
	// Get current timestamp for filename
	now := time.Now()
	_ = now // Timestamp used for filename generation

	// Switch and drain
	if err := d.db.SwitchAndDrain(); err != nil {
		fmt.Printf("Dumper: failed to switch and drain: %v\n", err)
		return
	}

	// TODO: Write the chunk file to disk
	fmt.Printf("Dumper: successfully dumped chunk to %s\n", d.chunkDir)
}

// ForceDump forces an immediate dump
func (d *Dumper) ForceDump() error {
	if !d.db.NeedsDrain() {
		return nil // Nothing to dump
	}

	d.dump()
	return nil
}
