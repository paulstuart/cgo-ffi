# WASM Vectorscan Matcher

A WebAssembly-based multi-pattern regex matcher using [Vectorscan](https://github.com/VectorCamp/vectorscan) (a fork of Intel Hyperscan) compiled to WASM and run via [wasmtime-go](https://github.com/bytecodealliance/wasmtime-go).

## Overview

This package provides high-performance multi-pattern regex matching without requiring CGO for the Vectorscan library itself. Instead, Vectorscan is compiled to WebAssembly using Emscripten and executed via the wasmtime runtime.

### Supported Pattern Features

All standard regex features work:
- Literals: `hello`
- Quantifiers: `ab*c`, `ab+c`, `a?b`, `\d+`
- Character classes: `[a-z]+`, `[0-9]`
- Alternation: `foo|bar`
- Anchors: `^foo`, `bar$`
- Escapes: `\.txt`, `\d`, `\w`
- Case insensitive: `(?i)HELLO`
- And more...

## Requirements

### Build Requirements
- [Emscripten](https://emscripten.org/) (tested with 4.0.20)
- [Binaryen](https://github.com/WebAssembly/binaryen) (`wasm-opt` command)
- CMake
- Boost headers (for Vectorscan compilation)

### Runtime Requirements
- Go 1.23+
- wasmtime-go v39.0.1

## Building

### Quick Build

```bash
cd matcher/wasm
./build.sh nosimd   # Build without SIMD (most compatible)
# or
./build.sh simd     # Build with WASM SIMD (faster, requires SIMD support)
# or
./build.sh all      # Build all variants
```

### Docker Build (Reproducible)

```bash
cd matcher/wasm
docker build -t vectorscan-wasm .
docker run -v $(pwd):/workspace vectorscan-wasm ./build.sh nosimd
```

### Build Variants

| Variant | SIMD | Optimization | Use Case |
|---------|------|--------------|----------|
| `nosimd-O3` | No | -O3 | Maximum compatibility |
| `simd-O3` | Yes | -O3 | Best performance (requires WASM SIMD) |

## Usage

### Go API

```go
package main

import (
    "fmt"
    wasmvs "github.com/paulstuart/cgo-ffi/matcher/wasm/host"
)

func main() {
    // Create matcher with patterns
    patterns := []string{
        `error`,
        `warning`,
        `\d+\.\d+\.\d+`,  // Version numbers
        `[a-z]+@[a-z]+\.com`,  // Simple email
    }

    m, err := wasmvs.NewWasmMatcher(patterns)
    if err != nil {
        panic(err)
    }
    defer m.Close()

    // Match against input
    input := "Error: version 1.2.3 released"
    result := m.Match(input)
    if result >= 0 {
        fmt.Printf("Matched pattern %d: %s\n", result, patterns[result])
    }
}
```

### CLI Tool

```bash
# Build the CLI
cd matcher/wasm/host/cmd/matcher
go build

# Run with patterns
./matcher -p 'hello,world' -i 'hello there'
# Output: Match: pattern[0] = "hello"

# Load patterns from file
./matcher -f patterns.txt -i 'test input'

# Verbose output
./matcher -p '\d+,[a-z]+' -i '123abc' -v
```

## Architecture

```
matcher/wasm/
├── build.sh              # Build script for WASM compilation
├── Dockerfile            # Docker environment for reproducible builds
├── vectorscan/           # Vectorscan source (submodule)
├── src/
│   └── matcher.cpp       # C++ wrapper exposing simple API
├── out/                  # Built WASM variants
└── host/
    ├── matcher.go        # Go bindings using wasmtime-go
    ├── matcher.wasm      # Embedded WASM module
    ├── config_cgo.go     # CGO helper for exception handling
    └── cmd/matcher/      # CLI tool
```

## Technical Details

### Exception Handling

Vectorscan uses C++ exceptions internally. This requires special handling for WASM:

1. **Emscripten Compilation**: Uses `-fwasm-exceptions` flag to enable native WASM exception handling

2. **Binaryen Transformation**: The `--translate-to-exnref` pass converts legacy exception format (`try`/`catch`) to the new format (`try_table`/`exnref`) required by wasmtime v39+

3. **wasmtime Configuration**: Exception handling and GC features must be enabled in the wasmtime engine config (done via CGO in `config_cgo.go`)

### Why Binaryen Transformation is Needed

Emscripten generates WASM with the **legacy** exception handling format (using `try`/`catch` instructions), but wasmtime v39 only supports the **new** exception handling format (using `try_table`/`exnref`). The build script uses:

```bash
wasm-opt --all-features --translate-to-exnref --emit-exnref -o output.wasm input.wasm
```

This transforms the exception handling instructions to the format wasmtime expects.

### Enabling Exceptions in wasmtime-go

wasmtime-go v39 doesn't expose `SetWasmExceptions()` method directly. The `config_cgo.go` file uses CGO to call the C API:

```go
// Uses reflection to access wasmtime.Config's private pointer
// Then calls wasmtime_config_wasm_exceptions_set() via CGO
```

### Memory Management

- WASM module uses 64MB initial memory with growth enabled
- Patterns are passed as newline-separated strings
- The Go host allocates/frees memory in WASM space via exported `wasm_alloc`/`wasm_free` functions

### WASI Support

The module uses WASI for basic system calls (memory, environment). The wasmtime linker provides WASI implementation.

## Performance

The WASM version is slower than native Vectorscan via CGO but provides:
- No CGO dependency for the regex engine
- Portable across platforms
- Sandboxed execution

Typical overhead: 2-5x compared to native (varies by pattern complexity).

## Troubleshooting

### Build Errors

**"exceptions proposal not enabled"**
- Ensure you're using wasmtime v39+
- The `config_cgo.go` file must be compiled to enable exceptions

**"legacy_exceptions feature required"**
- The WASM module needs transformation via `wasm-opt --translate-to-exnref`
- Run `./build.sh` which includes this step

**Boost headers not found**
- Install Boost: `brew install boost` (macOS) or `apt install libboost-dev` (Linux)
- Or use Docker build

### Runtime Errors

**"incompatible import type for emscripten_notify_memory_growth"**
- The function signature must be `func(int32)` not `func()`

**"exception refs not supported without the exception handling feature"**
- The wasmtime engine needs exception handling enabled
- Ensure `config_cgo.go` is being compiled (check for CGO errors)

**Pattern compilation fails**
- Check pattern syntax - Vectorscan uses PCRE-like syntax
- Use `m.GetError()` to retrieve error details

## Development

### Running Tests

```bash
cd matcher/wasm/host
go test -v
```

### Rebuilding WASM Module

After modifying `src/matcher.cpp`:

```bash
cd matcher/wasm
rm -rf vectorscan/build-*  # Clean previous builds
./build.sh nosimd
```

### Checking WASM Module

```bash
# Verify no legacy exception instructions remain
wasm2wat --enable-all host/matcher.wasm | grep -c "^\s*try\s"
# Should output: 0

# Check exports
wasm-objdump -x host/matcher.wasm | head -50
```

## References

- [Vectorscan](https://github.com/VectorCamp/vectorscan) - High-performance regex matching
- [wasmtime-go](https://github.com/bytecodealliance/wasmtime-go) - Go bindings for wasmtime
- [Emscripten Exception Handling](https://emscripten.org/docs/porting/exceptions.html)
- [WebAssembly Exception Handling Proposal](https://github.com/WebAssembly/exception-handling)
- [Binaryen](https://github.com/WebAssembly/binaryen) - WASM optimization toolkit
- [LLVM -wasm-use-legacy-eh option](https://github.com/llvm/llvm-project/pull/122158)
- [wasmtime Exception Handling PR](https://github.com/bytecodealliance/wasmtime/pull/11326)

## License

See LICENSE file in repository root.
