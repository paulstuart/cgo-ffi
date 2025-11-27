// C implementation of vector operations for WASM
//
// Uses pre-allocated static buffers to eliminate per-call allocation.
// The host copies data into these buffers at known offsets.
//
// Build with wasi-sdk:
//   $WASI_SDK/bin/clang --target=wasm32-wasi -O3 -nostartfiles \
//     -Wl,--no-entry -Wl,--export-all -o vector.wasm vector_wasm.c
//
// Or with Emscripten:
//   emcc -O3 -s STANDALONE_WASM=1 -s EXPORTED_FUNCTIONS='["_sum","_dot",...]' \
//     --no-entry -o vector.wasm vector_wasm.c

#include <stdint.h>
#include <stddef.h>

// Pre-allocated buffer capacity (100K f64 elements = 800KB per buffer)
#define CAPACITY 100000

// Static buffers - allocated once, stable addresses
static double buffer_a[CAPACITY];
static double buffer_b[CAPACITY];
static double result_buf[CAPACITY];

// WASM export attribute
#define WASM_EXPORT __attribute__((visibility("default")))

WASM_EXPORT double sum(uint32_t len) {
    size_t n = len < CAPACITY ? len : CAPACITY;
    double s = 0.0;
    for (size_t i = 0; i < n; i++) {
        s += buffer_a[i];
    }
    return s;
}

WASM_EXPORT double dot(uint32_t len) {
    size_t n = len < CAPACITY ? len : CAPACITY;
    double d = 0.0;
    for (size_t i = 0; i < n; i++) {
        d += buffer_a[i] * buffer_b[i];
    }
    return d;
}

WASM_EXPORT void mul(uint32_t len) {
    size_t n = len < CAPACITY ? len : CAPACITY;
    for (size_t i = 0; i < n; i++) {
        result_buf[i] = buffer_a[i] * buffer_b[i];
    }
}

WASM_EXPORT void scale(double scalar, uint32_t len) {
    size_t n = len < CAPACITY ? len : CAPACITY;
    for (size_t i = 0; i < n; i++) {
        buffer_a[i] *= scalar;
    }
}

WASM_EXPORT double sum_simd(uint32_t len) {
    size_t n = len < CAPACITY ? len : CAPACITY;
    // 4-way unrolling for better auto-vectorization
    double sum0 = 0.0, sum1 = 0.0, sum2 = 0.0, sum3 = 0.0;
    size_t i = 0;

    for (; i + 3 < n; i += 4) {
        sum0 += buffer_a[i];
        sum1 += buffer_a[i + 1];
        sum2 += buffer_a[i + 2];
        sum3 += buffer_a[i + 3];
    }
    for (; i < n; i++) {
        sum0 += buffer_a[i];
    }
    return sum0 + sum1 + sum2 + sum3;
}

WASM_EXPORT uint32_t get_buffer_a_offset(void) {
    return (uint32_t)(uintptr_t)buffer_a;
}

WASM_EXPORT uint32_t get_buffer_b_offset(void) {
    return (uint32_t)(uintptr_t)buffer_b;
}

WASM_EXPORT uint32_t get_result_offset(void) {
    return (uint32_t)(uintptr_t)result_buf;
}

WASM_EXPORT uint32_t get_capacity(void) {
    return CAPACITY;
}
