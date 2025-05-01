package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	COLOR_RESET  = "\033[0m"
	COLOR_RED    = "\033[31m"
	COLOR_GREEN  = "\033[32m"
	COLOR_YELLOW = "\033[33m"
	COLOR_BLUE   = "\033[36m"
	COLOR_BOLD   = "\033[1m"
)

func FormatDuration(d time.Duration) string {
	// For microsecond precision (less than 1 millisecond)
	if d.Milliseconds() < 1 {
		return fmt.Sprintf("%.2f μs", float64(d.Nanoseconds())/1000.0)
	}

	// For millisecond precision (less than 1 second)
	if d.Seconds() < 1 {
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1000000.0)
	}

	// For second precision (less than 1 minute)
	if d.Seconds() < 60 {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}

	// For minute precision (less than 1 hour)
	if d.Minutes() < 60 {
		return fmt.Sprintf("%dm %.2fs", int(d.Minutes()), d.Seconds()-math.Floor(d.Minutes())*60)
	}

	// For hour precision
	return fmt.Sprintf("%dh %dm %.2fs", int(d.Hours()),
		int(d.Minutes())%60,
		d.Seconds()-math.Floor(d.Minutes())*60)
}

func FormatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func PrintPerformanceReport(loadTime, processTime, outputTime, totalTime time.Duration, dataSize, numCores int) {
	// Calculate processing metrics
	linesPerSecond := 1_000_000_000 / totalTime.Seconds()
	millionLinesPerSecond := linesPerSecond / 1_000_000

	// Create a box for the output
	width := 70
	horizontalLine := strings.Repeat("─", width)
	fmt.Printf("\n%s%s%s\n", COLOR_BLUE, horizontalLine, COLOR_RESET)
	fmt.Printf("%s%s ✨ 1BRC PERFORMANCE REPORT ✨ %s%s\n",
		COLOR_BLUE, COLOR_BOLD, COLOR_RESET, COLOR_BLUE)
	fmt.Printf("%s%s\n", horizontalLine, COLOR_RESET)

	// Print configuration
	fmt.Printf("  %s%sCONFIGURATION%s\n", COLOR_BOLD, COLOR_YELLOW, COLOR_RESET)
	fmt.Printf("  Cores Used:      %s%d%s\n", COLOR_GREEN, numCores, COLOR_RESET)
	fmt.Printf("  Input Size:      %s%s%s\n", COLOR_GREEN, FormatBytes(dataSize), COLOR_RESET)

	// Print timing table
	fmt.Printf("\n  %s%sTIMING BREAKDOWN%s\n", COLOR_BOLD, COLOR_YELLOW, COLOR_RESET)
	fmt.Printf("  %-15s %s%14s%s %7.2f%%\n", "File Loading:", COLOR_GREEN,
		FormatDuration(loadTime), COLOR_RESET,
		float64(loadTime.Nanoseconds())/float64(totalTime.Nanoseconds())*100)

	fmt.Printf("  %-15s %s%14s%s %7.2f%%\n", "Processing:", COLOR_GREEN,
		FormatDuration(processTime), COLOR_RESET,
		float64(processTime.Nanoseconds())/float64(totalTime.Nanoseconds())*100)

	fmt.Printf("  %-15s %s%14s%s %7.2f%%\n", "Output Writing:", COLOR_GREEN,
		FormatDuration(outputTime), COLOR_RESET,
		float64(outputTime.Nanoseconds())/float64(totalTime.Nanoseconds())*100)

	fmt.Printf("  %s%s%s\n", COLOR_BLUE, strings.Repeat("┄", width-4), COLOR_RESET)

	fmt.Printf("  %-15s %s%14s%s\n", "Total Time:", COLOR_BOLD+COLOR_GREEN,
		FormatDuration(totalTime), COLOR_RESET)

	// Print performance metrics
	fmt.Printf("\n  %s%sPERFORMANCE METRICS%s\n", COLOR_BOLD, COLOR_YELLOW, COLOR_RESET)
	fmt.Printf("  Processing Speed: %s%.2f million lines/sec%s\n",
		COLOR_BOLD+COLOR_GREEN, millionLinesPerSecond, COLOR_RESET)
	fmt.Printf("  Throughput:       %s%.2f MB/sec%s\n",
		COLOR_BOLD+COLOR_GREEN, float64(dataSize)/(1024*1024)/totalTime.Seconds(), COLOR_RESET)

	fmt.Printf("%s%s%s\n\n", COLOR_BLUE, horizontalLine, COLOR_RESET)
}
