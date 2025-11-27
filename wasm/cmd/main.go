// Command wasm-demo demonstrates running WASM vector operations from Go.
//
// Usage:
//
//	go run ./cmd [rust|tinygo|c]
//
// If no argument given, runs whichever WASM modules are available.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/paulstuart/cgo-ffi/wasm/host"
)

// stats holds timing statistics for a benchmark
type stats struct {
	min   time.Duration
	avg   time.Duration
	p95   time.Duration
	total time.Duration
	n     int
}

// benchmark runs a function n times and collects timing statistics
func benchmark(n int, fn func()) stats {
	times := make([]time.Duration, n)

	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		times[i] = time.Since(start)
	}

	// Sort for percentile calculation
	slices.Sort(times)

	// Calculate statistics
	var total time.Duration
	for _, t := range times {
		total += t
	}

	p95idx := int(float64(n) * 0.95)
	if p95idx >= n {
		p95idx = n - 1
	}

	return stats{
		min:   times[0],
		avg:   total / time.Duration(n),
		p95:   times[p95idx],
		total: total,
		n:     n,
	}
}

func (s stats) String() string {
	return fmt.Sprintf("avg=%v  min=%v  p95=%v  (n=%d, total=%v)",
		s.avg.Round(time.Microsecond),
		s.min.Round(time.Microsecond),
		s.p95.Round(time.Microsecond),
		s.n,
		s.total.Round(time.Millisecond))
}

func main() {
	fmt.Println("=== WASM Vector Operations Demo ===")
	fmt.Println()

	// Find available WASM modules
	wasmDir := findWasmDir()
	modules := map[string]string{
		"rust":   filepath.Join(wasmDir, "rust", "vector.wasm"),
		"tinygo": filepath.Join(wasmDir, "tinygo", "vector.wasm"),
		"c":      filepath.Join(wasmDir, "c", "vector.wasm"),
	}

	// Filter to requested or available modules
	requested := os.Args[1:]
	if len(requested) == 0 {
		// Run all available
		for name, path := range modules {
			if _, err := os.Stat(path); err == nil {
				requested = append(requested, name)
			}
		}
	}

	if len(requested) == 0 {
		fmt.Println("No WASM modules found. Build them first:")
		fmt.Println("  cd wasm && ./build.sh all")
		os.Exit(1)
	}

	// Create test data
	const size = 100000
	data := make([]float64, size)
	data2 := make([]float64, size)
	for i := range data {
		data[i] = rand.Float64() * 100
		data2[i] = rand.Float64() * 100
	}
	fmt.Printf("Test data: %d float64 elements, 1000 iterations\n\n", size)

	// Run each module
	for _, name := range requested {
		path, ok := modules[name]
		if !ok {
			fmt.Printf("Unknown module: %s\n", name)
			continue
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("=== %s === (not built, skipping)\n\n", name)
			continue
		}

		runModule(name, path, data, data2)
	}

	// Compare with pure Go
	fmt.Println("=== Pure Go Reference ===")
	runGoReference(data, data2)
}

func runModule(name, path string, data, data2 []float64) {
	fmt.Printf("=== %s ===\n", name)

	// Load module
	start := time.Now()
	ops, err := host.NewWasmVectorOpsFromFile(path)
	if err != nil {
		fmt.Printf("  Error loading: %v\n\n", err)
		return
	}
	defer ops.Close()
	fmt.Printf("  Load time: %v\n", time.Since(start))
	fmt.Printf("  Capacity:  %d elements\n", ops.Capacity())

	iterations := 1000

	// Sum
	var sumResult float64
	sumStats := benchmark(iterations, func() {
		sumResult = ops.Sum(data)
	})
	fmt.Printf("  Sum:       %s  result=%.2f\n", sumStats, sumResult)

	// Sum SIMD
	var simdResult float64
	simdStats := benchmark(iterations, func() {
		simdResult = ops.SumSIMD(data)
	})
	fmt.Printf("  Sum SIMD:  %s  result=%.2f\n", simdStats, simdResult)

	// Dot product
	var dotResult float64
	dotStats := benchmark(iterations, func() {
		dotResult = ops.Dot(data, data2)
	})
	fmt.Printf("  Dot:       %s  result=%.2f\n", dotStats, dotResult)

	fmt.Println()
}

func runGoReference(data, data2 []float64) {
	iterations := 1000

	// Sum
	var sumResult float64
	sumStats := benchmark(iterations, func() {
		sumResult = goSum(data)
	})
	fmt.Printf("  Sum:       %s  result=%.2f\n", sumStats, sumResult)

	// Dot
	var dotResult float64
	dotStats := benchmark(iterations, func() {
		dotResult = goDot(data, data2)
	})
	fmt.Printf("  Dot:       %s  result=%.2f\n", dotStats, dotResult)

	fmt.Println()
}

func goSum(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum
}

func goDot(a, b []float64) float64 {
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}

// findWasmDir locates the wasm directory relative to the executable
func findWasmDir() string {
	// Try relative to current working directory
	if _, err := os.Stat("rust/vector.wasm"); err == nil {
		return "."
	}
	if _, err := os.Stat("../rust/vector.wasm"); err == nil {
		return ".."
	}

	// Try relative to executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "rust/vector.wasm")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "../rust/vector.wasm")); err == nil {
			return filepath.Join(dir, "..")
		}
	}

	// Default to parent of cmd
	return ".."
}
