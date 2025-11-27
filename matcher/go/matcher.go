// Package matcher provides a pure Go implementation of multi-pattern regex matching.
//
// This serves as the baseline for comparison against WASM implementations.
// It uses Go's regexp package which is based on RE2 (guaranteed linear time).
package matcher

import (
	"fmt"
	"regexp"
)

// Matcher interface for multi-pattern regex matching.
type Matcher interface {
	// Match returns the index of the first matching pattern, or -1 if no match.
	Match(input string) int

	// MatchAll returns indices of all matching patterns.
	MatchAll(input string) []int

	// PatternCount returns the number of patterns.
	PatternCount() int

	// Close releases resources.
	Close()
}

// GoMatcher implements Matcher using Go's regexp package.
// Patterns are matched sequentially in order.
type GoMatcher struct {
	patterns []*regexp.Regexp
}

// NewGoMatcher creates a new GoMatcher from the given pattern strings.
// Returns an error if any pattern fails to compile.
func NewGoMatcher(patterns []string) (*GoMatcher, error) {
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("pattern %d (%q): %w", i, p, err)
		}
		compiled[i] = re
	}
	return &GoMatcher{patterns: compiled}, nil
}

// Match returns the index of the first matching pattern, or -1 if no match.
// Patterns are tested in order; returns on first match.
func (m *GoMatcher) Match(input string) int {
	for i, re := range m.patterns {
		if re.MatchString(input) {
			return i
		}
	}
	return -1
}

// MatchAll returns indices of all matching patterns.
func (m *GoMatcher) MatchAll(input string) []int {
	var matches []int
	for i, re := range m.patterns {
		if re.MatchString(input) {
			matches = append(matches, i)
		}
	}
	return matches
}

// PatternCount returns the number of patterns.
func (m *GoMatcher) PatternCount() int {
	return len(m.patterns)
}

// Close releases resources. For GoMatcher this is a no-op.
func (m *GoMatcher) Close() {}
