//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// VLCEntry represents one VLC table entry
type VLCEntry struct {
	Table uint8
	Ctx   uint8
	Rho   uint8
	UOff  uint8
	EK    uint8
	Code  uint8
	Len   uint8
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cross_verify_spec.go <spec_file.txt>")
		os.Exit(1)
	}

	// Parse spec file
	specTable0, specTable1 := parseSpecFile(os.Args[1])

	fmt.Printf("=== Cross-Verification Report ===\n\n")
	fmt.Printf("Spec file: %s\n", os.Args[1])
	fmt.Printf("Extracted from spec:\n")
	fmt.Printf("  CxtVLC_table_0: %d entries\n", len(specTable0))
	fmt.Printf("  CxtVLC_table_1: %d entries\n", len(specTable1))
	fmt.Printf("\n")

	// Load generated tables (we'll need to import them)
	// For now, print what we found in the spec

	fmt.Println("=== Table 0 Verification ===")
	verifyTable("CxtVLC_table_0", specTable0, 1795)

	fmt.Println("\n=== Table 1 Verification ===")
	verifyTable("CxtVLC_table_1", specTable1, 2110)
}

func parseSpecFile(filename string) ([]VLCEntry, []VLCEntry) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		lines = append(lines, scanner.Text())
	}

	table0 := extractTableWithLineNums(lines, "CxtVLC_table_0", 1795)
	table1 := extractTableWithLineNums(lines, "CxtVLC_table_1", 2110)

	return table0, table1
}

func extractTableWithLineNums(lines []string, tableName string, expectedLine int) []VLCEntry {
	var entries []VLCEntry
	inTable := false
	startLine := 0

	// Three patterns to match entries
	pattern1 := regexp.MustCompile(`\{(\d+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*(\d+)\}`)
	pattern2 := regexp.MustCompile(`\((\d+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*(\d+)\}`)
	pattern3 := regexp.MustCompile(`=(\d+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*0x([0-9A-Fa-f]+),\s*(\d+)\}`)

	for i, line := range lines {
		if strings.Contains(line, tableName) && strings.Contains(line, "=") &&
			(strings.Contains(line, "0x") || strings.Contains(line, "{")) {
			inTable = true
			startLine = i + 1
			fmt.Printf("Found %s at line %d (expected %d)\n", tableName, startLine, expectedLine)

			// Extract entries from this line
			entries = append(entries, extractFromLine(line, pattern1, pattern2, pattern3)...)
			continue
		}

		if inTable && (strings.Contains(line, "CxtVLC_table") ||
			strings.Contains(line, "Annex") ||
			(strings.Contains(line, "Rec.ITU-T") && len(entries) > 100)) {
			if !strings.Contains(line, tableName) {
				break
			}
		}

		if inTable {
			entries = append(entries, extractFromLine(line, pattern1, pattern2, pattern3)...)
		}
	}

	return entries
}

func extractFromLine(line string, patterns ...*regexp.Regexp) []VLCEntry {
	var entries []VLCEntry

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if entry := parseEntry(match); entry != nil {
				entries = append(entries, *entry)
			}
		}
	}

	return entries
}

func parseEntry(match []string) *VLCEntry {
	if len(match) != 8 {
		return nil
	}

	table, _ := strconv.ParseUint(match[1], 10, 8)
	ctx, _ := strconv.ParseUint(match[2], 16, 8)
	rho, _ := strconv.ParseUint(match[3], 16, 8)
	uoff, _ := strconv.ParseUint(match[4], 16, 8)
	ek, _ := strconv.ParseUint(match[5], 16, 8)
	code, _ := strconv.ParseUint(match[6], 16, 8)
	len_, _ := strconv.ParseUint(match[7], 10, 8)

	return &VLCEntry{
		Table: uint8(table),
		Ctx:   uint8(ctx),
		Rho:   uint8(rho),
		UOff:  uint8(uoff),
		EK:    uint8(ek),
		Code:  uint8(code),
		Len:   uint8(len_),
	}
}

func verifyTable(name string, entries []VLCEntry, startLine int) {
	fmt.Printf("\nTable: %s (starting at line %d)\n", name, startLine)
	fmt.Printf("Total entries: %d\n\n", len(entries))

	// Print first 5 and last 5 for verification
	fmt.Println("First 5 entries:")
	for i := 0; i < 5 && i < len(entries); i++ {
		e := entries[i]
		fmt.Printf("  [%3d] {%d, 0x%02X, 0x%X, 0x%X, 0x%X, 0x%02X, %d}\n",
			i, e.Table, e.Ctx, e.Rho, e.UOff, e.EK, e.Code, e.Len)
	}

	if len(entries) > 10 {
		fmt.Println("\n...")
		fmt.Println("\nLast 5 entries:")
		for i := len(entries) - 5; i < len(entries); i++ {
			e := entries[i]
			fmt.Printf("  [%3d] {%d, 0x%02X, 0x%X, 0x%X, 0x%X, 0x%02X, %d}\n",
				i, e.Table, e.Ctx, e.Rho, e.UOff, e.EK, e.Code, e.Len)
		}
	}

	// Check for potential OCR errors in hex values
	fmt.Println("\n=== Checking for suspicious patterns ===")
	checkSuspiciousValues(entries)
}

func checkSuspiciousValues(entries []VLCEntry) {
	suspiciousCount := 0

	// Check for patterns that might indicate OCR errors
	for i, e := range entries {
		suspicious := false
		reasons := []string{}

		// Context should typically be 0x00-0x0F
		if e.Ctx > 0x0F {
			suspicious = true
			reasons = append(reasons, fmt.Sprintf("ctx=0x%02X > 0x0F", e.Ctx))
		}

		// Rho should be 0 or 1 in most cases, but can be higher
		if e.Rho > 0xF {
			suspicious = true
			reasons = append(reasons, fmt.Sprintf("rho=0x%X > 0xF", e.Rho))
		}

		// Code should fit in Len bits
		maxCode := uint8((1 << e.Len) - 1)
		if e.Code > maxCode {
			suspicious = true
			reasons = append(reasons, fmt.Sprintf("code=0x%02X exceeds len=%d (max=0x%02X)", e.Code, e.Len, maxCode))
		}

		// Length should be reasonable (1-8 for uint8 code)
		if e.Len == 0 || e.Len > 8 {
			suspicious = true
			reasons = append(reasons, fmt.Sprintf("len=%d out of range", e.Len))
		}

		if suspicious {
			suspiciousCount++
			fmt.Printf("  Entry [%3d]: {%d, 0x%02X, 0x%X, 0x%X, 0x%X, 0x%02X, %d}\n",
				i, e.Table, e.Ctx, e.Rho, e.UOff, e.EK, e.Code, e.Len)
			for _, reason := range reasons {
				fmt.Printf("    ⚠ %s\n", reason)
			}
		}
	}

	if suspiciousCount == 0 {
		fmt.Println("  ✓ No suspicious patterns detected")
	} else {
		fmt.Printf("  ⚠ Found %d entries with suspicious values\n", suspiciousCount)
	}
}
