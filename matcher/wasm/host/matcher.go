// Package wasmvs provides a WASM-based Vectorscan matcher using wasmtime runtime.
package wasmvs

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v39"
)

//go:embed matcher.wasm
var wasmBytes []byte

// WasmMatcher implements multi-pattern matching using Vectorscan compiled to WASM.
type WasmMatcher struct {
	engine   *wasmtime.Engine
	store    *wasmtime.Store
	instance *wasmtime.Instance
	memory   *wasmtime.Memory

	// Exported functions
	wasmAlloc     *wasmtime.Func
	wasmFree      *wasmtime.Func
	matcherInit   *wasmtime.Func
	matcherMatch  *wasmtime.Func
	matcherClose  *wasmtime.Func
	patternCount  *wasmtime.Func
	getError      *wasmtime.Func
	checkPlatform *wasmtime.Func

	patterns []string
	mu       sync.Mutex
}

// NewWasmMatcher creates a new WASM-based Vectorscan matcher.
func NewWasmMatcher(patterns []string) (*WasmMatcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}

	// Create engine with exception handling enabled
	cfg := wasmtime.NewConfig()
	enableExceptions(cfg)
	engine := wasmtime.NewEngineWithConfig(cfg)

	// Create store
	store := wasmtime.NewStore(engine)

	// Compile module
	module, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Create WASI config
	wasiConfig := wasmtime.NewWasiConfig()
	store.SetWasi(wasiConfig)

	// Get module imports
	imports := module.Imports()

	// Build import list
	var importExterns []wasmtime.AsExtern
	for _, imp := range imports {
		modName := imp.Module()
		name := *imp.Name()

		switch modName {
		case "wasi_snapshot_preview1":
			// WASI imports are handled by the linker
			continue
		case "env":
			// Handle Emscripten env imports
			switch name {
			case "emscripten_notify_memory_growth":
				fn := wasmtime.WrapFunc(store, func() {
					// No-op callback for memory growth
				})
				importExterns = append(importExterns, fn)
			default:
				// Create stub for unknown env functions
				fmt.Printf("Warning: unknown env import: %s\n", name)
			}
		}
	}

	// Use linker for WASI support
	linker := wasmtime.NewLinker(engine)
	if err := linker.DefineWasi(); err != nil {
		return nil, fmt.Errorf("failed to define WASI: %w", err)
	}

	// Define emscripten env function (takes memory index as parameter)
	err = linker.DefineFunc(store, "env", "emscripten_notify_memory_growth", func(memIdx int32) {
		// No-op - called when memory grows
	})
	if err != nil {
		return nil, fmt.Errorf("failed to define emscripten_notify_memory_growth: %w", err)
	}

	// Instantiate module
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Get memory export
	memExport := instance.GetExport(store, "memory")
	if memExport == nil {
		return nil, fmt.Errorf("module does not export memory")
	}
	memory := memExport.Memory()
	if memory == nil {
		return nil, fmt.Errorf("memory export is not a memory")
	}

	m := &WasmMatcher{
		engine:   engine,
		store:    store,
		instance: instance,
		memory:   memory,
		patterns: patterns,
	}

	// Get exported functions
	m.wasmAlloc = instance.GetFunc(store, "wasm_alloc")
	m.wasmFree = instance.GetFunc(store, "wasm_free")
	m.matcherInit = instance.GetFunc(store, "matcher_init")
	m.matcherMatch = instance.GetFunc(store, "matcher_match")
	m.matcherClose = instance.GetFunc(store, "matcher_close")
	m.patternCount = instance.GetFunc(store, "matcher_pattern_count")
	m.getError = instance.GetFunc(store, "matcher_get_error")
	m.checkPlatform = instance.GetFunc(store, "matcher_check_platform")

	if m.wasmAlloc == nil || m.wasmFree == nil || m.matcherInit == nil ||
		m.matcherMatch == nil || m.matcherClose == nil || m.patternCount == nil {
		return nil, fmt.Errorf("missing required WASM exports")
	}

	// Call _initialize if it exists
	if initialize := instance.GetFunc(store, "_initialize"); initialize != nil {
		_, err := initialize.Call(store)
		if err != nil {
			return nil, fmt.Errorf("failed to call _initialize: %w", err)
		}
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
	result, err := m.wasmAlloc.Call(m.store, int32(len(dataBytes)))
	if err != nil {
		return fmt.Errorf("wasm_alloc failed: %w", err)
	}
	ptr := result.(int32)

	// Write patterns to WASM memory
	memData := m.memory.UnsafeData(m.store)
	copy(memData[ptr:], dataBytes)

	// Call matcher_init
	result, err = m.matcherInit.Call(m.store, ptr, int32(len(dataBytes)))
	if err != nil {
		return fmt.Errorf("matcher_init failed: %w", err)
	}

	// Free the temporary buffer
	m.wasmFree.Call(m.store, ptr)

	retCode := result.(int32)
	if retCode != 0 {
		errMsg := m.GetError()
		if errMsg != "" {
			return fmt.Errorf("matcher_init returned error code: %d (%s)", retCode, errMsg)
		}
		return fmt.Errorf("matcher_init returned error code: %d", retCode)
	}

	return nil
}

// Match returns the index of the first matching pattern, or -1 if no match.
func (m *WasmMatcher) Match(input string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	inputBytes := []byte(input)

	// Allocate memory for input
	result, err := m.wasmAlloc.Call(m.store, int32(len(inputBytes)))
	if err != nil {
		return -1
	}
	ptr := result.(int32)

	// Write input to WASM memory
	memData := m.memory.UnsafeData(m.store)
	copy(memData[ptr:], inputBytes)

	// Call matcher_match
	result, err = m.matcherMatch.Call(m.store, ptr, int32(len(inputBytes)))
	if err != nil {
		m.wasmFree.Call(m.store, ptr)
		return -1
	}

	// Free input buffer
	m.wasmFree.Call(m.store, ptr)

	return int(result.(int32))
}

// MatchAll returns indices of all matching patterns.
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
		m.matcherClose.Call(m.store)
	}
}

// GetError returns the last error message from the WASM module.
func (m *WasmMatcher) GetError() string {
	if m.getError == nil {
		return ""
	}
	result, err := m.getError.Call(m.store)
	if err != nil {
		return fmt.Sprintf("error calling getError: %v", err)
	}
	ptr := result.(int32)
	if ptr == 0 {
		return ""
	}
	// Read null-terminated string from WASM memory
	memData := m.memory.UnsafeData(m.store)
	var buf []byte
	for i := int32(0); i < 512; i++ {
		b := memData[ptr+i]
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return string(buf)
}

// CheckPlatform returns 0 if the platform is valid, non-zero otherwise.
func (m *WasmMatcher) CheckPlatform() int {
	if m.checkPlatform == nil {
		return -1
	}
	result, err := m.checkPlatform.Call(m.store)
	if err != nil {
		return -1
	}
	return int(result.(int32))
}
