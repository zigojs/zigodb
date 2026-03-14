package main

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	zigodb "github.com/zigojs/zigodb"
)

type BenchmarkResults struct {
	WriteThroughput float64
	ReadThroughput  float64
	TotalWrites     uint64
	TotalReads      uint64
	FailedWrites    uint64
	FailedReads     uint64
	AvgLatencyUs    float64
	NumCores        int
	Goroutines      int
	MemoryUsedMB    float64
	DumpCount       int
}

type BenchmarkMetrics struct {
	writeCount   atomic.Uint64
	readCount    atomic.Uint64
	failedWrites atomic.Uint64
	failedReads  atomic.Uint64
	latencies    atomic.Uint64
	latencyCount atomic.Uint64
	dumpCount    atomic.Int32
}

func NewBenchmarkMetrics() *BenchmarkMetrics {
	return &BenchmarkMetrics{}
}

func (m *BenchmarkMetrics) RecordWrite(latencyMicrosec uint64) {
	m.writeCount.Add(1)
	m.latencies.Add(latencyMicrosec)
	m.latencyCount.Add(1)
}

func (m *BenchmarkMetrics) RecordFailedWrite() {
	m.failedWrites.Add(1)
}

func (m *BenchmarkMetrics) RecordRead(latencyMicrosec uint64) {
	m.readCount.Add(1)
	m.latencies.Add(latencyMicrosec)
	m.latencyCount.Add(1)
}

func (m *BenchmarkMetrics) RecordFailedRead() {
	m.failedReads.Add(1)
}

func (m *BenchmarkMetrics) RecordDump() {
	m.dumpCount.Add(1)
}

func (m *BenchmarkMetrics) GetResults(duration time.Duration, numCores int, memUsedMB float64) BenchmarkResults {
	totalWrites := m.writeCount.Load()
	totalReads := m.readCount.Load()

	seconds := duration.Seconds()

	var avgLatency float64
	latencyCount := m.latencyCount.Load()
	if latencyCount > 0 {
		avgLatency = float64(m.latencies.Load()) / float64(latencyCount)
	}

	return BenchmarkResults{
		WriteThroughput: float64(totalWrites) / seconds,
		ReadThroughput:  float64(totalReads) / seconds,
		TotalWrites:     totalWrites,
		TotalReads:      totalReads,
		FailedWrites:    m.failedWrites.Load(),
		FailedReads:     m.failedReads.Load(),
		AvgLatencyUs:    avgLatency,
		NumCores:        numCores,
		Goroutines:      runtime.NumGoroutine(),
		MemoryUsedMB:    memUsedMB,
		DumpCount:       int(m.dumpCount.Load()),
	}
}

func GetMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / (1024 * 1024)
}

func BenchmarkWrite(numWriters int, duration time.Duration, msgSize int) BenchmarkResults {
	fmt.Printf("\n=== Write Benchmark (%d writers, %v) ===\n", numWriters, duration)

	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	testMsg := make([]byte, msgSize)
	for i := range testMsg {
		testMsg[i] = byte(i % 256)
	}

	metrics := NewBenchmarkMetrics()
	var wg sync.WaitGroup
	quit := make(chan struct{})

	startTime := time.Now()

	// Drain ticker - check every 10 microseconds for faster drain
	drainTick := time.NewTicker(10 * time.Microsecond)
	defer drainTick.Stop()

	// Shared drain mutex to prevent multiple drains
	var drainMutex sync.Mutex
	needsDrain := atomic.Bool{}

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				case <-drainTick.C:
					// Check cursor and trigger drain if needed
					cursor := zigodb.Global().GetMessageCount()
					if cursor >= 28000 && !needsDrain.Load() { // 90% of 31250
						if drainMutex.TryLock() {
							if !needsDrain.Load() {
								needsDrain.Store(true)
								zigodb.Global().SwitchAndDrain()
								metrics.RecordDump()
								needsDrain.Store(false)
							}
							drainMutex.Unlock()
						}
					}
				default:
					writeStart := time.Now()
					_, err := zigodb.Global().Write(testMsg)
					latency := time.Since(writeStart).Microseconds()
					if err != nil {
						metrics.RecordFailedWrite()
						// Try to drain on failure too
						if drainMutex.TryLock() {
							if !needsDrain.Load() {
								needsDrain.Store(true)
								zigodb.Global().SwitchAndDrain()
								metrics.RecordDump()
								needsDrain.Store(false)
							}
							drainMutex.Unlock()
						}
					} else {
						metrics.RecordWrite(uint64(latency))
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(quit)
	wg.Wait()

	elapsed := time.Since(startTime)
	memUsed := GetMemoryUsageMB()
	results := metrics.GetResults(elapsed, runtime.NumCPU(), memUsed)

	return results
}

func BenchmarkRead(numReaders int, duration time.Duration) BenchmarkResults {
	fmt.Printf("\n=== Read Benchmark (%d readers, %v) ===\n", numReaders, duration)

	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	testMsg := []byte("{\"benchmark\": \"read test\"}")
	for i := 0; i < 10000; i++ {
		zigodb.Global().Write(testMsg)
	}

	metrics := NewBenchmarkMetrics()
	var wg sync.WaitGroup
	quit := make(chan struct{})

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				default:
					readStart := time.Now()
					_, err := zigodb.Global().ReadLast()
					latency := time.Since(readStart).Microseconds()
					if err != nil {
						metrics.RecordFailedRead()
					} else {
						metrics.RecordRead(uint64(latency))
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(quit)
	wg.Wait()

	memUsed := GetMemoryUsageMB()
	results := metrics.GetResults(duration, runtime.NumCPU(), memUsed)

	return results
}

func BenchmarkMixed(numWriters, numReaders int, duration time.Duration) BenchmarkResults {
	fmt.Printf("\n=== Mixed Benchmark (%d writers + %d readers, %v) ===\n", numWriters, numReaders, duration)

	err := zigodb.Init()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize: %v", err))
	}
	defer zigodb.Shutdown()

	testMsg := []byte("{\"benchmark\": \"mixed\"}")

	writeMetrics := NewBenchmarkMetrics()
	readMetrics := NewBenchmarkMetrics()

	var wg sync.WaitGroup
	quit := make(chan struct{})

	// Drain ticker
	drainTick := time.NewTicker(10 * time.Microsecond)
	defer drainTick.Stop()

	var drainMutex sync.Mutex
	needsDrain := atomic.Bool{}

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				case <-drainTick.C:
					cursor := zigodb.Global().GetMessageCount()
					if cursor >= 28000 && !needsDrain.Load() {
						if drainMutex.TryLock() {
							if !needsDrain.Load() {
								needsDrain.Store(true)
								zigodb.Global().SwitchAndDrain()
								writeMetrics.RecordDump()
								needsDrain.Store(false)
							}
							drainMutex.Unlock()
						}
					}
				default:
					writeStart := time.Now()
					_, err := zigodb.Global().Write(testMsg)
					latency := time.Since(writeStart).Microseconds()
					if err != nil {
						writeMetrics.RecordFailedWrite()
						if drainMutex.TryLock() {
							if !needsDrain.Load() {
								needsDrain.Store(true)
								zigodb.Global().SwitchAndDrain()
								writeMetrics.RecordDump()
								needsDrain.Store(false)
							}
							drainMutex.Unlock()
						}
					} else {
						writeMetrics.RecordWrite(uint64(latency))
					}
				}
			}
		}()
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				default:
					readStart := time.Now()
					_, err := zigodb.Global().ReadLast()
					latency := time.Since(readStart).Microseconds()
					if err != nil {
						readMetrics.RecordFailedRead()
					} else {
						readMetrics.RecordRead(uint64(latency))
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(quit)
	wg.Wait()

	memUsed := GetMemoryUsageMB()

	results := BenchmarkResults{
		WriteThroughput: float64(writeMetrics.writeCount.Load()) / duration.Seconds(),
		ReadThroughput:  float64(readMetrics.readCount.Load()) / duration.Seconds(),
		TotalWrites:     writeMetrics.writeCount.Load(),
		TotalReads:      readMetrics.readCount.Load(),
		FailedWrites:    writeMetrics.failedWrites.Load(),
		FailedReads:     readMetrics.failedReads.Load(),
		NumCores:        runtime.NumCPU(),
		Goroutines:      runtime.NumGoroutine(),
		MemoryUsedMB:    memUsed,
		DumpCount:       int(writeMetrics.dumpCount.Load()),
	}

	return results
}

func PrintResults(name string, r BenchmarkResults) {
	writeStr := strconv.FormatFloat(r.WriteThroughput, 'f', 0, 64)
	readStr := strconv.FormatFloat(r.ReadThroughput, 'f', 0, 64)
	fmt.Printf("\n%s:\n", name)
	fmt.Printf("  Write:  %s msg/sec (total: %d)\n", writeStr, r.TotalWrites)
	fmt.Printf("  Read:   %s msg/sec (total: %d)\n", readStr, r.TotalReads)
	fmt.Printf("  Latency: %.2f us avg\n", r.AvgLatencyUs)
	fmt.Printf("  Failed:  W:%d R:%d\n", r.FailedWrites, r.FailedReads)
	fmt.Printf("  Dumps:   %d\n", r.DumpCount)
	fmt.Printf("  Memory: %.1f MB | Goroutines: %d\n", r.MemoryUsedMB, r.Goroutines)
}

func main() {
	fmt.Println("==============================================")
	fmt.Println("    Zigo-DB Performance Benchmark")
	fmt.Println("==============================================")

	numCores := runtime.NumCPU()
	fmt.Printf("\nSystem: %d CPU Cores | Go %s\n", numCores, runtime.Version())

	testDuration := 3 * time.Second

	fmt.Printf("\n\n")
	fmt.Println("================= WRITE BENCHMARKS =================")

	writeResults := make([]BenchmarkResults, 0)

	writerCounts := []int{1, 4, 8}
	for i, n := range writerCounts {
		zigodb.Shutdown()
		time.Sleep(100 * time.Millisecond)
		r := BenchmarkWrite(n, testDuration, 256)
		writeResults = append(writeResults, r)
		PrintResults(fmt.Sprintf("Test %d: %d Writer(s)", i+1, n), r)
		runtime.GC()
	}

	fmt.Printf("\n\n")
	fmt.Println("================= READ BENCHMARKS =================")

	readResults := make([]BenchmarkResults, 0)

	readerCounts := []int{1, 4}
	for i, n := range readerCounts {
		zigodb.Shutdown()
		time.Sleep(100 * time.Millisecond)
		r := BenchmarkRead(n, testDuration)
		readResults = append(readResults, r)
		PrintResults(fmt.Sprintf("Test %d: %d Reader(s)", i+1, n), r)
		runtime.GC()
	}

	fmt.Printf("\n\n")
	fmt.Println("================= MIXED BENCHMARKS =================")

	mixedResults := make([]BenchmarkResults, 0)

	mixedConfigs := []struct{ w, r int }{{1, 1}, {4, 4}, {8, 8}}
	for i, cfg := range mixedConfigs {
		zigodb.Shutdown()
		time.Sleep(100 * time.Millisecond)
		r := BenchmarkMixed(cfg.w, cfg.r, testDuration)
		mixedResults = append(mixedResults, r)
		PrintResults(fmt.Sprintf("Test %d: %dW + %dR", i+1, cfg.w, cfg.r), r)
		runtime.GC()
	}

	fmt.Printf("\n\n")
	fmt.Println("=================== SUMMARY ===================")

	bestWrite, bestRead := 0.0, 0.0
	for _, r := range writeResults {
		if r.WriteThroughput > bestWrite {
			bestWrite = r.WriteThroughput
		}
	}
	for _, r := range readResults {
		if r.ReadThroughput > bestRead {
			bestRead = r.ReadThroughput
		}
	}

	bestMixedW, bestMixedR := 0.0, 0.0
	for _, r := range mixedResults {
		if r.WriteThroughput > bestMixedW {
			bestMixedW = r.WriteThroughput
		}
		if r.ReadThroughput > bestMixedR {
			bestMixedR = r.ReadThroughput
		}
	}

	fmt.Printf("\nBest Performance:\n")
	fmt.Printf("  Write:  %s msg/sec\n", strconv.FormatFloat(bestWrite, 'f', 0, 64))
	fmt.Printf("  Read:   %s msg/sec\n", strconv.FormatFloat(bestRead, 'f', 0, 64))
	fmt.Printf("  Mixed:  %s write + %s read msg/sec\n",
		strconv.FormatFloat(bestMixedW, 'f', 0, 64),
		strconv.FormatFloat(bestMixedR, 'f', 0, 64))

	fmt.Println("\nBenchmark completed!")
}
