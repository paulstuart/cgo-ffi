// vector.c - C implementations of vector operations
#include "vector.h"

// Simple sum
double vector_sum(const double* arr, size_t len) {
    double sum = 0.0;
    for (size_t i = 0; i < len; i++) {
        sum += arr[i];
    }
    return sum;
}

// Dot product
double vector_dot(const double* a, const double* b, size_t len) {
    double dot = 0.0;
    for (size_t i = 0; i < len; i++) {
        dot += a[i] * b[i];
    }
    return dot;
}

// Element-wise multiply
void vector_mul(const double* a, const double* b, double* result, size_t len) {
    for (size_t i = 0; i < len; i++) {
        result[i] = a[i] * b[i];
    }
}

// Scale in-place
void vector_scale(double* arr, double scalar, size_t len) {
    for (size_t i = 0; i < len; i++) {
        arr[i] *= scalar;
    }
}

// SIMD-optimized sum using loop unrolling
// Compilers with -O2/-O3 will auto-vectorize this
double vector_sum_simd(const double* arr, size_t len) {
    double sum0 = 0.0, sum1 = 0.0, sum2 = 0.0, sum3 = 0.0;
    size_t i = 0;

    // Process 4 elements at a time
    for (; i + 3 < len; i += 4) {
        sum0 += arr[i];
        sum1 += arr[i + 1];
        sum2 += arr[i + 2];
        sum3 += arr[i + 3];
    }

    // Handle remainder
    for (; i < len; i++) {
        sum0 += arr[i];
    }

    return sum0 + sum1 + sum2 + sum3;
}
