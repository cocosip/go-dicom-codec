//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run extract_from_openjpeg.go <t1_ht_generate_luts.c>")
		os.Exit(1)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Extract tbl0 and tbl1
	tbl0 := extractTable(lines, "tbl0", 72, 520)
	tbl1 := extractTable(lines, "tbl1", 520, 900)

	fmt.Printf("Extracted tbl0: %d entries\n", len(tbl0))
	fmt.Printf("Extracted tbl1: %d entries\n", len(tbl1))

	// Generate Go code
	generateGoCode(tbl0, tbl1)
}

func extractTable(lines []string, name string, start, end int) []string {
	var entries []string
	pattern := regexp.MustCompile(`\{.*?\}`)

	for i := start; i < end && i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "{") && strings.Contains(line, "}") {
			matches := pattern.FindAllString(line, -1)
			for _, match := range matches {
				// Clean up the entry
				match = strings.TrimSpace(match)
				if match != "" && match != "{" {
					entries = append(entries, match)
				}
			}
		}
	}

	return entries
}

func generateGoCode(tbl0, tbl1 []string) {
	out, err := os.Create("vlc_tables_openjpeg.go")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	fmt.Fprintln(out, "package t1ht")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "// VLC tables extracted from OpenJPEG")
	fmt.Fprintln(out, "// Source: https://github.com/uclouvain/openjpeg/blob/master/src/lib/openjp2/t1_ht_generate_luts.c")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "// VLCEntry represents one entry in the VLC table")
	fmt.Fprintln(out, "// Structure: {c_q, rho, u_off, e_k, e_1, cwd, cwd_len}")
	fmt.Fprintln(out, "type VLCEntry struct {")
	fmt.Fprintln(out, "\tCQ     uint8  // Context (c_q)")
	fmt.Fprintln(out, "\tRho    uint8  // Significance state")
	fmt.Fprintln(out, "\tUOff   uint8  // Unsigned offset")
	fmt.Fprintln(out, "\tEK     uint8  // E_k value")
	fmt.Fprintln(out, "\tE1     uint8  // E_1 value")
	fmt.Fprintln(out, "\tCwd    uint8  // Codeword")
	fmt.Fprintln(out, "\tCwdLen uint8  // Codeword length")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// Table 0
	fmt.Fprintln(out, "// VLC_tbl0 for initial quad rows")
	fmt.Fprintf(out, "var VLC_tbl0 = []VLCEntry{\n")
	for _, entry := range tbl0 {
		fmt.Fprintf(out, "\t%s,\n", entry)
	}
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	// Table 1
	fmt.Fprintln(out, "// VLC_tbl1 for non-initial quad rows")
	fmt.Fprintf(out, "var VLC_tbl1 = []VLCEntry{\n")
	for _, entry := range tbl1 {
		fmt.Fprintf(out, "\t%s,\n", entry)
	}
	fmt.Fprintln(out, "}")

	fmt.Println("Generated vlc_tables_openjpeg.go")
}
