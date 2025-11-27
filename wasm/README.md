# WASM Vector Operations

This directory contains WebAssembly implementations of the vector operations, demonstrating the pre-allocated memory pattern for Go-to-WASM interop.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Go Host                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ WasmVectorOps                                            │   │
│  │  - Cached buffer offsets (obtained once at init)        │   │
│  │  - Cached function references                           │   │
│  │  - copyToWasm() / copyFromWasm()                        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                    wasmtime-go runtime                          │
└──────────────────────────────┼──────────────────────────────────┘
                               │
┌──────────────────────────────┼──────────────────────────────────┐
│                         WASM Module                              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Linear Memory                                            │   │
│  │  [offset_a] buffer_a: [f64; 100000]                     │   │
│  │  [offset_b] buffer_b: [f64; 100000]                     │   │
│  │  [offset_r] result:   [f64; 100000]                     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Exports:                                                        │
│    sum(len) -> f64           get_buffer_a_offset() -> u32       │
│    dot(len) -> f64           get_buffer_b_offset() -> u32       │
│    mul(len)                  get_result_offset() -> u32         │
│    scale(scalar, len)        get_capacity() -> u32              │
│    sum_simd(len) -> f64                                         │
└──────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
wasm/
├── wit/
│   └── vector.wit       # WebAssembly Interface Types definition
├── rust/
│   ├── Cargo.toml
│   └── src/lib.rs       # Rust implementation with wit-bindgen
├── tinygo/
│   └── main.go          # TinyGo implementation with //export
├── c/
│   └── vector_wasm.c    # C implementation for wasi-sdk/Emscripten
├── host/
│   ├── wasm.go          # Go host using wasmtime-go
│   └── wasm_test.go     # Tests and benchmarks
├── build.sh             # Build script for all WASM modules
└── README.md
```

## Prerequisites

### Rust (recommended)
```bash
# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Add WASM target
rustup target add wasm32-wasip1
```

### TinyGo
```bash
# macOS
brew install tinygo

# Linux - see https://tinygo.org/getting-started/install/
```

### C (choose one)

**wasi-sdk (recommended):**
```bash
# Download from https://github.com/WebAssembly/wasi-sdk/releases
export WASI_SDK_PATH=/path/to/wasi-sdk
```

**Emscripten:**
```bash
# Install emsdk - see https://emscripten.org/docs/getting_started/downloads.html
```

## Building

```bash
# Build all WASM modules
./build.sh all

# Or individually
./build.sh rust
./build.sh tinygo
./build.sh c
```

## Running Tests

```bash
# From the wasm/host directory
cd host
go test -v

# Run benchmarks
go test -bench=. -benchmem
```

## The Pre-Allocated Memory Pattern

### Why This Pattern?

Traditional WASM interop requires:
1. Allocate memory in WASM for input
2. Copy data from host to WASM
3. Call WASM function
4. Allocate memory for result
5. Copy result back to host
6. Free WASM memory

This pattern eliminates steps 1, 4, and 6 on the hot path:

### How It Works

**Initialization (once):**
```go
ops, _ := NewWasmVectorOpsFromFile("vector.wasm")
// Internally:
// 1. Loads WASM module
// 2. Calls get_buffer_a_offset() etc. to learn where buffers are
// 3. Caches offsets and function references
```

**Per-call (hot path):**
```go
result := ops.Sum(data)
// Internally:
// 1. Lock mutex
// 2. Copy data to WASM memory at cached offset (no allocation)
// 3. Call WASM function
// 4. Return result
// 5. Unlock mutex
```

### Memory Layout

The WASM module pre-allocates static buffers:

```rust
// Rust
static mut BUFFER_A: [f64; 100000] = [0.0; 100000];
```

```go
// TinyGo
var bufferA [100000]float64
```

```c
// C
static double buffer_a[100000];
```

These have fixed addresses in linear memory. The host queries these addresses once and caches them.

## Performance Characteristics

| Factor | Impact |
|--------|--------|
| Copy overhead | O(n) per call - data must be copied to/from WASM memory |
| Call overhead | ~100-500ns per WASM function call |
| No allocation | Zero malloc/free after initialization |
| Computation | Depends on WASM runtime optimization (Wasmtime uses Cranelift JIT) |

**When WASM wins:**
- Compute-intensive operations where copy is small relative to computation
- Rust/C algorithms with better optimization than Go
- SIMD operations (with WASM SIMD proposal)

**When Go wins:**
- Small data sizes (copy overhead dominates)
- Simple operations Go handles well
- Memory-bound operations (WASM adds copy overhead)

## Benchmark Results

Run `go test -bench=. -benchmem` in the `host/` directory to see results on your machine.

Typical patterns:
- Rust WASM ~1.5-2x slower than native cgo for small data
- TinyGo WASM ~2-3x slower than native Go
- C WASM similar to Rust
- All WASM implementations scale linearly with data size
- Crossover point (where WASM becomes competitive) depends on operation complexity

## WIT Interface

The `wit/vector.wit` file defines the interface using WebAssembly Interface Types:

```wit
interface ops {
    sum: func(len: u32) -> float64;
    dot: func(len: u32) -> float64;
    // ...
    get-buffer-a-offset: func() -> u32;
    // ...
}
```

This is the modern approach for WASM component model, though the current implementations use simpler direct exports for broader compatibility.

## Extending

To add a new operation:

1. Add to `wit/vector.wit`:
   ```wit
   new-op: func(len: u32) -> float64;
   ```

2. Implement in each language (rust/src/lib.rs, tinygo/main.go, c/vector_wasm.c)

3. Add to host (host/wasm.go):
   ```go
   func (w *WasmVectorOps) NewOp(data []float64) float64 {
       // ...
   }
   ```

4. Add tests and benchmarks (host/wasm_test.go)
