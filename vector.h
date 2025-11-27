// vector.h - C functions for vector operations
#ifndef VECTOR_H
#define VECTOR_H

#include <stdint.h>
#include <stddef.h>

// Sum all elements in a float64 array
double vector_sum(const double* arr, size_t len);

// Dot product of two float64 arrays
double vector_dot(const double* a, const double* b, size_t len);

// Element-wise multiply: result[i] = a[i] * b[i]
void vector_mul(const double* a, const double* b, double* result, size_t len);

// Scale array in-place: arr[i] *= scalar
void vector_scale(double* arr, double scalar, size_t len);

// SIMD-optimized sum (if available)
double vector_sum_simd(const double* arr, size_t len);

#endif
