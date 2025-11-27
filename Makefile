# Makefile for cgo-ffi project
#
# Targets:
#   make build      - Build Go code and cgo components
#   make wasm       - Build all WASM modules (requires Rust, TinyGo, wasi-sdk)
#   make wasmx      - Build all WASM runner in wasm dir
#   make test       - Run all tests
#   make bench      - Run all benchmarks
#   make bench-cgo  - Run only cgo benchmarks
#   make bench-wasm - Run only WASM benchmarks
#   make matchx     - Build all regexp matcher
#   make clean      - Clean build artifacts

.PHONY: all build wasm test bench bench-cgo bench-wasm clean help demo wasmx wasmg matchx

all: build

# Build Go code with cgo
build:
	go build ./...

# Build demo app
demo:
	go build -o demo ./cmd/.

# Build WASM runner
wasmx:
	go build -o wasm/wasmx ./wasm/cmd

# Build WASM runner
matchx:
	go build -o matcher/matchx ./matcher/cmd

# Build all WASM modules
wasm:
	chmod +x wasm/build.sh
	cd wasm && ./build.sh all

wasm-rust:
	chmod +x wasm/build.sh
	cd wasm && ./build.sh rust

wasm-tinygo:
	chmod +x wasm/build.sh
	cd wasm && ./build.sh tinygo

wasmg:
	GOOS=js GOARCH=wasm go build -o std.wasm ./wasm/tinygo

wasm-c:
	chmod +x wasm/build.sh
	cd wasm && ./build.sh c

# Run all tests
test: build
	go test -v ./...

# Run tests only (skip if WASM not built)
test-cgo:
	go test -v -run 'Test.*Correctness' .

test-wasm:
	cd wasm/host && go test -v -run 'Test.*'

# Run all benchmarks
bench: build
	@echo "=== CGO Benchmarks ==="
	go test -bench=. -benchmem -run=^$$ .
	@echo ""
	@echo "=== WASM Benchmarks ==="
	cd wasm/host && go test -bench=. -benchmem -run=^$$

# Run only cgo benchmarks
bench-cgo:
	go test -bench=. -benchmem -run=^$$ .

# Run only WASM benchmarks
bench-wasm:
	cd wasm/host && go test -bench=. -benchmem -run=^$$

# Run comparative benchmarks (all implementations at same sizes)
bench-compare:
	@echo "=== Sum 10K elements ==="
	go test -bench='Sum.*10000' -benchmem -run=^$$ .
	cd wasm/host && go test -bench='Sum.*10000' -benchmem -run=^$$
	@echo ""
	@echo "=== Sum 100K elements ==="
	go test -bench='Sum.*100000' -benchmem -run=^$$ .
	cd wasm/host && go test -bench='Sum.*100000' -benchmem -run=^$$

# Run cgo demo
demo-run:
	go run ./cmd

# Run WASM demo
demo-wasm:
	cd wasm && go run ./cmd

# Clean build artifacts
clean:
	go clean ./...
	rm -f wasm/rust/target/wasm32-wasip1/release/*.wasm
	rm -f wasm/tinygo/*.wasm
	rm -f wasm/c/*.wasm

# Update dependencies
deps:
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build       - Build Go code with cgo"
	@echo "  wasm        - Build all WASM modules"
	@echo "  wasm-rust   - Build Rust WASM only"
	@echo "  wasm-tinygo - Build TinyGo WASM only"
	@echo "  wasm-c      - Build C WASM only"
	@echo "  test        - Run all tests"
	@echo "  test-cgo    - Run cgo tests only"
	@echo "  test-wasm   - Run WASM tests only"
	@echo "  bench       - Run all benchmarks"
	@echo "  bench-cgo   - Run cgo benchmarks only"
	@echo "  bench-wasm  - Run WASM benchmarks only"
	@echo "  bench-compare - Compare implementations at same sizes"
	@echo "  demo        - Run cgo interactive demo"
	@echo "  demo-wasm   - Run WASM interactive demo"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Update Go dependencies"
