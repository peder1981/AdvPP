package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/advpl/compiler/pkg/rpo"
)

const maxStringAnalysis = 500000 // Limit bytes for string analysis

func cmdRpoAnalyze(args []string) error {
	// Criar flag set para análise
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed analysis")
	
	// Parse flags
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	// Primeiro arg não-flag deve ser o arquivo
	if fs.NArg() == 0 {
		return fmt.Errorf("uso: advplc rpo analyze [--verbose] <arquivo.rpo>")
	}
	
	rpoPath := fs.Arg(0)
	
	data, err := os.ReadFile(rpoPath)
	if err != nil {
		return fmt.Errorf("failed to read RPO file: %w", err)
	}
	
	// Parse RPO
	info, err := rpo.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse RPO: %w", err)
	}
	
	// Header analysis
	fmt.Println("=== RPO Analysis ===")
	fmt.Printf("Name: %s\n", info.Name)
	fmt.Printf("Size: %d bytes (%.2f MB)\n", len(data), float64(len(data))/1024/1024)
	fmt.Printf("Magic: %s\n", info.FooterMagic)
	fmt.Printf("Header size: %d bytes\n", rpo.HeaderSize)
	fmt.Printf("Footer size: %d bytes\n", rpo.FooterSize)
	fmt.Printf("Admin section: %d bytes\n", len(info.AdminSection))
	fmt.Printf("Body: %d bytes\n", len(info.Body))
	fmt.Println()
	
	// Content analysis - sample first N bytes for performance
	content := append(info.AdminSection, info.Body...)
	sampleSize := min(maxStringAnalysis, len(content))
	contentSample := content[:sampleSize]
	
	// Byte distribution
	fmt.Println("=== Byte Distribution ===")
	byteCounts := make([]int, 256)
	for _, b := range contentSample {
		byteCounts[b]++
	}
	
	fmt.Println("Most frequent bytes:")
	type freqByte struct {
		val   int
		count int
	}
	freqs := make([]freqByte, 0, 256)
	for i, count := range byteCounts {
		if count > 0 {
			freqs = append(freqs, freqByte{i, count})
		}
	}
	// Sort by count
	for i := 0; i < len(freqs); i++ {
		for j := i + 1; j < len(freqs); j++ {
			if freqs[j].count > freqs[i].count {
				freqs[i], freqs[j] = freqs[j], freqs[i]
			}
		}
	}
	for _, fb := range freqs[:10] {
		fmt.Printf("  0x%02X: %d (%.1f%%)\n", fb.val, fb.count, float64(fb.count)*100/float64(len(contentSample)))
	}
	fmt.Println()
	
	// Entropy calculation
	entropy := calculateEntropy(byteCounts, len(contentSample))
	fmt.Printf("Entropy: %.2f bits/byte\n", entropy)
	fmt.Println()
	
	// String extraction (limited)
	fmt.Println("=== Printable Strings (sample) ===")
	strings_list := extractStrings(contentSample, 4)
	for i, s := range strings_list[:min(20, len(strings_list))] {
		fmt.Printf("  [%d] +%d: %s\n", i, s.Offset, s.Text)
	}
	if len(strings_list) > 20 {
		fmt.Printf("  ... and %d more (total: %d)\n", len(strings_list)-20, len(strings_list))
	}
	fmt.Println()
	
	// Pattern analysis
	if *verbose {
		fmt.Println("=== Pattern Analysis (verbose) ===")
		analyzePatterns(contentSample)
		fmt.Println("\n=== Content Size Distribution (verbose) ===")
		analyzeSizes(contentSample)
	}
	
	return nil
}

type StringMatch struct {
	Offset int
	Text   string
}

func extractStrings(data []byte, minLength int) []StringMatch {
	matches := make([]StringMatch, 0)
	var current []byte
	
	for i, b := range data {
		if 32 <= b && b <= 126 { // Printable ASCII
			current = append(current, b)
		} else {
			if len(current) >= minLength {
				text := string(current)
				// Filter out common non-useful strings
				if !strings.Contains(text, "\\x") && !strings.HasPrefix(text, "APNSRM") {
					matches = append(matches, StringMatch{i - len(current), text})
				}
			}
			current = nil
		}
	}
	
	return matches
}

func calculateEntropy(byteCounts []int, total int) float64 {
	entropy := 0.0
	for _, count := range byteCounts {
		if count > 0 {
			p := float64(count) / float64(total)
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func analyzePatterns(data []byte) {
	fmt.Println("\nRepeating 16-byte patterns:")
	patternCounts := make(map[string]int)
	patternFirst := make(map[string]int)
	
	limit := min(len(data)-16, 100000) // Sample first 100KB
	for i := 0; i < limit; i += 16 {
		pattern := fmt.Sprintf("%x", data[i:i+16])
		patternCounts[pattern]++
		if patternCounts[pattern] == 1 {
			patternFirst[pattern] = i
		}
	}
	
	type patternStat struct {
		pattern string
		count   int
		offset  int
	}
	stats := make([]patternStat, 0, len(patternCounts))
	for p, c := range patternCounts {
		stats = append(stats, patternStat{p, c, patternFirst[p]})
	}
	
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].count > stats[i].count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
	
	for _, stat := range stats[:10] {
		fmt.Printf("  %s: %d occurrences (first at +%d)\n", stat.pattern, stat.count, stat.offset)
	}
}

func analyzeSizes(data []byte) {
	typeSizeCounts := make(map[int]int)
	limit := min(len(data)-4, 100000)
	
	for i := 0; i < limit; i += 4 {
		size := int(binary.LittleEndian.Uint32(data[i : i+4]))
		if size >= 16 && size <= 10000 {
			rounded := (size + 15) / 16 * 16
			typeSizeCounts[rounded]++
		}
	}
	
	fmt.Println("\nPotential record sizes:")
	type sizeStat struct {
		size  int
		count int
	}
	stats := make([]sizeStat, 0, len(typeSizeCounts))
	for s, c := range typeSizeCounts {
		stats = append(stats, sizeStat{s, c})
	}
	
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].size < stats[i].size {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
	
	for _, stat := range stats[:20] {
		fmt.Printf("  %d bytes: %d occurrences\n", stat.size, stat.count)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
