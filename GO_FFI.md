# Go FFI Memory Management Patterns

This document describes patterns for eliminating per-call overhead in Go's Foreign Function Interface (FFI) - applicable to both cgo and WebAssembly (WASM) interop.

## Core Principle: Pre-allocated, Stable Memory

The key insight is separating **initialization-time costs** (acceptable) from **per-call costs** (critical path). After initialization:

- No memory allocation
- No garbage collector interaction
- No pointer validation overhead
- Just a function call with stable pointers

## Pattern 1: Pinned Go Memory (cgo)

For cgo, Go owns the memory and pins it so C can safely access it:

```go
type Ops struct {
    buffer  []float64
    pinner  runtime.Pinner  // Go 1.21+
    cPtr    *C.double       // Cached, stable after init
}

func NewOps(capacity int) *Ops {
    o := &Ops{buffer: make([]float64, capacity)}
    o.pinner.Pin(&o.buffer[0])                           // Pin once
    o.cPtr = (*C.double)(unsafe.Pointer(&o.buffer[0]))   // Cache pointer
    return o
}

func (o *Ops) Process(data []float64) float64 {
    copy(o.buffer, data)                    // Copy to stable buffer
    return float64(C.process(o.cPtr, n))    // Just a function call
}
```

## Pattern 2: Foreign-Owned Memory (WASM)

For WASM, the foreign side owns linear memory. Go copies data into pre-allocated WASM buffers:

```go
type WasmOps struct {
    inputOffset  uint32  // Offset into WASM linear memory
    outputOffset uint32  // Pre-allocated by WASM module
    memory       []byte  // View into WASM linear memory
}

func (w *WasmOps) Process(data []float64) float64 {
    // Copy Go data into WASM's pre-allocated input buffer
    dst := w.memory[w.inputOffset:]
    for i, v := range data {
        binary.LittleEndian.PutUint64(dst[i*8:], math.Float64bits(v))
    }
    // Call WASM function - it reads from known offset, writes to known offset
    return w.callWasm("process", len(data))
}
```

### WASM Zero-Alloc Pattern

For WASM specifically, when:

1. Go has input data to pass to WASM
2. WASM has pre-defined input/output buffer regions
3. Go memory is not needed after copy

The workflow is:

```text
Initialization (once):
┌─────────────────────────────────────────────────────┐
│ WASM Module                                         │
│  ┌─────────────────────────────────────────────┐   │
│  │ Linear Memory                                │   │
│  │  [0x1000] input_buffer  (pre-allocated)     │   │
│  │  [0x2000] output_buffer (pre-allocated)     │   │
│  └─────────────────────────────────────────────┘   │
│  exports: process(input_len) -> result_len         │
└─────────────────────────────────────────────────────┘

Per-call (no allocation):
┌──────────┐    copy    ┌──────────────────────┐
│ Go slice │ ────────── │ WASM input_buffer    │
└──────────┘            └──────────────────────┘
     │                           │
     │ (no longer needed)        │ WASM processes
     ▼                           ▼
   [GC can                ┌──────────────────────┐
    collect]              │ WASM output_buffer   │
                          └──────────────────────┘
                                  │
                                  │ copy (if needed)
                                  ▼
                          ┌──────────────────────┐
                          │ Go result slice      │
                          └──────────────────────┘
```

Key advantages:

- WASM allocates buffers once during module instantiation
- Go caller just copies into known offsets
- No malloc/free per call on either side
- Buffer sizes are fixed/known, bounds checking is trivial

## Memory Ownership Rules

| Pattern | Memory Owner | Allocation | Pointer Stability |
|---------|--------------|------------|-------------------|
| cgo pinned | Go | Go heap, pinned | `runtime.Pinner` |
| WASM linear | WASM | WASM linear memory | Fixed offsets |
| C-allocated | C | `malloc` | Until `free` |

**Critical**: Never mix ownership. If C/WASM allocated it, C/WASM frees it.

## When This Pattern Helps

**Good fit:**

- High-frequency calls to foreign code
- Known, bounded data sizes
- Compute-intensive operations (amortizes copy cost)
- Long-lived processing contexts

**Poor fit:**

- Highly variable data sizes (wastes pre-allocated space)
- Infrequent calls (initialization not amortized)
- Very small data (copy overhead dominates)

## Implementation Checklist

- [ ] Buffers allocated once at initialization
- [ ] Pointers/offsets cached and stable
- [ ] Data copied to stable buffers before foreign call
- [ ] Foreign code operates within buffer bounds
- [ ] No allocation on hot path
- [ ] Cleanup called on context teardown

## Thread Safety Considerations

Pre-allocated buffers are inherently not thread-safe. Options:

1. **Mutex per ops instance** (this project's approach)
2. **Per-goroutine instances** via `sync.Pool`
3. **Lock-free** if foreign code is thread-safe and buffers are independent

```go
// Option 2: Per-goroutine pooling
var opsPool = sync.Pool{
    New: func() any { return NewOps(maxSize) },
}

func Process(data []float64) float64 {
    ops := opsPool.Get().(*Ops)
    defer opsPool.Put(ops)
    return ops.Process(data)
}
```

## Benchmark Results (Apple M2 Pro)

For sum of 100,000 float64 elements:

| Implementation | Time | Allocs | Notes |
|----------------|------|--------|-------|
| Pure Go | 45µs | 0 | Fastest for simple ops |
| cgo SIMD | 69µs | 0 | C with loop unrolling |
| cgo Direct | 149µs | 0 | Per-call pin/unpin |
| cgo Pre-alloc | 192µs | 0 | Copy + mutex overhead |
| WASM Rust SIMD | 305µs | 10 | wasmtime-go overhead |
| WASM Rust | 462µs | 10 | Copy dominates |

**Key insight**: For simple numeric operations, FFI overhead (copy + call) dominates.
The crossover point where native/WASM wins is for more complex, compute-intensive operations
where the O(1) call overhead is amortized over O(n) or O(n²) computation.

## References

- [runtime.Pinner](https://pkg.go.dev/runtime#Pinner) (Go 1.21+)
- [cgo documentation](https://pkg.go.dev/cmd/cgo)
- [WASM linear memory](https://webassembly.github.io/spec/core/syntax/modules.html#memories)
- [wasmtime-go](https://pkg.go.dev/github.com/bytecodealliance/wasmtime-go)
