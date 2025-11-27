# WASM Vectorscan Implementation

This directory will contain the WebAssembly build of Vectorscan for comparison against:
- Pure Go sequential matching
- Native Vectorscan via cgo bindings

## Current Status

**Not yet implemented.** Building Vectorscan to WASM is non-trivial.

## Challenges

1. **Large C++ codebase** (~100K lines)
2. **Complex dependencies**: Boost headers, PCRE (optional)
3. **SIMD usage**: Heavy use of AVX2/AVX512 on x86, NEON on ARM
4. **No existing WASM port**: Would be first known Emscripten build

## Potential Approach

Vectorscan 5.4.12+ includes a [SIMDe](https://github.com/simd-everywhere/simde) backend that emulates SIMD instructions. This could work with Emscripten's SIMD support:

```bash
# Theoretical build (untested)
git clone https://github.com/VectorCamp/vectorscan.git
cd vectorscan

emcmake cmake -B build \
    -DSIMDE_BACKEND=ON \
    -DFAT_RUNTIME=OFF \
    -DBUILD_SHARED_LIBS=OFF \
    -DCMAKE_BUILD_TYPE=Release

emmake make -C build
```

## Expected API

The WASM module would export:

```c
// Initialize with patterns (called once)
int hs_init(const char** patterns, int count);

// Match input against all patterns
// Returns: first matching pattern ID, or -1
int hs_match(const char* input, int len);

// Free resources
void hs_close();
```

## Go Host Integration

Would follow the same pattern as `wasm/host/wasm.go`:
- Pre-allocate buffers in WASM linear memory
- Copy input string to WASM memory
- Call match function
- Read result (single int, no copy needed)

## Performance Expectations

| Implementation    | 256 patterns | Notes |
|-------------------|--------------|-------|
| Pure Go           | ~9K files/sec | O(n) sequential |
| Native Vectorscan | ~2.2M files/sec | O(1) simultaneous |
| WASM Vectorscan   | ~500K files/sec? | Estimated - TBD |

WASM overhead (memory copies, no native SIMD) may reduce Vectorscan's advantage, but should still significantly outperform Go for large pattern sets.

## References

- [Vectorscan GitHub](https://github.com/VectorCamp/vectorscan)
- [SIMDe - SIMD Everywhere](https://github.com/simd-everywhere/simde)
- [Emscripten SIMD](https://emscripten.org/docs/porting/simd.html)
