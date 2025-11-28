// Command matcher provides a CLI for testing WASM Vectorscan patterns
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	wasmvs "github.com/paulstuart/cgo-ffi/matcher/wasm/host"
)

func main() {
	var (
		patterns = flag.String("p", "", "Comma-separated patterns to compile")
		input    = flag.String("i", "", "Input string to match")
		file     = flag.String("f", "", "File containing patterns (one per line)")
		verbose  = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	if *patterns == "" && *file == "" {
		fmt.Fprintln(os.Stderr, "Usage: matcher -p 'pattern1,pattern2' -i 'input string'")
		fmt.Fprintln(os.Stderr, "       matcher -f patterns.txt -i 'input string'")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var patternList []string
	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				patternList = append(patternList, line)
			}
		}
	} else {
		patternList = strings.Split(*patterns, ",")
	}

	if len(patternList) == 0 {
		fmt.Fprintln(os.Stderr, "No patterns provided")
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Compiling %d pattern(s):\n", len(patternList))
		for i, p := range patternList {
			fmt.Printf("  [%d] %q\n", i, p)
		}
	}

	m, err := wasmvs.NewWasmMatcher(patternList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to compile patterns: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	if *verbose {
		fmt.Printf("Successfully compiled %d pattern(s)\n", m.PatternCount())
		fmt.Printf("Platform check: %d\n", m.CheckPlatform())
	}

	if *input == "" {
		fmt.Println("Patterns compiled successfully")
		return
	}

	result := m.Match(*input)
	if result >= 0 {
		fmt.Printf("Match: pattern[%d] = %q\n", result, patternList[result])
	} else {
		fmt.Println("No match")
	}
}
