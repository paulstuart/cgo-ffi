package vectorscan

import (
	"fmt"
	"testing"

	"github.com/paulstuart/cgo-ffi/matcher/testdata"
)

func TestVsMatcher_Match(t *testing.T) {
	patterns := []string{
		`\d{3}-\d{4}`,         // 0: phone format
		`[a-z]+@[a-z]+\.\w+`,  // 1: simple email
		`error|fail|panic`,    // 2: error keywords
		`https?://`,           // 3: URL prefix
	}

	m, err := NewVsMatcher(patterns)
	if err != nil {
		t.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	tests := []struct {
		input string
		want  int
	}{
		{"123-4567", 0},
		{"test@example.com", 1},
		{"something failed here", 2},
		{"visit https://example.com", 3},
		{"no match here", -1},
	}

	for _, tt := range tests {
		got := m.Match(tt.input)
		if got != tt.want {
			t.Errorf("Match(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestVsMatcher_MatchAll(t *testing.T) {
	patterns := []string{
		`error`,
		`fail`,
		`panic`,
	}

	m, err := NewVsMatcher(patterns)
	if err != nil {
		t.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	tests := []struct {
		input    string
		wantLen  int
	}{
		{"all good", 0},
		{"error occurred", 1},
		{"error and fail", 2},
		{"error fail panic", 3},
	}

	for _, tt := range tests {
		got := m.MatchAll(tt.input)
		if len(got) != tt.wantLen {
			t.Errorf("MatchAll(%q) returned %d matches, want %d", tt.input, len(got), tt.wantLen)
		}
	}
}

func TestVsMatcher_DatabaseInfo(t *testing.T) {
	patterns := []string{`test`, `pattern`}
	m, err := NewVsMatcher(patterns)
	if err != nil {
		t.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	info, err := m.DatabaseInfo()
	if err != nil {
		t.Errorf("DatabaseInfo failed: %v", err)
	}
	t.Logf("Database info: %s", info)

	size, err := m.DatabaseSize()
	if err != nil {
		t.Errorf("DatabaseSize failed: %v", err)
	}
	t.Logf("Database size: %d bytes", size)
}

func TestVsMatcher_MalwarePatterns(t *testing.T) {
	// Test with real malware patterns
	m, err := NewVsMatcher(testdata.MalwarePatterns)
	if err != nil {
		t.Fatalf("NewVsMatcher failed with malware patterns: %v", err)
	}
	defer m.Close()

	t.Logf("Compiled %d malware patterns", m.PatternCount())

	size, _ := m.DatabaseSize()
	t.Logf("Database size: %d bytes (%.2f KB)", size, float64(size)/1024)

	// Test some known malicious files
	maliciousTests := []struct {
		file      string
		shouldHit bool
	}{
		{`C:\Users\Public\Downloads\mimikatz.exe`, true},
		{`C:\Users\jsmith\Downloads\invoice_march.pdf.exe`, true},
		{`C:\Windows\System32\notepad.exe`, false},
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

// Benchmarks with varying pattern counts
func BenchmarkVsMatcher_Match_10(b *testing.B)   { benchmarkVsMatch(b, 10) }
func BenchmarkVsMatcher_Match_100(b *testing.B)  { benchmarkVsMatch(b, 100) }
func BenchmarkVsMatcher_Match_256(b *testing.B)  { benchmarkVsMatch(b, 256) }

func benchmarkVsMatch(b *testing.B, patternCount int) {
	if patternCount > len(testdata.MalwarePatterns) {
		patternCount = len(testdata.MalwarePatterns)
	}
	patterns := testdata.MalwarePatterns[:patternCount]

	m, err := NewVsMatcher(patterns)
	if err != nil {
		b.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	// Use a benign file that won't match (worst case for sequential, same for Vectorscan)
	input := `C:\Windows\System32\notepad.exe`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

// Benchmark with match at different positions
func BenchmarkVsMatcher_Match_FirstPattern(b *testing.B) {
	m, err := NewVsMatcher(testdata.MalwarePatterns)
	if err != nil {
		b.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	// File that matches early pattern (emotet - pattern 0)
	input := `C:\Users\marketing\AppData\Local\Temp\emotet_12345.exe`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

func BenchmarkVsMatcher_Match_LastPattern(b *testing.B) {
	m, err := NewVsMatcher(testdata.MalwarePatterns)
	if err != nil {
		b.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	// File that matches late pattern (crack/keygen - pattern ~246)
	input := `C:\Downloads\crack_photoshop_2024.exe`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

func BenchmarkVsMatcher_Match_NoMatch(b *testing.B) {
	m, err := NewVsMatcher(testdata.MalwarePatterns)
	if err != nil {
		b.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	// Benign file - no match
	input := `C:\Program Files\Microsoft Office\root\Office16\WINWORD.EXE`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

// Benchmark scanning all test files
func BenchmarkVsMatcher_ScanAllFiles(b *testing.B) {
	m, err := NewVsMatcher(testdata.MalwarePatterns)
	if err != nil {
		b.Fatalf("NewVsMatcher failed: %v", err)
	}
	defer m.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range testdata.TestFilenames {
			m.Match(f)
		}
	}
}

// Helper to generate simple patterns
func generatePatterns(n int) []string {
	patterns := make([]string, n)
	for i := 0; i < n; i++ {
		patterns[i] = fmt.Sprintf(`pattern_%d`, i)
	}
	return patterns
}
