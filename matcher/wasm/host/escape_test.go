package wasmvs

import (
	"fmt"
	"testing"
)

func TestPatternEscaping(t *testing.T) {
	// Test what patterns look like when passed to WASM
	testCases := []struct {
		name    string
		pattern string
		hex     string
	}{
		{"literal_d", `\d+`, ""},
		{"escaped_d", "\\d+", ""},
		{"double_escaped", "\\\\d+", ""},
		{"raw_bracket", `[a-z]+`, ""},
		{"pipe", `a|b`, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Print the raw bytes of the pattern
			t.Logf("Pattern: %q", tc.pattern)
			t.Logf("Bytes: % x", []byte(tc.pattern))
			t.Logf("Length: %d", len(tc.pattern))

			// Try to compile it
			patterns := []string{tc.pattern}
			m, err := NewWasmMatcher(patterns)
			if err != nil {
				t.Logf("FAILED to compile: %v", err)
			} else {
				t.Logf("SUCCESS - compiled")
				m.Close()
			}
		})
	}
}

func TestSimplePatternVariants(t *testing.T) {
	// Try different ways of expressing the same pattern
	variants := []string{
		"abc",           // simple literal - should work
		"a.c",           // dot any - might fail
		`a\.c`,          // escaped dot - should work
		"a\\.c",         // Go string escaped dot
	}

	for i, p := range variants {
		t.Run(fmt.Sprintf("variant_%d", i), func(t *testing.T) {
			t.Logf("Pattern string: %q", p)
			t.Logf("Pattern bytes: % x", []byte(p))

			m, err := NewWasmMatcher([]string{p})
			if err != nil {
				t.Logf("FAILED: %v", err)
			} else {
				t.Logf("SUCCESS")
				// Test matching
				testInputs := []string{"abc", "a.c", "aXc", "axc"}
				for _, input := range testInputs {
					result := m.Match(input)
					t.Logf("  Match(%q) = %d", input, result)
				}
				m.Close()
			}
		})
	}
}

func TestPlatformCheck(t *testing.T) {
	// Create a minimal matcher just to check platform
	m, err := NewWasmMatcher([]string{"abc"})
	if err != nil {
		t.Logf("Failed to create matcher: %v", err)
		return
	}
	defer m.Close()

	platform := m.CheckPlatform()
	t.Logf("Platform check: %d (0 = valid)", platform)

	errMsg := m.GetError()
	t.Logf("Error message: %q", errMsg)
}

func TestMorePatterns(t *testing.T) {
	// Test various pattern types
	tests := []struct {
		name    string
		pattern string
		input   string
		want    int // -1 = no match, 0 = match at pattern 0
	}{
		{"literal", "hello", "hello world", 0},
		{"literal_nomatch", "hello", "world", -1},
		{"case_insensitive", "(?i)HELLO", "hello world", 0},
		{"escaped_dot", `\.txt`, "file.txt", 0},
		{"star_quantifier", "ab*c", "ac", 0},
		{"star_quantifier2", "ab*c", "abbc", 0},
		{"plus_quantifier", "ab+c", "abc", 0},
		// These might fail - test to understand what works
		{"dot_any", "a.c", "abc", 0},
		{"char_class", "[a-z]+", "abc", 0},
		{"alternation", "foo|bar", "bar", 0},
		{"digit", `\d+`, "123", 0},
		{"anchor_start", "^foo", "foobar", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Pattern: %q (bytes: % x)", tt.pattern, []byte(tt.pattern))

			m, err := NewWasmMatcher([]string{tt.pattern})
			if err != nil {
				t.Logf("COMPILE FAILED: %v", err)
				return
			}
			defer m.Close()

			result := m.Match(tt.input)
			t.Logf("Match(%q) = %d, want %d", tt.input, result, tt.want)
			if result != tt.want {
				t.Errorf("Match result mismatch")
			}
		})
	}
}
