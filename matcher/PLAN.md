# Matcher: High-Performance Multi-Pattern Regex Matching

## Goal

Create a function that evaluates an incoming string against an array of regexps:

- **Input**: A string to match against
- **Output**: Index of the first matching regexp, or -1 if no match
- **Initialization**: A vector of strings, each containing a valid regexp

Following the project's principles:

- Pre-allocate buffers at initialization
- Minimize per-call overhead (no malloc/free on hot path)
- Compare WASM implementation against pure Go baseline

## Architecture

```
matcher/
├── PLAN.md              # This document
├── go/
│   └── matcher.go       # Pure Go naive implementation (baseline)
├── wasm/
│   ├── build.sh         # Build script for WASM modules
│   └── vectorscan/      # Vectorscan compiled to WASM (if feasible)
├── host/
│   ├── matcher.go       # Go host for WASM matcher
│   └── matcher_test.go  # Tests and benchmarks
└── cmd/
    └── main.go          # Demo/benchmark runner
```

## API Design

```go
// Matcher interface - implemented by both Go and WASM backends
type Matcher interface {
    // Match returns the index of the first matching pattern, or -1
    Match(input string) int

    // MatchAll returns indices of all matching patterns
    // TODO: I think all that matters is first match
    // MatchAll(input string) []int

    // Close releases resources
    Close()
}

// NewGoMatcher creates a pure Go regex matcher (baseline)
func NewGoMatcher(patterns []string) (Matcher, error)

// NewWasmMatcher creates a WASM-backed matcher (Vectorscan/RE2)
func NewWasmMatcher(wasmPath string, patterns []string) (Matcher, error)
```

## Implementation 1: Pure Go Baseline

Simple implementation using Go's `regexp` package:

```go
type GoMatcher struct {
    patterns []*regexp.Regexp
}

func (m *GoMatcher) Match(input string) int {
    for i, re := range m.patterns {
        if re.MatchString(input) {
            return i
        }
    }
    return -1
}
```

**Characteristics:**

- Sequential pattern matching (O(n) where n = pattern count)
- Uses Go's RE2-based regexp engine
- No ReDoS vulnerability (RE2 guarantees linear time)
- Good baseline for comparison

## Implementation 2: Vectorscan WASM

### What is Vectorscan/Hyperscan?

[Vectorscan](https://github.com/VectorCamp/vectorscan) is a fork of Intel's [Hyperscan](https://intel.github.io/hyperscan/dev-reference/), a high-performance regex matching library that:

- Matches **multiple patterns simultaneously** using hybrid automata
- Scales to **tens of thousands** of patterns
- Uses **SIMD instructions** for performance
- Provides **streaming** and **block** mode matching

### Hyperscan C API

Key functions we need to bind:

```c
// Compile multiple patterns into a database
hs_error_t hs_compile_multi(
    const char *const *expressions,  // Array of pattern strings
    const unsigned int *flags,       // Per-pattern flags (can be NULL)
    const unsigned int *ids,         // Per-pattern IDs (can be NULL)
    unsigned int elements,           // Number of patterns
    unsigned int mode,               // HS_MODE_BLOCK for our use case
    const hs_platform_info_t *platform,
    hs_database_t **db,
    hs_compile_error_t **error
);

// Allocate scratch space (reusable across scans)
hs_error_t hs_alloc_scratch(
    const hs_database_t *db,
    hs_scratch_t **scratch
);

// Scan a block of data
hs_error_t hs_scan(
    const hs_database_t *db,
    const char *data,
    unsigned int length,
    unsigned int flags,
    hs_scratch_t *scratch,
    match_event_handler onEvent,
    void *context
);

// Match callback signature
typedef int (*match_event_handler)(
    unsigned int id,           // Pattern ID that matched
    unsigned long long from,   // Start offset (if SOM enabled)
    unsigned long long to,     // End offset
    unsigned int flags,
    void *context
);
```

### Challenge: Compiling Vectorscan to WASM

**This is non-trivial.** Vectorscan has:

1. **Complex dependencies**: Boost, PCRE, SQLite (for tools)
2. **Heavy SIMD usage**: AVX2/AVX512 on x86, NEON on ARM
3. **Large codebase**: ~100K lines of C++
4. **No existing WASM port**: No known Emscripten build

**Potential approach using SIMDe backend:**

Vectorscan 5.4.12+ includes a [SIMDe](https://github.com/simd-everywhere/simde) backend that emulates SIMD instructions. This could theoretically work with Emscripten's SIMD support:

```bash
# Theoretical build (untested)
emcmake cmake -S vectorscan -B build \
    -DSIMDE_BACKEND=ON \
    -DFAT_RUNTIME=OFF \
    -DBUILD_SHARED_LIBS=OFF \
    -DCMAKE_BUILD_TYPE=Release

emmake make -C build
```

**Estimated effort**: High (weeks). Would need to:

- Stub out or port Boost dependencies
- Handle C++ exceptions in WASM
- Deal with memory allocation patterns
- Test and debug extensively

## Alternative: RE2 WASM

If Vectorscan proves too difficult, consider [RE2](https://github.com/google/re2):

**Existing projects:**
- [google/re2-wasm](https://github.com/google/re2-wasm) - Official RE2 WASM build for Node.js
- [wasilibs/go-re2](https://github.com/wasilibs/go-re2) - Go drop-in using RE2 via wazero

**Advantages:**
- Proven WASM compilation
- Same RE2 engine as Go's regexp (consistent behavior)
- Simpler than Vectorscan

**Disadvantages:**
- Single-pattern matching (need to compile each pattern separately)
- No simultaneous multi-pattern matching like Hyperscan
- May not provide significant speedup over Go's regexp

## Proposed Phases

### Phase 1: Pure Go Baseline
1. Implement `GoMatcher` with sequential regexp matching
2. Add benchmarks with various pattern counts (10, 100, 1000)
3. Test with different input sizes and pattern complexities

### Phase 2: Evaluate Vectorscan WASM Feasibility
1. Clone Vectorscan and attempt SIMDe + Emscripten build
2. Document build challenges and solutions
3. If successful, create minimal WASM module with:
   - `init(patterns)` - Compile patterns and allocate scratch
   - `match(input)` - Return first matching pattern ID
   - `match_all(input)` - Return all matching pattern IDs

### Phase 3: Implement WASM Host (if Phase 2 succeeds)

1. Create Go host using wasmtime-go
2. Pre-allocate input buffer in WASM linear memory
3. Implement same `Matcher` interface
4. Benchmark against Go baseline

### Phase 4: Alternative (if Phase 2 fails)

1. Use wasilibs/go-re2 or build RE2 WASM ourselves
2. Compare RE2 WASM vs Go regexp
3. Document findings

## Expected Performance Characteristics

| Implementation | Pattern Compile | Per-Match (10 patterns) | Per-Match (1000 patterns) |
|----------------|-----------------|------------------------|---------------------------|
| Go regexp | Fast | O(n) sequential | O(n) sequential |
| Vectorscan WASM | Slow (DFA build) | O(1) simultaneous | O(1) simultaneous |
| RE2 WASM | Medium | Similar to Go | Similar to Go |

**Key insight**: Vectorscan's advantage is **simultaneous multi-pattern matching**. For few patterns, Go may win due to WASM overhead. For many patterns (100+), Vectorscan should dominate.

## Memory Model for WASM

Following the project's pre-allocated buffer pattern:

```
WASM Linear Memory:
┌─────────────────────────────────────────────┐
│ [0x1000] input_buffer (pre-allocated, 64KB) │
│ [0x11000] match_results (pattern IDs)       │
│ [0x12000] scratch_space (Hyperscan scratch) │
│ [0x?????] compiled_db (Hyperscan database)  │
└─────────────────────────────────────────────┘

Per-call flow:
1. Host copies input string to input_buffer
2. Host calls match(input_len)
3. WASM scans using pre-allocated scratch
4. WASM writes matching ID(s) to match_results
5. Host reads result (no copy needed for single int)
```

## Questions for Review

1. **Vectorscan vs RE2**: Should we pursue Vectorscan despite complexity, or start with RE2 WASM as a more achievable target?

2. **Pattern count**: What's the target use case - few patterns (10s) or many (1000s)? This affects which engine makes sense.

3. **Match semantics**: Should `Match()` return on first match, or scan entire input? (Hyperscan can do either)

4. **Streaming support**: Do we need streaming mode (match across chunked input) or is block mode sufficient?

## References

- [Vectorscan GitHub](https://github.com/VectorCamp/vectorscan)
- [Hyperscan Developer Reference](https://intel.github.io/hyperscan/dev-reference/)
- [Hyperscan simplegrep example](https://github.com/intel/hyperscan/tree/master/examples)
- [wasilibs/go-re2](https://github.com/wasilibs/go-re2)
- [google/re2-wasm](https://github.com/google/re2-wasm)
- [SIMDe - SIMD Everywhere](https://github.com/simd-everywhere/simde)
