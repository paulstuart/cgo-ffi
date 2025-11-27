// Package ffi demonstrates optimized Go-C interop with pre-allocated memory.
//
// This example shows how to minimize FFI overhead by:
// 1. Pre-allocating memory buffers at initialization
// 2. Reusing buffers across multiple calls
// 3. Using runtime.Pinner to prevent GC interference
// 4. Batching operations to reduce call frequency
package ffi

/*
#cgo CFLAGS: -O3 -march=native
#include "vector.h"
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

// VectorOps provides optimized C-backed vector operations.
// After initialization, calls are just function invocations with no allocation.
type VectorOps struct {
	// Pre-allocated buffers
	bufferA []float64
	bufferB []float64
	result  []float64

	// Pinners to prevent GC from moving memory during C calls
	pinnerA runtime.Pinner
	pinnerB runtime.Pinner
	pinnerR runtime.Pinner

	// C pointers (cached after pinning)
	ptrA *C.double
	ptrB *C.double
	ptrR *C.double

	// Capacity
	capacity int

	// Mutex for thread safety (C code may not be thread-safe)
	mu sync.Mutex
}

// NewVectorOps creates a new VectorOps with pre-allocated buffers.
// This is the one-time initialization cost.
func NewVectorOps(capacity int) *VectorOps {
	v := &VectorOps{
		bufferA:  make([]float64, capacity),
		bufferB:  make([]float64, capacity),
		result:   make([]float64, capacity),
		capacity: capacity,
	}

	// Pin the buffers so GC won't move them
	v.pinnerA.Pin(&v.bufferA[0])
	v.pinnerB.Pin(&v.bufferB[0])
	v.pinnerR.Pin(&v.result[0])

	// Cache the C pointers - these won't change now
	v.ptrA = (*C.double)(unsafe.Pointer(&v.bufferA[0]))
	v.ptrB = (*C.double)(unsafe.Pointer(&v.bufferB[0]))
	v.ptrR = (*C.double)(unsafe.Pointer(&v.result[0]))

	return v
}

// Close releases pinned memory. Must be called when done.
func (v *VectorOps) Close() {
	v.pinnerA.Unpin()
	v.pinnerB.Unpin()
	v.pinnerR.Unpin()
}

// Sum returns the sum of all elements.
// After initialization, this is effectively just a C function call.
func (v *VectorOps) Sum(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Copy data to pinned buffer
	copy(v.bufferA[:n], data[:n])

	// Call C - this is now just a function call, no allocation
	return float64(C.vector_sum(v.ptrA, C.size_t(n)))
}

// SumSIMD uses the SIMD-optimized C function.
func (v *VectorOps) SumSIMD(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	copy(v.bufferA[:n], data[:n])
	return float64(C.vector_sum_simd(v.ptrA, C.size_t(n)))
}

// Dot computes the dot product of two vectors.
func (v *VectorOps) Dot(a, b []float64) float64 {
	n := len(a)
	if n == 0 || len(b) < n {
		return 0
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	copy(v.bufferA[:n], a[:n])
	copy(v.bufferB[:n], b[:n])

	return float64(C.vector_dot(v.ptrA, v.ptrB, C.size_t(n)))
}

// Mul performs element-wise multiplication: result[i] = a[i] * b[i]
// Returns a slice view into the internal result buffer.
func (v *VectorOps) Mul(a, b []float64) []float64 {
	n := len(a)
	if n == 0 || len(b) < n {
		return nil
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	copy(v.bufferA[:n], a[:n])
	copy(v.bufferB[:n], b[:n])

	C.vector_mul(v.ptrA, v.ptrB, v.ptrR, C.size_t(n))

	// Return a copy to avoid data races after unlock
	result := make([]float64, n)
	copy(result, v.result[:n])
	return result
}

// MulInto performs element-wise multiplication into a provided destination.
// This avoids allocation if caller provides the buffer.
func (v *VectorOps) MulInto(a, b, dst []float64) {
	n := len(a)
	if n == 0 || len(b) < n || len(dst) < n {
		return
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	copy(v.bufferA[:n], a[:n])
	copy(v.bufferB[:n], b[:n])

	C.vector_mul(v.ptrA, v.ptrB, v.ptrR, C.size_t(n))

	copy(dst[:n], v.result[:n])
}

// Scale multiplies all elements by a scalar in-place.
func (v *VectorOps) Scale(data []float64, scalar float64) {
	n := len(data)
	if n == 0 {
		return
	}
	if n > v.capacity {
		n = v.capacity
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	copy(v.bufferA[:n], data[:n])

	C.vector_scale(v.ptrA, C.double(scalar), C.size_t(n))

	copy(data[:n], v.bufferA[:n])
}

// --- Direct FFI calls (for comparison - shows per-call overhead) ---

// DirectSum calls C directly without pre-allocated buffers.
// This shows the overhead of non-optimized FFI.
func DirectSum(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	// Each call pins, calls C, unpins
	var pinner runtime.Pinner
	pinner.Pin(&data[0])
	defer pinner.Unpin()

	ptr := (*C.double)(unsafe.Pointer(&data[0]))
	return float64(C.vector_sum(ptr, C.size_t(len(data))))
}

// DirectDot calls C directly without pre-allocated buffers.
func DirectDot(a, b []float64) float64 {
	if len(a) == 0 || len(b) < len(a) {
		return 0
	}
	var pinnerA, pinnerB runtime.Pinner
	pinnerA.Pin(&a[0])
	pinnerB.Pin(&b[0])
	defer pinnerA.Unpin()
	defer pinnerB.Unpin()

	ptrA := (*C.double)(unsafe.Pointer(&a[0]))
	ptrB := (*C.double)(unsafe.Pointer(&b[0]))
	return float64(C.vector_dot(ptrA, ptrB, C.size_t(len(a))))
}
