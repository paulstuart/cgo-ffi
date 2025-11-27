// Command matcher-demo benchmarks multi-pattern regex matching comparing:
// - Pure Go (sequential regexp matching)
// - Vectorscan (simultaneous pattern matching via native library)
// - WASM Vectorscan (future - when available)
//
// Usage:
//
//	go run ./cmd
package main

import (
	"fmt"
	"math/rand"
	"slices"
	"time"

	gomatcher "github.com/paulstuart/cgo-ffi/matcher/go"
	"github.com/paulstuart/cgo-ffi/matcher/testdata"
	"github.com/paulstuart/cgo-ffi/matcher/vectorscan"
)

// Matcher interface for comparing implementations
type Matcher interface {
	Match(input string) int
	MatchAll(input string) []int
	PatternCount() int
	Close()
}

// stats holds timing statistics for a benchmark
type stats struct {
	min   time.Duration
	avg   time.Duration
	p95   time.Duration
	max   time.Duration
	total time.Duration
	n     int
}

// benchmark runs a function n times and collects timing statistics
func benchmark(n int, fn func()) stats {
	times := make([]time.Duration, n)

	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		times[i] = time.Since(start)
	}

	slices.Sort(times)

	var total time.Duration
	for _, t := range times {
		total += t
	}

	p95idx := int(float64(n) * 0.95)
	if p95idx >= n {
		p95idx = n - 1
	}

	return stats{
		min:   times[0],
		avg:   total / time.Duration(n),
		p95:   times[p95idx],
		max:   times[n-1],
		total: total,
		n:     n,
	}
}

func (s stats) String() string {
	return fmt.Sprintf("avg=%-8s min=%-8s p95=%-8s max=%-8s",
		formatDuration(s.avg),
		formatDuration(s.min),
		formatDuration(s.p95),
		formatDuration(s.max))
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	}
	return d.Round(time.Microsecond).String()
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Multi-Pattern Regex Matcher Comparison: Go vs Vectorscan                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Test data: %d malware patterns, %d test filenames (%d malicious)\n",
		len(testdata.MalwarePatterns),
		len(testdata.TestFilenames),
		len(testdata.MaliciousIndices))
	fmt.Println()

	// Pattern counts to test
	patternCounts := []int{8, 64, 128, 256}

	for _, count := range patternCounts {
		runComparison(count)
	}

	// Full throughput comparison
	runThroughputComparison()
}

func runComparison(patternCount int) {
	fmt.Printf("┌──────────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│ Pattern Count: %-4d                                                         │\n", patternCount)
	fmt.Printf("└──────────────────────────────────────────────────────────────────────────────┘\n")

	patterns := testdata.MalwarePatterns[:patternCount]
	benignFiles := testdata.BenignFilenames()
	iterations := 1000

	// === Pure Go Matcher ===
	fmt.Println("\n  ┌─ Pure Go (sequential matching) ─────────────────────────────────────────┐")

	goStart := time.Now()
	goMatcher, err := gomatcher.NewGoMatcher(patterns)
	if err != nil {
		fmt.Printf("  │ ERROR: %v\n", err)
	} else {
		defer goMatcher.Close()
		fmt.Printf("  │ Compile time: %v\n", time.Since(goStart))

		firstHit, middleHit, lastHit := findHitPositions(goMatcher, patternCount)

		if firstHit != "" {
			s := benchmark(iterations, func() { goMatcher.Match(firstHit) })
			fmt.Printf("  │ First hit:    %s\n", s)
		}
		if middleHit != "" {
			s := benchmark(iterations, func() { goMatcher.Match(middleHit) })
			fmt.Printf("  │ Middle hit:   %s\n", s)
		}
		if lastHit != "" {
			s := benchmark(iterations, func() { goMatcher.Match(lastHit) })
			fmt.Printf("  │ Last hit:     %s\n", s)
		}
		if len(benignFiles) > 0 {
			benign := benignFiles[rand.Intn(len(benignFiles))]
			s := benchmark(iterations, func() { goMatcher.Match(benign) })
			fmt.Printf("  │ No match:     %s\n", s)
		}

		// Batch scan
		var goMatches int
		batchStart := time.Now()
		for _, f := range testdata.TestFilenames {
			if goMatcher.Match(f) >= 0 {
				goMatches++
			}
		}
		batchTime := time.Since(batchStart)
		fmt.Printf("  │ Scan all:     %v (%d files, %d matches, %.0f files/sec)\n",
			batchTime, len(testdata.TestFilenames), goMatches,
			float64(len(testdata.TestFilenames))/batchTime.Seconds())
	}
	fmt.Println("  └────────────────────────────────────────────────────────────────────────────┘")

	// === Vectorscan Matcher ===
	fmt.Println("\n  ┌─ Vectorscan (simultaneous matching) ────────────────────────────────────┐")

	vsStart := time.Now()
	vsMatcher, err := vectorscan.NewVsMatcher(patterns)
	if err != nil {
		fmt.Printf("  │ ERROR: %v\n", err)
	} else {
		defer vsMatcher.Close()
		compileTime := time.Since(vsStart)
		dbSize, _ := vsMatcher.DatabaseSize()
		fmt.Printf("  │ Compile time: %v (database: %.1f KB)\n", compileTime, float64(dbSize)/1024)

		firstHit, middleHit, lastHit := findVsHitPositions(vsMatcher, patternCount)

		if firstHit != "" {
			s := benchmark(iterations, func() { vsMatcher.Match(firstHit) })
			fmt.Printf("  │ First hit:    %s\n", s)
		}
		if middleHit != "" {
			s := benchmark(iterations, func() { vsMatcher.Match(middleHit) })
			fmt.Printf("  │ Middle hit:   %s\n", s)
		}
		if lastHit != "" {
			s := benchmark(iterations, func() { vsMatcher.Match(lastHit) })
			fmt.Printf("  │ Last hit:     %s\n", s)
		}
		if len(benignFiles) > 0 {
			benign := benignFiles[rand.Intn(len(benignFiles))]
			s := benchmark(iterations, func() { vsMatcher.Match(benign) })
			fmt.Printf("  │ No match:     %s\n", s)
		}

		// Batch scan
		var vsMatches int
		batchStart := time.Now()
		for _, f := range testdata.TestFilenames {
			if vsMatcher.Match(f) >= 0 {
				vsMatches++
			}
		}
		batchTime := time.Since(batchStart)
		fmt.Printf("  │ Scan all:     %v (%d files, %d matches, %.0f files/sec)\n",
			batchTime, len(testdata.TestFilenames), vsMatches,
			float64(len(testdata.TestFilenames))/batchTime.Seconds())
	}
	fmt.Println("  └────────────────────────────────────────────────────────────────────────────┘")

	fmt.Println()
}

func runThroughputComparison() {
	fmt.Printf("╔══════════════════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  THROUGHPUT COMPARISON (256 patterns, 10 full scans)                         ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════════════════╝\n\n")

	patterns := testdata.MalwarePatterns
	numScans := 10

	// Go Matcher
	goMatcher, err := gomatcher.NewGoMatcher(patterns)
	if err != nil {
		fmt.Printf("Go ERROR: %v\n", err)
		return
	}
	defer goMatcher.Close()

	// Vectorscan Matcher
	vsMatcher, err := vectorscan.NewVsMatcher(patterns)
	if err != nil {
		fmt.Printf("Vectorscan ERROR: %v\n", err)
		return
	}
	defer vsMatcher.Close()

	// Warm up
	for _, f := range testdata.TestFilenames[:100] {
		goMatcher.Match(f)
		vsMatcher.Match(f)
	}

	// Benchmark Go
	var goTimes []time.Duration
	var goTotalMatches int
	for i := 0; i < numScans; i++ {
		matches := 0
		start := time.Now()
		for _, f := range testdata.TestFilenames {
			if goMatcher.Match(f) >= 0 {
				matches++
			}
		}
		goTimes = append(goTimes, time.Since(start))
		goTotalMatches += matches
	}

	// Benchmark Vectorscan
	var vsTimes []time.Duration
	var vsTotalMatches int
	for i := 0; i < numScans; i++ {
		matches := 0
		start := time.Now()
		for _, f := range testdata.TestFilenames {
			if vsMatcher.Match(f) >= 0 {
				matches++
			}
		}
		vsTimes = append(vsTimes, time.Since(start))
		vsTotalMatches += matches
	}

	// Calculate stats
	slices.Sort(goTimes)
	slices.Sort(vsTimes)

	var goTotal, vsTotal time.Duration
	for i := 0; i < numScans; i++ {
		goTotal += goTimes[i]
		vsTotal += vsTimes[i]
	}
	goAvg := goTotal / time.Duration(numScans)
	vsAvg := vsTotal / time.Duration(numScans)

	goFilesPerSec := float64(len(testdata.TestFilenames)) / goAvg.Seconds()
	vsFilesPerSec := float64(len(testdata.TestFilenames)) / vsAvg.Seconds()
	speedup := goAvg.Seconds() / vsAvg.Seconds()

	fmt.Printf("  Files per scan:    %d\n", len(testdata.TestFilenames))
	fmt.Printf("  Patterns:          %d\n", len(patterns))
	fmt.Printf("  Scans:             %d\n\n", numScans)

	fmt.Println("  ┌─────────────────┬──────────────┬──────────────┬──────────────┬────────────┐")
	fmt.Println("  │ Implementation  │ Avg Time     │ Min Time     │ Max Time     │ Files/sec  │")
	fmt.Println("  ├─────────────────┼──────────────┼──────────────┼──────────────┼────────────┤")
	fmt.Printf("  │ Pure Go         │ %12v │ %12v │ %12v │ %10.0f │\n",
		goAvg.Round(time.Microsecond), goTimes[0].Round(time.Microsecond), goTimes[numScans-1].Round(time.Microsecond), goFilesPerSec)
	fmt.Printf("  │ Vectorscan      │ %12v │ %12v │ %12v │ %10.0f │\n",
		vsAvg.Round(time.Microsecond), vsTimes[0].Round(time.Microsecond), vsTimes[numScans-1].Round(time.Microsecond), vsFilesPerSec)
	fmt.Println("  └─────────────────┴──────────────┴──────────────┴──────────────┴────────────┘")

	fmt.Printf("\n  Vectorscan speedup: %.1fx faster\n", speedup)
	fmt.Printf("  Matches per scan: Go=%d, Vectorscan=%d\n",
		goTotalMatches/numScans, vsTotalMatches/numScans)
	fmt.Println()
}

// findHitPositions finds test files that match patterns at different positions (Go matcher)
func findHitPositions(m *gomatcher.GoMatcher, patternCount int) (first, middle, last string) {
	type match struct {
		file       string
		patternIdx int
	}
	var matches []match

	for _, idx := range testdata.MaliciousIndices {
		if idx >= len(testdata.TestFilenames) {
			continue
		}
		f := testdata.TestFilenames[idx]
		matchIdx := m.Match(f)
		if matchIdx >= 0 && matchIdx < patternCount {
			matches = append(matches, match{f, matchIdx})
		}
	}

	if len(matches) == 0 {
		return "", "", ""
	}

	slices.SortFunc(matches, func(a, b match) int {
		return a.patternIdx - b.patternIdx
	})

	first = matches[0].file
	if len(matches) > 1 {
		last = matches[len(matches)-1].file
	}
	if len(matches) > 2 {
		middle = matches[len(matches)/2].file
	}

	return first, middle, last
}

// findVsHitPositions finds test files that match patterns at different positions (Vectorscan matcher)
func findVsHitPositions(m *vectorscan.VsMatcher, patternCount int) (first, middle, last string) {
	type match struct {
		file       string
		patternIdx int
	}
	var matches []match

	for _, idx := range testdata.MaliciousIndices {
		if idx >= len(testdata.TestFilenames) {
			continue
		}
		f := testdata.TestFilenames[idx]
		matchIdx := m.Match(f)
		if matchIdx >= 0 && matchIdx < patternCount {
			matches = append(matches, match{f, matchIdx})
		}
	}

	if len(matches) == 0 {
		return "", "", ""
	}

	slices.SortFunc(matches, func(a, b match) int {
		return a.patternIdx - b.patternIdx
	})

	first = matches[0].file
	if len(matches) > 1 {
		last = matches[len(matches)-1].file
	}
	if len(matches) > 2 {
		middle = matches[len(matches)/2].file
	}

	return first, middle, last
}
