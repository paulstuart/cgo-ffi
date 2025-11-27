// TinyGo implementation of vector operations for WASM
//
// Uses pre-allocated static buffers to eliminate per-call allocation.
// The host copies data into these buffers at known offsets.
//
// Build: tinygo build -o vector.wasm -target=wasi -opt=2 main.go

package main

import "unsafe"

// Pre-allocated buffer capacity (100K f64 elements = 800KB per buffer)
const capacity = 100_000

// Static buffers - allocated once, stable addresses
var bufferA [capacity]float64
var bufferB [capacity]float64
var result [capacity]float64

// main is required but empty for WASM library
func main() {}

//export sum
func sum(len uint32) float64 {
	n := int(len)
	if n > capacity {
		n = capacity
	}
	var s float64
	for i := 0; i < n; i++ {
		s += bufferA[i]
	}
	return s
}

//export dot
func dot(len uint32) float64 {
	n := int(len)
	if n > capacity {
		n = capacity
	}
	var d float64
	for i := 0; i < n; i++ {
		d += bufferA[i] * bufferB[i]
	}
	return d
}

//export mul
func mul(len uint32) {
	n := int(len)
	if n > capacity {
		n = capacity
	}
	for i := 0; i < n; i++ {
		result[i] = bufferA[i] * bufferB[i]
	}
}

//export scale
func scale(scalar float64, len uint32) {
	n := int(len)
	if n > capacity {
		n = capacity
	}
	for i := 0; i < n; i++ {
		bufferA[i] *= scalar
	}
}

//export sum_simd
func sumSimd(len uint32) float64 {
	n := int(len)
	if n > capacity {
		n = capacity
	}
	// 4-way unrolling for better performance
	var sum0, sum1, sum2, sum3 float64
	i := 0
	for ; i+3 < n; i += 4 {
		sum0 += bufferA[i]
		sum1 += bufferA[i+1]
		sum2 += bufferA[i+2]
		sum3 += bufferA[i+3]
	}
	for ; i < n; i++ {
		sum0 += bufferA[i]
	}
	return sum0 + sum1 + sum2 + sum3
}

//export get_buffer_a_offset
func getBufferAOffset() uint32 {
	return uint32(uintptr(unsafe.Pointer(&bufferA[0])))
}

//export get_buffer_b_offset
func getBufferBOffset() uint32 {
	return uint32(uintptr(unsafe.Pointer(&bufferB[0])))
}

//export get_result_offset
func getResultOffset() uint32 {
	return uint32(uintptr(unsafe.Pointer(&result[0])))
}

//export get_capacity
func getCapacity() uint32 {
	return capacity
}
