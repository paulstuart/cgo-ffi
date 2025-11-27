// Command ffi-demo demonstrates optimized Go-C FFI with benchmarks.
package main

import (
	"fmt"
	"math/rand"
	"time"

	ffi "github.com/paulstuart/cgo-ffi"
)

func main() {
	fmt.Println("=== Go FFI with C Libraries - Optimized Demo ===")
	fmt.Println()

	// Create test data
	const size = 100000
	data := make([]float64, size)
	data2 := make([]float64, size)
	for i := range data {
		data[i] = rand.Float64() * 100
		data2[i] = rand.Float64() * 100
	}

	fmt.Printf("Test data: %d float64 elements\n\n", size)

	// --- Initialize VectorOps ONCE ---
	fmt.Println("Initializing VectorOps (one-time cost)...")
	initStart := time.Now()
	ops := ffi.NewVectorOps(size)
	defer ops.Close()
	fmt.Printf("Initialization time: %v\n\n", time.Since(initStart))

	// --- Demonstrate that FFI calls are now just function calls ---
	fmt.Println("=== Performance Comparison ===")
	fmt.Println()

	iterations := 1000

	// Sum - Pure Go
	start := time.Now()
	var goSum float64
	for i := 0; i < iterations; i++ {
		goSum = ffi.GoSum(data)
	}
	goTime := time.Since(start)
	fmt.Printf("Go Sum:           %v (%d iterations), result=%.2f\n", goTime, iterations, goSum)

	// Sum - Go Unrolled
	start = time.Now()
	var goUnrolledSum float64
	for i := 0; i < iterations; i++ {
		goUnrolledSum = ffi.GoSumUnrolled(data)
	}
	goUnrolledTime := time.Since(start)
	fmt.Printf("Go Sum (unrolled): %v (%d iterations), result=%.2f\n", goUnrolledTime, iterations, goUnrolledSum)

	// Sum - C with pre-allocated buffers (optimized FFI)
	start = time.Now()
	var cSum float64
	for i := 0; i < iterations; i++ {
		cSum = ops.Sum(data)
	}
	cOptTime := time.Since(start)
	fmt.Printf("C Sum (optimized): %v (%d iterations), result=%.2f\n", cOptTime, iterations, cSum)

	// Sum - C SIMD
	start = time.Now()
	var cSIMDSum float64
	for i := 0; i < iterations; i++ {
		cSIMDSum = ops.SumSIMD(data)
	}
	cSIMDTime := time.Since(start)
	fmt.Printf("C Sum (SIMD):     %v (%d iterations), result=%.2f\n", cSIMDTime, iterations, cSIMDSum)

	// Sum - C Direct (non-optimized FFI for comparison)
	start = time.Now()
	var cDirectSum float64
	for i := 0; i < iterations; i++ {
		cDirectSum = ffi.DirectSum(data)
	}
	cDirectTime := time.Since(start)
	fmt.Printf("C Sum (direct):   %v (%d iterations), result=%.2f\n", cDirectTime, iterations, cDirectSum)

	fmt.Println()
	fmt.Println("=== Analysis ===")
	fmt.Printf("Optimized C vs Go:        %.2fx speedup\n", float64(goTime)/float64(cOptTime))
	fmt.Printf("Optimized C vs Direct C:  %.2fx speedup (FFI overhead reduction)\n", float64(cDirectTime)/float64(cOptTime))
	fmt.Printf("SIMD C vs Go:             %.2fx speedup\n", float64(goTime)/float64(cSIMDTime))
	fmt.Println()

	// --- Dot Product ---
	fmt.Println("=== Dot Product ===")

	start = time.Now()
	var goDot float64
	for i := 0; i < iterations; i++ {
		goDot = ffi.GoDot(data, data2)
	}
	goDotTime := time.Since(start)
	fmt.Printf("Go Dot:           %v, result=%.2f\n", goDotTime, goDot)

	start = time.Now()
	var cDot float64
	for i := 0; i < iterations; i++ {
		cDot = ops.Dot(data, data2)
	}
	cDotTime := time.Since(start)
	fmt.Printf("C Dot (optimized): %v, result=%.2f\n", cDotTime, cDot)

	fmt.Printf("Speedup: %.2fx\n", float64(goDotTime)/float64(cDotTime))
	fmt.Println()

	// --- Key Takeaway ---
	fmt.Println("=== Key Takeaway ===")
	fmt.Println("After initialization, the C FFI calls are effectively just function calls.")
	fmt.Println("The pre-allocated, pinned buffers eliminate:")
	fmt.Println("  - Memory allocation per call")
	fmt.Println("  - GC pressure")
	fmt.Println("  - Runtime.Pinner overhead per call")
	fmt.Println()
	fmt.Println("For detailed benchmarks, run: go test -bench=. -benchmem")
}
