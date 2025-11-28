package wasmvs

import (
	"fmt"
	"testing"

	"github.com/paulstuart/cgo-ffi/matcher/testdata"
)

func TestWasmMatcher_Simple(t *testing.T) {
	// Very simple patterns to test basic functionality
	patterns := []string{
		`hello`,
		`world`,
	}

	m, err := NewWasmMatcher(patterns)
	if err != nil {
		t.Fatalf("NewWasmMatcher failed: %v", err)
	}
	defer m.Close()

	tests := []struct {
		input string
		want  int
	}{
		{"hello there", 0},
		{"world peace", 1},
		{"no match", -1},
	}

	for _, tt := range tests {
		got := m.Match(tt.input)
		if got != tt.want {
			t.Errorf("Match(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestWasmMatcher_Match(t *testing.T) {
	// Use simpler patterns that work with SIMDe backend
	patterns := []string{
		`error`,   // 0: literal
		`fail`,    // 1: literal
		`panic`,   // 2: literal
		`warning`, // 3: literal
	}

	m, err := NewWasmMatcher(patterns)
	if err != nil {
		t.Fatalf("NewWasmMatcher failed: %v", err)
	}
	defer m.Close()

	tests := []struct {
		input string
		want  int
	}{
		{"error occurred", 0},
		{"test failed", 1},
		{"kernel panic", 2},
		{"warning message", 3},
		{"no match here", -1},
	}

	for _, tt := range tests {
		got := m.Match(tt.input)
		if got != tt.want {
			t.Errorf("Match(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestWasmMatcher_MalwarePatterns(t *testing.T) {
	// WASM Vectorscan only supports simple literal patterns.
	// Use SimpleMalwarePatterns which contains only literals.
	m, err := NewWasmMatcher(testdata.SimpleMalwarePatterns)
	if err != nil {
		t.Fatalf("NewWasmMatcher failed with malware patterns: %v", err)
	}
	defer m.Close()

	t.Logf("Compiled %d simple malware patterns", m.PatternCount())

	// Test some known malicious files (Unix paths)
	maliciousTests := []struct {
		file      string
		shouldHit bool
	}{
		{`/tmp/downloads/mimikatz.bin`, true},
		{`/tmp/ransomware_kit.tar.gz`, true},
		{`/home/user/.cache/cobalt strike`, true},
		{`/usr/bin/ls`, false},
		{`/usr/bin/bash`, false},
	}

	for _, tt := range maliciousTests {
		result := m.Match(tt.file)
		gotHit := result >= 0
		if gotHit != tt.shouldHit {
			t.Errorf("Match(%q) = %d, shouldHit=%v but got hit=%v", tt.file, result, tt.shouldHit, gotHit)
		}
	}
}

func BenchmarkWasmMatcher_Match_10(b *testing.B) { benchmarkWasmMatch(b, 10) }
func BenchmarkWasmMatcher_Match_50(b *testing.B) { benchmarkWasmMatch(b, 50) }

func benchmarkWasmMatch(b *testing.B, patternCount int) {
	// Generate simple literal patterns
	patterns := make([]string, patternCount)
	for i := 0; i < patternCount; i++ {
		patterns[i] = fmt.Sprintf("pattern%d", i)
	}

	m, err := NewWasmMatcher(patterns)
	if err != nil {
		b.Fatalf("NewWasmMatcher failed: %v", err)
	}
	defer m.Close()

	input := `/usr/bin/notepad`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

func BenchmarkWasmMatcher_ScanAllFiles(b *testing.B) {
	// Use SimpleMalwarePatterns which work with WASM backend
	m, err := NewWasmMatcher(testdata.SimpleMalwarePatterns)
	if err != nil {
		b.Fatalf("NewWasmMatcher failed: %v", err)
	}
	defer m.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range testdata.TestFilenames {
			m.Match(f)
		}
	}
}
