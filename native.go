package ffi

// Pure Go implementations for comparison benchmarks

// GoSum computes sum using pure Go.
func GoSum(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum
}

// GoSumUnrolled uses loop unrolling for better performance.
func GoSumUnrolled(data []float64) float64 {
	var sum0, sum1, sum2, sum3 float64
	n := len(data)
	i := 0

	// Process 4 elements at a time
	for ; i+3 < n; i += 4 {
		sum0 += data[i]
		sum1 += data[i+1]
		sum2 += data[i+2]
		sum3 += data[i+3]
	}

	// Handle remainder
	for ; i < n; i++ {
		sum0 += data[i]
	}

	return sum0 + sum1 + sum2 + sum3
}

// GoDot computes dot product using pure Go.
func GoDot(a, b []float64) float64 {
	var dot float64
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	return dot
}

// GoDotUnrolled uses loop unrolling.
func GoDotUnrolled(a, b []float64) float64 {
	var dot0, dot1, dot2, dot3 float64
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0

	for ; i+3 < n; i += 4 {
		dot0 += a[i] * b[i]
		dot1 += a[i+1] * b[i+1]
		dot2 += a[i+2] * b[i+2]
		dot3 += a[i+3] * b[i+3]
	}

	for ; i < n; i++ {
		dot0 += a[i] * b[i]
	}

	return dot0 + dot1 + dot2 + dot3
}

// GoMul performs element-wise multiplication.
func GoMul(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = a[i] * b[i]
	}
	return result
}

// GoMulInto performs element-wise multiplication into dst.
func GoMulInto(a, b, dst []float64) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if len(dst) < n {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		dst[i] = a[i] * b[i]
	}
}

// GoScale multiplies all elements by scalar in-place.
func GoScale(data []float64, scalar float64) {
	for i := range data {
		data[i] *= scalar
	}
}
