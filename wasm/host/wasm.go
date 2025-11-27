// Package host provides a Go host for WASM vector operations.
//
// This demonstrates the pre-allocated memory pattern for WASM:
// - WASM module allocates buffers once at initialization
// - Host gets buffer offsets and caches them
// - Per-call: copy data to WASM memory, call function, copy result back
// - No malloc/free on hot path
package host

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v39"
)

// wasiConfig creates a minimal WASI configuration for modules that need it
func wasiConfig() *wasmtime.WasiConfig {
	config := wasmtime.NewWasiConfig()
	return config
}

// WasmVectorOps provides WASM-backed vector operations.
// After initialization, calls involve only memory copies and function invocations.
type WasmVectorOps struct {
	engine   *wasmtime.Engine
	store    *wasmtime.Store
	instance *wasmtime.Instance
	memory   *wasmtime.Memory

	// Cached function references
	fnSum        *wasmtime.Func
	fnDot        *wasmtime.Func
	fnMul        *wasmtime.Func
	fnScale      *wasmtime.Func
	fnSumSimd    *wasmtime.Func

	// Pre-computed buffer offsets in WASM linear memory
	bufferAOffset uint32
	bufferBOffset uint32
	resultOffset  uint32
	capacity      uint32

	// Thread safety
	mu sync.Mutex
}

// WasmRuntime identifies which WASM implementation is loaded
type WasmRuntime string

const (
	RuntimeRust   WasmRuntime = "rust"
	RuntimeTinyGo WasmRuntime = "tinygo"
	RuntimeC      WasmRuntime = "c"
)

// NewWasmVectorOps loads a WASM module and initializes the vector operations.
// The wasmBytes should be the compiled WASM binary.
func NewWasmVectorOps(wasmBytes []byte) (*WasmVectorOps, error) {
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)

	module, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module: %w", err)
	}

	return newWasmVectorOpsFromModule(engine, store, module)
}

// NewWasmVectorOpsFromFile loads a WASM module from a file path.
func NewWasmVectorOpsFromFile(path string) (*WasmVectorOps, error) {
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)

	module, err := wasmtime.NewModuleFromFile(engine, path)
	if err != nil {
		return nil, fmt.Errorf("failed to load module from %s: %w", path, err)
	}

	return newWasmVectorOpsFromModule(engine, store, module)
}

func newWasmVectorOpsFromModule(engine *wasmtime.Engine, store *wasmtime.Store, module *wasmtime.Module) (*WasmVectorOps, error) {
	// Check if module needs WASI imports
	needsWasi := false
	for _, imp := range module.Imports() {
		if imp.Module() == "wasi_snapshot_preview1" {
			needsWasi = true
			break
		}
	}

	var instance *wasmtime.Instance
	var err error

	if needsWasi {
		// Use linker with WASI support
		linker := wasmtime.NewLinker(engine)
		if err := linker.DefineWasi(); err != nil {
			return nil, fmt.Errorf("failed to define WASI: %w", err)
		}

		// Configure WASI
		store.SetWasi(wasiConfig())

		instance, err = linker.Instantiate(store, module)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate module with WASI: %w", err)
		}
	} else {
		// Direct instantiation for non-WASI modules
		instance, err = wasmtime.NewInstance(store, module, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate module: %w", err)
		}
	}

	// Get memory export
	memExtern := instance.GetExport(store, "memory")
	if memExtern == nil {
		return nil, fmt.Errorf("module does not export 'memory'")
	}
	memory := memExtern.Memory()
	if memory == nil {
		return nil, fmt.Errorf("'memory' export is not a memory")
	}

	w := &WasmVectorOps{
		engine:   engine,
		store:    store,
		instance: instance,
		memory:   memory,
	}

	// Cache function references
	if err := w.cacheFunctions(); err != nil {
		return nil, err
	}

	// Get buffer offsets from WASM module
	if err := w.cacheOffsets(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *WasmVectorOps) cacheFunctions() error {
	funcs := map[string]**wasmtime.Func{
		"sum":      &w.fnSum,
		"dot":      &w.fnDot,
		"mul":      &w.fnMul,
		"scale":    &w.fnScale,
		"sum_simd": &w.fnSumSimd,
	}

	for name, ptr := range funcs {
		fn := w.instance.GetFunc(w.store, name)
		if fn == nil {
			return fmt.Errorf("module does not export function '%s'", name)
		}
		*ptr = fn
	}
	return nil
}

func (w *WasmVectorOps) cacheOffsets() error {
	// Get buffer A offset
	fn := w.instance.GetFunc(w.store, "get_buffer_a_offset")
	if fn == nil {
		return fmt.Errorf("module does not export 'get_buffer_a_offset'")
	}
	result, err := fn.Call(w.store)
	if err != nil {
		return fmt.Errorf("get_buffer_a_offset failed: %w", err)
	}
	w.bufferAOffset = uint32(result.(int32))

	// Get buffer B offset
	fn = w.instance.GetFunc(w.store, "get_buffer_b_offset")
	if fn == nil {
		return fmt.Errorf("module does not export 'get_buffer_b_offset'")
	}
	result, err = fn.Call(w.store)
	if err != nil {
		return fmt.Errorf("get_buffer_b_offset failed: %w", err)
	}
	w.bufferBOffset = uint32(result.(int32))

	// Get result offset
	fn = w.instance.GetFunc(w.store, "get_result_offset")
	if fn == nil {
		return fmt.Errorf("module does not export 'get_result_offset'")
	}
	result, err = fn.Call(w.store)
	if err != nil {
		return fmt.Errorf("get_result_offset failed: %w", err)
	}
	w.resultOffset = uint32(result.(int32))

	// Get capacity
	fn = w.instance.GetFunc(w.store, "get_capacity")
	if fn == nil {
		return fmt.Errorf("module does not export 'get_capacity'")
	}
	result, err = fn.Call(w.store)
	if err != nil {
		return fmt.Errorf("get_capacity failed: %w", err)
	}
	w.capacity = uint32(result.(int32))

	return nil
}

// Close releases WASM resources.
func (w *WasmVectorOps) Close() {
	w.store.Close()
	w.engine.Close()
}

// Capacity returns the maximum number of elements the buffers can hold.
func (w *WasmVectorOps) Capacity() int {
	return int(w.capacity)
}

// copyToWasm copies float64 slice to WASM linear memory at the given offset.
// Uses unsafe pointer casting for maximum performance (valid since f64 is same on both sides).
func (w *WasmVectorOps) copyToWasm(data []float64, offset uint32) {
	mem := w.memory.UnsafeData(w.store)
	dst := mem[offset : offset+uint32(len(data)*8)]
	src := unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*8)
	copy(dst, src)
}

// copyFromWasm copies float64 values from WASM linear memory.
// Uses unsafe pointer casting for maximum performance.
func (w *WasmVectorOps) copyFromWasm(dst []float64, offset uint32) {
	mem := w.memory.UnsafeData(w.store)
	src := mem[offset : offset+uint32(len(dst)*8)]
	dstBytes := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), len(dst)*8)
	copy(dstBytes, src)
}

// Sum returns the sum of all elements.
func (w *WasmVectorOps) Sum(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Copy data to WASM buffer A
	w.copyToWasm(data[:n], w.bufferAOffset)

	// Call WASM function
	result, err := w.fnSum.Call(w.store, int32(n))
	if err != nil {
		return 0
	}
	return result.(float64)
}

// SumSIMD uses the SIMD-optimized sum function.
func (w *WasmVectorOps) SumSIMD(data []float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.copyToWasm(data[:n], w.bufferAOffset)

	result, err := w.fnSumSimd.Call(w.store, int32(n))
	if err != nil {
		return 0
	}
	return result.(float64)
}

// Dot computes the dot product of two vectors.
func (w *WasmVectorOps) Dot(a, b []float64) float64 {
	n := len(a)
	if n == 0 || len(b) < n {
		return 0
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.copyToWasm(a[:n], w.bufferAOffset)
	w.copyToWasm(b[:n], w.bufferBOffset)

	result, err := w.fnDot.Call(w.store, int32(n))
	if err != nil {
		return 0
	}
	return result.(float64)
}

// Mul performs element-wise multiplication: result[i] = a[i] * b[i]
func (w *WasmVectorOps) Mul(a, b []float64) []float64 {
	n := len(a)
	if n == 0 || len(b) < n {
		return nil
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.copyToWasm(a[:n], w.bufferAOffset)
	w.copyToWasm(b[:n], w.bufferBOffset)

	_, err := w.fnMul.Call(w.store, int32(n))
	if err != nil {
		return nil
	}

	result := make([]float64, n)
	w.copyFromWasm(result, w.resultOffset)
	return result
}

// MulInto performs element-wise multiplication into a provided destination.
func (w *WasmVectorOps) MulInto(a, b, dst []float64) {
	n := len(a)
	if n == 0 || len(b) < n || len(dst) < n {
		return
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.copyToWasm(a[:n], w.bufferAOffset)
	w.copyToWasm(b[:n], w.bufferBOffset)

	_, err := w.fnMul.Call(w.store, int32(n))
	if err != nil {
		return
	}

	w.copyFromWasm(dst[:n], w.resultOffset)
}

// Scale multiplies all elements by a scalar.
// Note: This modifies the internal buffer, not the input slice.
func (w *WasmVectorOps) Scale(data []float64, scalar float64) {
	n := len(data)
	if n == 0 {
		return
	}
	if n > int(w.capacity) {
		n = int(w.capacity)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.copyToWasm(data[:n], w.bufferAOffset)

	_, err := w.fnScale.Call(w.store, scalar, int32(n))
	if err != nil {
		return
	}

	w.copyFromWasm(data[:n], w.bufferAOffset)
}
