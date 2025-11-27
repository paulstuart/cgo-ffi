// Package wasmvs provides a WASM-based Vectorscan matcher using wazero runtime.
package wasmvs

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed matcher.wasm
var wasmBytes []byte

// WasmMatcher implements multi-pattern matching using Vectorscan compiled to WASM.
type WasmMatcher struct {
	runtime wazero.Runtime
	module  api.Module
	ctx     context.Context

	// Exported functions
	wasmAlloc     api.Function
	wasmFree      api.Function
	matcherInit   api.Function
	matcherMatch  api.Function
	matcherClose  api.Function
	patternCount  api.Function

	patterns []string
	mu       sync.Mutex
}

// NewWasmMatcher creates a new WASM-based Vectorscan matcher.
func NewWasmMatcher(patterns []string) (*WasmMatcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}

	ctx := context.Background()

	// Create wazero runtime
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI for standalone WASM support
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	// Compile and instantiate the WASM module
	module, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Call _initialize if it exists (required for standalone WASM reactors)
	if initialize := module.ExportedFunction("_initialize"); initialize != nil {
		if _, err := initialize.Call(ctx); err != nil {
			module.Close(ctx)
			runtime.Close(ctx)
			return nil, fmt.Errorf("failed to call _initialize: %w", err)
		}
	}

	m := &WasmMatcher{
		runtime:  runtime,
		module:   module,
		ctx:      ctx,
		patterns: patterns,
	}

	// Get exported functions
	m.wasmAlloc = module.ExportedFunction("wasm_alloc")
	m.wasmFree = module.ExportedFunction("wasm_free")
	m.matcherInit = module.ExportedFunction("matcher_init")
	m.matcherMatch = module.ExportedFunction("matcher_match")
	m.matcherClose = module.ExportedFunction("matcher_close")
	m.patternCount = module.ExportedFunction("matcher_pattern_count")

	if m.wasmAlloc == nil || m.wasmFree == nil || m.matcherInit == nil ||
		m.matcherMatch == nil || m.matcherClose == nil || m.patternCount == nil {
		m.Close()
		return nil, fmt.Errorf("missing required WASM exports")
	}

	// Initialize with patterns
	if err := m.initPatterns(patterns); err != nil {
		m.Close()
		return nil, fmt.Errorf("failed to initialize patterns: %w", err)
	}

	return m, nil
}

// initPatterns sends patterns to the WASM module
func (m *WasmMatcher) initPatterns(patterns []string) error {
	// Join patterns with newlines
	data := strings.Join(patterns, "\n")
	dataBytes := []byte(data)

	// Allocate memory in WASM
	results, err := m.wasmAlloc.Call(m.ctx, uint64(len(dataBytes)))
	if err != nil {
		return fmt.Errorf("wasm_alloc failed: %w", err)
	}
	ptr := uint32(results[0])

	// Write patterns to WASM memory
	if !m.module.Memory().Write(ptr, dataBytes) {
		return fmt.Errorf("failed to write patterns to WASM memory")
	}

	// Call matcher_init
	results, err = m.matcherInit.Call(m.ctx, uint64(ptr), uint64(len(dataBytes)))
	if err != nil {
		return fmt.Errorf("matcher_init failed: %w", err)
	}

	// Free the temporary buffer
	m.wasmFree.Call(m.ctx, uint64(ptr), uint64(len(dataBytes)))

	if int32(results[0]) != 0 {
		return fmt.Errorf("matcher_init returned error code: %d", int32(results[0]))
	}

	return nil
}

// Match returns the index of the first matching pattern, or -1 if no match.
func (m *WasmMatcher) Match(input string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	inputBytes := []byte(input)

	// Allocate memory for input
	results, err := m.wasmAlloc.Call(m.ctx, uint64(len(inputBytes)))
	if err != nil {
		return -1
	}
	ptr := uint32(results[0])

	// Write input to WASM memory
	if !m.module.Memory().Write(ptr, inputBytes) {
		m.wasmFree.Call(m.ctx, uint64(ptr), uint64(len(inputBytes)))
		return -1
	}

	// Call matcher_match
	results, err = m.matcherMatch.Call(m.ctx, uint64(ptr), uint64(len(inputBytes)))
	if err != nil {
		m.wasmFree.Call(m.ctx, uint64(ptr), uint64(len(inputBytes)))
		return -1
	}

	// Free input buffer
	m.wasmFree.Call(m.ctx, uint64(ptr), uint64(len(inputBytes)))

	return int(int32(results[0]))
}

// MatchAll returns indices of all matching patterns.
// Note: This is a simplified implementation that just returns the first match.
func (m *WasmMatcher) MatchAll(input string) []int {
	result := m.Match(input)
	if result < 0 {
		return nil
	}
	return []int{result}
}

// PatternCount returns the number of patterns.
func (m *WasmMatcher) PatternCount() int {
	return len(m.patterns)
}

// Close releases WASM resources.
func (m *WasmMatcher) Close() {
	if m.matcherClose != nil {
		m.matcherClose.Call(m.ctx)
	}
	if m.module != nil {
		m.module.Close(m.ctx)
	}
	if m.runtime != nil {
		m.runtime.Close(m.ctx)
	}
}
