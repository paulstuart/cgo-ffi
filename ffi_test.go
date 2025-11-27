package ffi

import (
	"math"
	"math/rand"
	"testing"
)

// Test data sizes
var sizes = []int{100, 1000, 10000, 100000}

// Helper to create random test data
func makeData(n int) []float64 {
	data := make([]float64, n)
	for i := range data {
		data[i] = rand.Float64() * 100
	}
	return data
}

// --- Correctness Tests ---

func TestSumCorrectness(t *testing.T) {
	data := makeData(1000)

	goResult := GoSum(data)

	ops := NewVectorOps(len(data))
	defer ops.Close()
	cResult := ops.Sum(data)

	if math.Abs(goResult-cResult) > 1e-9 {
		t.Errorf("Sum mismatch: Go=%v, C=%v", goResult, cResult)
	}
}

func TestDotCorrectness(t *testing.T) {
	a := makeData(1000)
	b := makeData(1000)

	goResult := GoDot(a, b)

	ops := NewVectorOps(len(a))
	defer ops.Close()
	cResult := ops.Dot(a, b)

	if math.Abs(goResult-cResult) > 1e-6 {
		t.Errorf("Dot mismatch: Go=%v, C=%v", goResult, cResult)
	}
}

func TestMulCorrectness(t *testing.T) {
	a := makeData(1000)
	b := makeData(1000)

	goResult := GoMul(a, b)

	ops := NewVectorOps(len(a))
	defer ops.Close()
	cResult := ops.Mul(a, b)

	for i := range goResult {
		if math.Abs(goResult[i]-cResult[i]) > 1e-9 {
			t.Errorf("Mul mismatch at %d: Go=%v, C=%v", i, goResult[i], cResult[i])
			break
		}
	}
}

// --- Benchmarks ---

// BenchmarkSum compares sum implementations
func BenchmarkSum_Go_100(b *testing.B)       { benchmarkGoSum(b, 100) }
func BenchmarkSum_Go_1000(b *testing.B)      { benchmarkGoSum(b, 1000) }
func BenchmarkSum_Go_10000(b *testing.B)     { benchmarkGoSum(b, 10000) }
func BenchmarkSum_Go_100000(b *testing.B)    { benchmarkGoSum(b, 100000) }

func BenchmarkSum_GoUnrolled_100(b *testing.B)    { benchmarkGoSumUnrolled(b, 100) }
func BenchmarkSum_GoUnrolled_1000(b *testing.B)   { benchmarkGoSumUnrolled(b, 1000) }
func BenchmarkSum_GoUnrolled_10000(b *testing.B)  { benchmarkGoSumUnrolled(b, 10000) }
func BenchmarkSum_GoUnrolled_100000(b *testing.B) { benchmarkGoSumUnrolled(b, 100000) }

func BenchmarkSum_C_Optimized_100(b *testing.B)    { benchmarkCSum(b, 100) }
func BenchmarkSum_C_Optimized_1000(b *testing.B)   { benchmarkCSum(b, 1000) }
func BenchmarkSum_C_Optimized_10000(b *testing.B)  { benchmarkCSum(b, 10000) }
func BenchmarkSum_C_Optimized_100000(b *testing.B) { benchmarkCSum(b, 100000) }

func BenchmarkSum_C_SIMD_100(b *testing.B)    { benchmarkCSumSIMD(b, 100) }
func BenchmarkSum_C_SIMD_1000(b *testing.B)   { benchmarkCSumSIMD(b, 1000) }
func BenchmarkSum_C_SIMD_10000(b *testing.B)  { benchmarkCSumSIMD(b, 10000) }
func BenchmarkSum_C_SIMD_100000(b *testing.B) { benchmarkCSumSIMD(b, 100000) }

func BenchmarkSum_C_Direct_100(b *testing.B)    { benchmarkCDirect(b, 100) }
func BenchmarkSum_C_Direct_1000(b *testing.B)   { benchmarkCDirect(b, 1000) }
func BenchmarkSum_C_Direct_10000(b *testing.B)  { benchmarkCDirect(b, 10000) }
func BenchmarkSum_C_Direct_100000(b *testing.B) { benchmarkCDirect(b, 100000) }

func benchmarkGoSum(b *testing.B, n int) {
	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoSum(data)
	}
}

func benchmarkGoSumUnrolled(b *testing.B, n int) {
	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoSumUnrolled(data)
	}
}

func benchmarkCSum(b *testing.B, n int) {
	data := makeData(n)
	ops := NewVectorOps(n)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func benchmarkCSumSIMD(b *testing.B, n int) {
	data := makeData(n)
	ops := NewVectorOps(n)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.SumSIMD(data)
	}
}

func benchmarkCDirect(b *testing.B, n int) {
	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DirectSum(data)
	}
}

// BenchmarkDot compares dot product implementations
func BenchmarkDot_Go_1000(b *testing.B)      { benchmarkGoDot(b, 1000) }
func BenchmarkDot_Go_10000(b *testing.B)     { benchmarkGoDot(b, 10000) }
func BenchmarkDot_Go_100000(b *testing.B)    { benchmarkGoDot(b, 100000) }

func BenchmarkDot_GoUnrolled_1000(b *testing.B)   { benchmarkGoDotUnrolled(b, 1000) }
func BenchmarkDot_GoUnrolled_10000(b *testing.B)  { benchmarkGoDotUnrolled(b, 10000) }
func BenchmarkDot_GoUnrolled_100000(b *testing.B) { benchmarkGoDotUnrolled(b, 100000) }

func BenchmarkDot_C_Optimized_1000(b *testing.B)   { benchmarkCDot(b, 1000) }
func BenchmarkDot_C_Optimized_10000(b *testing.B)  { benchmarkCDot(b, 10000) }
func BenchmarkDot_C_Optimized_100000(b *testing.B) { benchmarkCDot(b, 100000) }

func BenchmarkDot_C_Direct_1000(b *testing.B)   { benchmarkCDotDirect(b, 1000) }
func BenchmarkDot_C_Direct_10000(b *testing.B)  { benchmarkCDotDirect(b, 10000) }
func BenchmarkDot_C_Direct_100000(b *testing.B) { benchmarkCDotDirect(b, 100000) }

func benchmarkGoDot(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoDot(a, c)
	}
}

func benchmarkGoDotUnrolled(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoDotUnrolled(a, c)
	}
}

func benchmarkCDot(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	ops := NewVectorOps(n)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Dot(a, c)
	}
}

func benchmarkCDotDirect(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DirectDot(a, c)
	}
}

// BenchmarkMul compares element-wise multiplication
func BenchmarkMul_Go_10000(b *testing.B)          { benchmarkGoMul(b, 10000) }
func BenchmarkMul_GoInto_10000(b *testing.B)      { benchmarkGoMulInto(b, 10000) }
func BenchmarkMul_C_Optimized_10000(b *testing.B) { benchmarkCMul(b, 10000) }
func BenchmarkMul_C_Into_10000(b *testing.B)      { benchmarkCMulInto(b, 10000) }

func benchmarkGoMul(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoMul(a, c)
	}
}

func benchmarkGoMulInto(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	dst := make([]float64, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GoMulInto(a, c, dst)
	}
}

func benchmarkCMul(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	ops := NewVectorOps(n)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Mul(a, c)
	}
}

func benchmarkCMulInto(b *testing.B, n int) {
	a, c := makeData(n), makeData(n)
	dst := make([]float64, n)
	ops := NewVectorOps(n)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops.MulInto(a, c, dst)
	}
}

// BenchmarkOverhead measures the FFI call overhead itself
func BenchmarkOverhead_Go_Empty(b *testing.B) {
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GoSum(data)
	}
}

func BenchmarkOverhead_C_Optimized_Empty(b *testing.B) {
	data := makeData(10)
	ops := NewVectorOps(10)
	defer ops.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func BenchmarkOverhead_C_Direct_Empty(b *testing.B) {
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DirectSum(data)
	}
}
