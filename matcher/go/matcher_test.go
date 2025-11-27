package matcher

import (
	"fmt"
	"testing"
)

func TestGoMatcher_Match(t *testing.T) {
	patterns := []string{
		`^\d{3}-\d{4}$`,      // 0: phone format
		`^[a-z]+@[a-z]+\.\w+$`, // 1: simple email
		`error|fail|panic`,   // 2: error keywords
		`https?://`,          // 3: URL prefix
	}

	m, err := NewGoMatcher(patterns)
	if err != nil {
		t.Fatalf("NewGoMatcher failed: %v", err)
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
		{"ERROR in caps", -1}, // case sensitive
	}

	for _, tt := range tests {
		got := m.Match(tt.input)
		if got != tt.want {
			t.Errorf("Match(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestGoMatcher_MatchAll(t *testing.T) {
	patterns := []string{
		`error`,
		`fail`,
		`panic`,
	}

	m, err := NewGoMatcher(patterns)
	if err != nil {
		t.Fatalf("NewGoMatcher failed: %v", err)
	}
	defer m.Close()

	tests := []struct {
		input string
		want  []int
	}{
		{"all good", nil},
		{"error occurred", []int{0}},
		{"error and fail", []int{0, 1}},
		{"error fail panic", []int{0, 1, 2}},
	}

	for _, tt := range tests {
		got := m.MatchAll(tt.input)
		if !intSliceEqual(got, tt.want) {
			t.Errorf("MatchAll(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGoMatcher_InvalidPattern(t *testing.T) {
	patterns := []string{
		`valid`,
		`[invalid`, // unclosed bracket
	}

	_, err := NewGoMatcher(patterns)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Benchmark with varying pattern counts
func BenchmarkGoMatcher_Match_10(b *testing.B)   { benchmarkMatch(b, 10) }
func BenchmarkGoMatcher_Match_100(b *testing.B)  { benchmarkMatch(b, 100) }
func BenchmarkGoMatcher_Match_1000(b *testing.B) { benchmarkMatch(b, 1000) }

func benchmarkMatch(b *testing.B, patternCount int) {
	patterns := generatePatterns(patternCount)
	m, err := NewGoMatcher(patterns)
	if err != nil {
		b.Fatalf("NewGoMatcher failed: %v", err)
	}
	defer m.Close()

	// Input that matches the last pattern (worst case)
	input := fmt.Sprintf("prefix_pattern_%d_suffix", patternCount-1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

// Benchmark with varying input sizes
func BenchmarkGoMatcher_Match_ShortInput(b *testing.B)  { benchmarkInputSize(b, 100, 50) }
func BenchmarkGoMatcher_Match_MediumInput(b *testing.B) { benchmarkInputSize(b, 100, 500) }
func BenchmarkGoMatcher_Match_LongInput(b *testing.B)   { benchmarkInputSize(b, 100, 5000) }

func benchmarkInputSize(b *testing.B, patternCount, inputLen int) {
	patterns := generatePatterns(patternCount)
	m, err := NewGoMatcher(patterns)
	if err != nil {
		b.Fatalf("NewGoMatcher failed: %v", err)
	}
	defer m.Close()

	// Generate input of specified length with match near end
	input := generateInput(inputLen, patternCount-1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

// Benchmark MatchAll
func BenchmarkGoMatcher_MatchAll_100(b *testing.B) {
	patterns := generatePatterns(100)
	m, err := NewGoMatcher(patterns)
	if err != nil {
		b.Fatalf("NewGoMatcher failed: %v", err)
	}
	defer m.Close()

	// Input that matches multiple patterns
	input := "pattern_10 pattern_50 pattern_99"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MatchAll(input)
	}
}

// Benchmark no-match case
func BenchmarkGoMatcher_Match_NoMatch(b *testing.B) {
	patterns := generatePatterns(100)
	m, err := NewGoMatcher(patterns)
	if err != nil {
		b.Fatalf("NewGoMatcher failed: %v", err)
	}
	defer m.Close()

	input := "this string matches nothing in our pattern set"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(input)
	}
}

// generatePatterns creates n patterns of the form "pattern_N"
func generatePatterns(n int) []string {
	patterns := make([]string, n)
	for i := 0; i < n; i++ {
		patterns[i] = fmt.Sprintf(`pattern_%d`, i)
	}
	return patterns
}

// generateInput creates a string of approximately len characters
// that contains the specified pattern
func generateInput(length, patternIdx int) string {
	pattern := fmt.Sprintf("pattern_%d", patternIdx)
	if length <= len(pattern) {
		return pattern
	}

	// Pad with non-matching content, put pattern near end
	padding := make([]byte, length-len(pattern)-1)
	for i := range padding {
		padding[i] = 'x'
	}
	return string(padding) + " " + pattern
}
