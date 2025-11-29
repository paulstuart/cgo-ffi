# Vectorscan WebAssembly Support - Proposed PR

This document describes the changes needed to add WebAssembly (Emscripten) support to [Vectorscan](https://github.com/VectorCamp/vectorscan) and the steps to submit a PR.

## Summary

Vectorscan can be compiled to WebAssembly using Emscripten, enabling high-performance multi-pattern regex matching in browsers and WASM runtimes. The key insight is that Vectorscan's existing **SIMDe backend** (used for x86 SSE2 emulation) works perfectly for WASM builds.

## Required Changes

### 1. CMakeLists.txt Modifications

Add WASM/Emscripten detection and configuration:

```cmake
# WASM/Emscripten support - force SIMDe backend
if(EMSCRIPTEN)
    set(ARCH_WASM32 TRUE)
    set(ARCH_32_BIT TRUE)
    set(SIMDE_BACKEND TRUE)
    # Clear architecture flags that aren't valid for WASM
    set(ARCH_C_FLAGS "")
    set(ARCH_CXX_FLAGS "")
    set(GNUCC_ARCH "")
    set(GNUCC_TUNE "")
    # Disable warnings-as-errors for unsupported flags
    set(EXTRA_C_FLAGS "-Wno-error=unused-command-line-argument")
    set(EXTRA_CXX_FLAGS "-Wno-error=unused-command-line-argument")
    message(STATUS "Building for WebAssembly with SIMDe emulation")
endif()
```

Also skip arch flag detection for WASM:

```cmake
if (NOT FAT_RUNTIME AND NOT EMSCRIPTEN)
    # existing arch flag detection...
endif()

# Clear arch flags for WASM builds
if (EMSCRIPTEN)
    set(ARCH_C_FLAGS "")
    set(ARCH_CXX_FLAGS "")
    set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -Wno-error=unused-command-line-argument -Wno-error=pass-failed -Wno-pass-failed")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wno-error=unused-command-line-argument -Wno-error=pass-failed -Wno-pass-failed")
endif()
```

### 2. Full Patch File

See `docker/vectorscan-wasm.patch` for the complete patch.

## Build Instructions

### With Docker (Recommended)

```bash
# Build the Docker image
docker build -t vectorscan-wasm -f docker/Dockerfile .

# Run the build
docker run -v $(pwd):/workspace vectorscan-wasm
```

### Without Docker

```bash
# Prerequisites: emscripten, cmake, boost headers, ragel

# Clone vectorscan with SIMDe submodule
git clone --recursive https://github.com/VectorCamp/vectorscan.git
cd vectorscan

# Apply the WASM patch
patch -p1 < ../vectorscan-wasm.patch

# Configure with Emscripten
emcmake cmake -B build-wasm \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF \
    -DFAT_RUNTIME=OFF \
    -DBUILD_AVX2=OFF \
    -DBUILD_AVX512=OFF

# Build
emmake make -C build-wasm -j$(nproc) hs

# Result: build-wasm/lib/libhs.a
```

## Example Usage

A simple C wrapper for WASM:

```c
#include "hs.h"

static hs_database_t *g_database = NULL;
static hs_scratch_t *g_scratch = NULL;

int matcher_init(const char* patterns_data, int patterns_len) {
    // Parse patterns and compile
    hs_compile_error_t *compile_err = NULL;
    hs_error_t err = hs_compile_multi(expressions, flags, ids, count,
                                      HS_MODE_BLOCK, NULL, &g_database, &compile_err);
    if (err != HS_SUCCESS) return -1;

    err = hs_alloc_scratch(g_database, &g_scratch);
    return (err == HS_SUCCESS) ? 0 : -1;
}

int matcher_match(const char* input, int input_len) {
    int match_id = -1;
    hs_scan(g_database, input, input_len, 0, g_scratch, match_handler, &match_id);
    return match_id;
}
```

Build with:

```bash
emcc -O3 -fwasm-exceptions \
    -I vectorscan/src \
    -I build-wasm \
    wrapper.cpp \
    build-wasm/lib/libhs.a \
    -o matcher.wasm \
    -s STANDALONE_WASM=1 \
    --no-entry \
    -s EXPORTED_FUNCTIONS='["_matcher_init","_matcher_match","_matcher_close"]'
```

## PR Steps

1. **Fork** the Vectorscan repository on GitHub

2. **Create a branch**:
   ```bash
   git checkout -b feature/wasm-emscripten-support
   ```

3. **Apply changes** from `docker/vectorscan-wasm.patch`:
   ```bash
   git apply ../vectorscan-wasm.patch
   ```

4. **Add documentation** to README or BUILDING.md:
   - Add "WebAssembly" to supported platforms section
   - Document build requirements (Emscripten 3.1.50+, Binaryen)
   - Add build instructions

5. **Add CI test** (optional but recommended):
   ```yaml
   # .github/workflows/wasm.yml
   name: WASM Build
   on: [push, pull_request]
   jobs:
     build:
       runs-on: ubuntu-latest
       container: emscripten/emsdk:3.1.50
       steps:
         - uses: actions/checkout@v4
           with:
             submodules: recursive
         - run: |
             emcmake cmake -B build -DFAT_RUNTIME=OFF
             emmake make -C build -j$(nproc) hs
   ```

6. **Commit** with descriptive message:
   ```
   Add WebAssembly/Emscripten build support

   - Detect EMSCRIPTEN in CMakeLists.txt
   - Enable SIMDe backend for WASM builds
   - Clear invalid arch-specific compiler flags
   - Tested with Emscripten 3.1.50+ and 4.0.x

   This enables building Vectorscan as a static library (libhs.a)
   that can be linked into WASM modules for browser or WASI usage.
   ```

7. **Push and create PR**:
   ```bash
   git push origin feature/wasm-emscripten-support
   ```

8. **PR Description** should include:
   - Motivation: WASM enables Vectorscan in browsers, serverless, edge
   - Changes made (reference this document)
   - Testing performed (Docker build, wasmtime execution)
   - Example usage with working code

## Testing the PR

Before submitting, verify:

1. **Library builds successfully**:
   ```bash
   ls -la build-wasm/lib/libhs.a
   ```

2. **Module loads in wasmtime**:
   ```bash
   wasmtime compile -W exceptions=y -W gc=y matcher.wasm
   ```

3. **Patterns compile and match**:
   ```bash
   ./matcher -p 'hello,\d+,[a-z]+' -i 'hello 123 abc' -v
   ```

## Notes

- The SIMDe dependency is already in the repository as a submodule
- No new dependencies are introduced
- WASM builds are optional - existing builds are unaffected
- Tested with Emscripten 3.1.50 and 4.0.20

## References

- [SIMDe - SIMD Everywhere](https://github.com/simd-everywhere/simde)
- [Emscripten SIMD](https://emscripten.org/docs/porting/simd.html)
- [WebAssembly Exception Handling](https://github.com/WebAssembly/exception-handling)
