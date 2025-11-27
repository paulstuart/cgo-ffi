package host

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// WASM module paths (relative to test execution directory)
var wasmPaths = map[WasmRuntime]string{
	RuntimeRust:   "../rust/vector.wasm",
	RuntimeTinyGo: "../tinygo/vector.wasm",
	RuntimeC:      "../c/vector.wasm",
}

// Helper to create random test data
func makeData(n int) []float64 {
	data := make([]float64, n)
	for i := range data {
		data[i] = rand.Float64() * 100
	}
	return data
}

// Helper to get absolute path from relative
func getWasmPath(runtime WasmRuntime) string {
	// Get the directory of this test file
	_, err := os.Getwd()
	if err != nil {
		return wasmPaths[runtime]
	}
	return wasmPaths[runtime]
}

// loadWasmOps loads a WASM module if it exists
func loadWasmOps(t testing.TB, runtime WasmRuntime) *WasmVectorOps {
	path := getWasmPath(runtime)
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Skipf("cannot resolve path for %s: %v", runtime, err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("WASM module not found: %s (run build script first)", absPath)
	}

	ops, err := NewWasmVectorOpsFromFile(absPath)
	if err != nil {
		t.Fatalf("failed to load %s WASM: %v", runtime, err)
	}
	return ops
}

// --- Pure Go reference implementations (for correctness comparison) ---

func goSum(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum
}

func goDot(a, b []float64) float64 {
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

// --- Correctness Tests ---

func testSumCorrectness(t *testing.T, runtime WasmRuntime) {
	ops := loadWasmOps(t, runtime)
	defer ops.Close()

	data := makeData(1000)
	goResult := goSum(data)
	wasmResult := ops.Sum(data)

	if math.Abs(goResult-wasmResult) > 1e-9 {
		t.Errorf("%s Sum mismatch: Go=%v, WASM=%v", runtime, goResult, wasmResult)
	}
}

func testDotCorrectness(t *testing.T, runtime WasmRuntime) {
	ops := loadWasmOps(t, runtime)
	defer ops.Close()

	a := makeData(1000)
	b := makeData(1000)
	goResult := goDot(a, b)
	wasmResult := ops.Dot(a, b)

	if math.Abs(goResult-wasmResult) > 1e-6 {
		t.Errorf("%s Dot mismatch: Go=%v, WASM=%v", runtime, goResult, wasmResult)
	}
}

func TestSumCorrectness_Rust(t *testing.T)   { testSumCorrectness(t, RuntimeRust) }
func TestSumCorrectness_TinyGo(t *testing.T) { testSumCorrectness(t, RuntimeTinyGo) }
func TestSumCorrectness_C(t *testing.T)      { testSumCorrectness(t, RuntimeC) }

func TestDotCorrectness_Rust(t *testing.T)   { testDotCorrectness(t, RuntimeRust) }
func TestDotCorrectness_TinyGo(t *testing.T) { testDotCorrectness(t, RuntimeTinyGo) }
func TestDotCorrectness_C(t *testing.T)      { testDotCorrectness(t, RuntimeC) }

// --- Benchmarks ---

// Benchmark helpers
func benchmarkWasmSum(b *testing.B, runtime WasmRuntime, n int) {
	ops := loadWasmOps(b, runtime)
	defer ops.Close()

	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func benchmarkWasmSumSIMD(b *testing.B, runtime WasmRuntime, n int) {
	ops := loadWasmOps(b, runtime)
	defer ops.Close()

	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.SumSIMD(data)
	}
}

func benchmarkWasmDot(b *testing.B, runtime WasmRuntime, n int) {
	ops := loadWasmOps(b, runtime)
	defer ops.Close()

	a := makeData(n)
	c := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Dot(a, c)
	}
}

func benchmarkWasmMul(b *testing.B, runtime WasmRuntime, n int) {
	ops := loadWasmOps(b, runtime)
	defer ops.Close()

	a := makeData(n)
	c := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Mul(a, c)
	}
}

// Pure Go benchmarks (for comparison)
func benchmarkGoSum(b *testing.B, n int) {
	data := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goSum(data)
	}
}

func benchmarkGoDot(b *testing.B, n int) {
	a := makeData(n)
	c := makeData(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goDot(a, c)
	}
}

// --- Rust Benchmarks ---
func BenchmarkSum_Wasm_Rust_100(b *testing.B)    { benchmarkWasmSum(b, RuntimeRust, 100) }
func BenchmarkSum_Wasm_Rust_1000(b *testing.B)   { benchmarkWasmSum(b, RuntimeRust, 1000) }
func BenchmarkSum_Wasm_Rust_10000(b *testing.B)  { benchmarkWasmSum(b, RuntimeRust, 10000) }
func BenchmarkSum_Wasm_Rust_100000(b *testing.B) { benchmarkWasmSum(b, RuntimeRust, 100000) }

func BenchmarkSum_Wasm_Rust_SIMD_100(b *testing.B)    { benchmarkWasmSumSIMD(b, RuntimeRust, 100) }
func BenchmarkSum_Wasm_Rust_SIMD_1000(b *testing.B)   { benchmarkWasmSumSIMD(b, RuntimeRust, 1000) }
func BenchmarkSum_Wasm_Rust_SIMD_10000(b *testing.B)  { benchmarkWasmSumSIMD(b, RuntimeRust, 10000) }
func BenchmarkSum_Wasm_Rust_SIMD_100000(b *testing.B) { benchmarkWasmSumSIMD(b, RuntimeRust, 100000) }

func BenchmarkDot_Wasm_Rust_1000(b *testing.B)   { benchmarkWasmDot(b, RuntimeRust, 1000) }
func BenchmarkDot_Wasm_Rust_10000(b *testing.B)  { benchmarkWasmDot(b, RuntimeRust, 10000) }
func BenchmarkDot_Wasm_Rust_100000(b *testing.B) { benchmarkWasmDot(b, RuntimeRust, 100000) }

func BenchmarkMul_Wasm_Rust_10000(b *testing.B) { benchmarkWasmMul(b, RuntimeRust, 10000) }

// --- TinyGo Benchmarks ---
func BenchmarkSum_Wasm_TinyGo_100(b *testing.B)    { benchmarkWasmSum(b, RuntimeTinyGo, 100) }
func BenchmarkSum_Wasm_TinyGo_1000(b *testing.B)   { benchmarkWasmSum(b, RuntimeTinyGo, 1000) }
func BenchmarkSum_Wasm_TinyGo_10000(b *testing.B)  { benchmarkWasmSum(b, RuntimeTinyGo, 10000) }
func BenchmarkSum_Wasm_TinyGo_100000(b *testing.B) { benchmarkWasmSum(b, RuntimeTinyGo, 100000) }

func BenchmarkDot_Wasm_TinyGo_1000(b *testing.B)   { benchmarkWasmDot(b, RuntimeTinyGo, 1000) }
func BenchmarkDot_Wasm_TinyGo_10000(b *testing.B)  { benchmarkWasmDot(b, RuntimeTinyGo, 10000) }
func BenchmarkDot_Wasm_TinyGo_100000(b *testing.B) { benchmarkWasmDot(b, RuntimeTinyGo, 100000) }

func BenchmarkMul_Wasm_TinyGo_10000(b *testing.B) { benchmarkWasmMul(b, RuntimeTinyGo, 10000) }

// --- C Benchmarks ---
func BenchmarkSum_Wasm_C_100(b *testing.B)    { benchmarkWasmSum(b, RuntimeC, 100) }
func BenchmarkSum_Wasm_C_1000(b *testing.B)   { benchmarkWasmSum(b, RuntimeC, 1000) }
func BenchmarkSum_Wasm_C_10000(b *testing.B)  { benchmarkWasmSum(b, RuntimeC, 10000) }
func BenchmarkSum_Wasm_C_100000(b *testing.B) { benchmarkWasmSum(b, RuntimeC, 100000) }

func BenchmarkDot_Wasm_C_1000(b *testing.B)   { benchmarkWasmDot(b, RuntimeC, 1000) }
func BenchmarkDot_Wasm_C_10000(b *testing.B)  { benchmarkWasmDot(b, RuntimeC, 10000) }
func BenchmarkDot_Wasm_C_100000(b *testing.B) { benchmarkWasmDot(b, RuntimeC, 100000) }

func BenchmarkMul_Wasm_C_10000(b *testing.B) { benchmarkWasmMul(b, RuntimeC, 10000) }

// --- Go Reference Benchmarks (for WASM comparison) ---
func BenchmarkSum_Go_Ref_100(b *testing.B)    { benchmarkGoSum(b, 100) }
func BenchmarkSum_Go_Ref_1000(b *testing.B)   { benchmarkGoSum(b, 1000) }
func BenchmarkSum_Go_Ref_10000(b *testing.B)  { benchmarkGoSum(b, 10000) }
func BenchmarkSum_Go_Ref_100000(b *testing.B) { benchmarkGoSum(b, 100000) }

func BenchmarkDot_Go_Ref_1000(b *testing.B)   { benchmarkGoDot(b, 1000) }
func BenchmarkDot_Go_Ref_10000(b *testing.B)  { benchmarkGoDot(b, 10000) }
func BenchmarkDot_Go_Ref_100000(b *testing.B) { benchmarkGoDot(b, 100000) }

// --- Overhead Benchmarks (small data to measure call overhead) ---
func BenchmarkOverhead_Wasm_Rust(b *testing.B) {
	ops := loadWasmOps(b, RuntimeRust)
	defer ops.Close()
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func BenchmarkOverhead_Wasm_TinyGo(b *testing.B) {
	ops := loadWasmOps(b, RuntimeTinyGo)
	defer ops.Close()
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func BenchmarkOverhead_Wasm_C(b *testing.B) {
	ops := loadWasmOps(b, RuntimeC)
	defer ops.Close()
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops.Sum(data)
	}
}

func BenchmarkOverhead_Go_Ref(b *testing.B) {
	data := makeData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = goSum(data)
	}
}
