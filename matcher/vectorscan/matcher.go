// Package vectorscan provides a Vectorscan-based implementation of multi-pattern regex matching.
//
// Vectorscan (https://github.com/VectorCamp/vectorscan) is the actively maintained fork
// of Intel's Hyperscan. It matches ALL patterns simultaneously using hybrid automata,
// giving O(1) matching regardless of pattern count, compared to Go's sequential O(n) approach.
//
// Requires libhs (Vectorscan) to be installed:
//   - macOS: brew install vectorscan
//   - Ubuntu: Build from source (see https://github.com/VectorCamp/vectorscan)
//   - From source: https://github.com/VectorCamp/vectorscan
//
// The Go bindings (github.com/flier/gohs) are API-compatible with both Hyperscan and Vectorscan.
package vectorscan

import (
	"fmt"
	"sync"

	hs "github.com/flier/gohs/hyperscan"
)

// VsMatcher implements multi-pattern matching using Vectorscan.
// It compiles all patterns into a single database and matches them simultaneously.
type VsMatcher struct {
	db       hs.BlockDatabase
	scratch  *hs.Scratch
	patterns []string
	mu       sync.Mutex
}

// NewVsMatcher creates a new Vectorscan-based matcher from the given patterns.
// Patterns are compiled into a block-mode database for simultaneous matching.
func NewVsMatcher(patterns []string) (*VsMatcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}

	// Convert to Vectorscan patterns with IDs
	vsPatterns := make([]*hs.Pattern, len(patterns))
	for i, p := range patterns {
		vsPatterns[i] = &hs.Pattern{
			Expression: p,
			Flags:      hs.Caseless | hs.SingleMatch | hs.Utf8Mode,
			Id:         i,
		}
	}

	// Compile all patterns into a single database
	db, err := hs.NewBlockDatabase(vsPatterns...)
	if err != nil {
		return nil, fmt.Errorf("failed to compile patterns: %w", err)
	}

	// Allocate scratch space for scanning
	scratch, err := hs.NewScratch(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to allocate scratch: %w", err)
	}

	return &VsMatcher{
		db:       db,
		scratch:  scratch,
		patterns: patterns,
	}, nil
}

// Match returns the index of the first matching pattern, or -1 if no match.
// All patterns are checked simultaneously - this is O(1) regardless of pattern count.
func (m *VsMatcher) Match(input string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	matchedID := -1

	// Scan with a handler that captures the first match
	handler := hs.MatchHandler(func(id uint, from, to uint64, flags uint, context interface{}) error {
		matchedID = int(id)
		// Return error to stop scanning after first match
		return hs.ErrScanTerminated
	})

	// Scan the input - ignoring ErrScanTerminated as it just means we found a match
	err := m.db.Scan([]byte(input), m.scratch, handler, nil)
	if err != nil && err != hs.ErrScanTerminated {
		return -1
	}

	return matchedID
}

// MatchAll returns indices of all matching patterns.
// All patterns are checked simultaneously.
func (m *VsMatcher) MatchAll(input string) []int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var matches []int
	seen := make(map[int]bool)

	handler := hs.MatchHandler(func(id uint, from, to uint64, flags uint, context interface{}) error {
		if !seen[int(id)] {
			matches = append(matches, int(id))
			seen[int(id)] = true
		}
		return nil // Continue scanning
	})

	m.db.Scan([]byte(input), m.scratch, handler, nil)
	return matches
}

// PatternCount returns the number of patterns.
func (m *VsMatcher) PatternCount() int {
	return len(m.patterns)
}

// Close releases Vectorscan resources.
func (m *VsMatcher) Close() {
	if m.scratch != nil {
		m.scratch.Free()
	}
	if m.db != nil {
		m.db.Close()
	}
}

// DatabaseInfo returns information about the compiled database.
func (m *VsMatcher) DatabaseInfo() (string, error) {
	info, err := m.db.Info()
	if err != nil {
		return "", err
	}
	return info.String(), nil
}

// DatabaseSize returns the size of the compiled database in bytes.
func (m *VsMatcher) DatabaseSize() (int, error) {
	return m.db.Size()
}
