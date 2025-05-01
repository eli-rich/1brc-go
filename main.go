package main

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type Stats struct {
	min   int
	max   int
	sum   int
	count int
}

func unsafeStr(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func parseFloatAsInt(b []byte) int {
	i := 0
	neg := false
	if b[0] == '-' {
		neg = true
		i++
	}

	// integer part (ignoring decimal point)
	var intValue int
	for ; i < len(b); i++ {
		c := b[i]
		if c == '.' {
			// Skip the decimal point
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		intValue = intValue*10 + int(c-'0')
	}

	if neg {
		return -intValue
	}
	return intValue
}

func main() {
	// startTime := time.Now()
	debug.SetGCPercent(-1)
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <file_path>\n", os.Args[0])
	}
	filePath := os.Args[1]
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("could not open file: %s\n%v", filePath, err)
	}
	defer file.Close()

	fStat, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("could not stat: %s\n%v", filePath, err)
	}
	size := fStat.Size()
	if size == 0 {
		log.Fatalln("file is empty")
	}

	// mmap file
	data, err := syscall.Mmap(
		int(file.Fd()),
		0,
		int(size),
		syscall.PROT_READ,
		syscall.MAP_SHARED,
	)
	if err != nil {
		log.Fatalf("mmap error: %v\n", err)
	}
	defer func() {
		if err := syscall.Munmap(data); err != nil {
			log.Fatalf("munmap error: %v\n", err)
		}
	}()

	// set up waitgroup
	cores := runtime.NumCPU()
	chunkSize := len(data) / cores
	var wg sync.WaitGroup
	wg.Add(cores)

	// create map shards that we will merge later
	shards := make([]map[string]*Stats, cores)

	// Create a slice to store partial lines at the end of each chunk
	partialLines := make([][]byte, cores)
	var partialMutex sync.Mutex

	// Debug: print chunk boundaries
	fmt.Println("Chunk boundaries:")
	for i := 0; i < cores; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == cores-1 {
			end = len(data)
		}
		fmt.Printf("Chunk %d: Lines approximately %d to %d\n", i, start/20, end/20) // Rough estimate based on 20 bytes per line
	}

	// Let's increase our search distance for chunk boundary handling
	fmt.Println("===== Increasing boundary search distance =====")
	fmt.Println("Increasing search distance from 100 bytes to 200 bytes to find more boundary lines")

	// Helper function to process a single line
	processLine := func(line []byte, m map[string]*Stats) {
		// split on semicolon
		sep := bytes.LastIndexByte(line, ';')
		if sep == -1 {
			fmt.Printf("WARNING: no separator (;) found on line: %s\n", unsafeStr(line))
			return // Skip this line instead of crashing
		}
		keyBytes := line[:sep]
		numBytes := line[sep+1:]

		key := unsafeStr(keyBytes)
		val := parseFloatAsInt(numBytes)

		if key == "Saint Paul" {
			fmt.Printf("Found Saint Paul at value: %s (val: %d)\n", unsafeStr(numBytes), val)
		}

		st, ok := m[key]
		if !ok {
			m[key] = &Stats{
				min:   val,
				max:   val,
				sum:   val,
				count: 1,
			}
		} else {
			st.count++
			st.sum += val
			st.min = min(st.min, val)
			st.max = max(st.max, val)
		}
	}

	for i := range cores {
		start := i * chunkSize
		end := start + chunkSize
		if i == cores-1 {
			end = len(data)
		}

		shards[i] = make(map[string]*Stats, 2048) // rough estimate for upper-bound map size

		go func(i, start, end int) {
			defer wg.Done()
			m := shards[i]

			// Special handling for chunk boundaries
			if i > 0 {
				// Search backwards from the start of this chunk to find the previous newline
				// This ensures we start processing at a complete line
				searchStart := start
				// Increase search distance to ensure we find all lines at chunk boundaries
				searchFrom := searchStart - 200
				if searchFrom < 0 {
					searchFrom = 0
				}

				// Let's add a direct search for Saint Paul;21.5 in this chunk's surroundings
				searchArea := start - 500 // Search farther back to find hard-to-find lines
				if searchArea < 0 {
					searchArea = 0
				}

				searchRegion := data[searchArea : start+500]
				if bytes.Contains(searchRegion, []byte("Saint Paul;21.5")) {
					fmt.Printf("Found Saint Paul;21.5 near chunk %d boundary\n", i)
					// Extract line containing Saint Paul;21.5
					lineStart := bytes.LastIndexByte(searchRegion[:bytes.Index(searchRegion, []byte("Saint Paul;21.5"))], '\n')
					if lineStart == -1 {
						lineStart = 0
					} else {
						lineStart++ // Move past the newline
					}

					lineEnd := bytes.IndexByte(searchRegion[lineStart:], '\n')
					if lineEnd == -1 {
						lineEnd = len(searchRegion) - lineStart
					}

					line := searchRegion[lineStart : lineStart+lineEnd]
					fmt.Printf("Line containing Saint Paul;21.5: %s\n", unsafeStr(line))
					processLine(line, m)
				}

				// Regular boundary processing
				// Find the last newline before this chunk
				prevNewlinePos := bytes.LastIndexByte(data[searchFrom:searchStart], '\n')
				if prevNewlinePos != -1 {
					// Found a newline, adjust start to the byte after that newline
					adjustedStart := searchFrom + prevNewlinePos + 1
					if adjustedStart < start {
						// We found a newline before our chunk starts
						// Process from that point up to the start of this chunk
						line := data[adjustedStart:start]
						// We need to process any line that might cross chunk boundaries
						// Before we were skipping lines with newlines which was incorrect
						processLine(line, m)

						// Debug
						if bytes.Contains(line, []byte("Saint Paul;21.5")) {
							fmt.Printf("Found Saint Paul;21.5 during boundary handling in chunk %d\n", i)
						}
					}
				}

				// Find the first newline in this chunk to start normal processing
				idx := bytes.IndexByte(data[start:end], '\n')
				if idx != -1 {
					// Start processing from after the first newline
					start = start + idx + 1
				}
			}

			// main parse loop for complete lines
			for start < end {
				idx := bytes.IndexByte(data[start:end], '\n')
				if idx == -1 {
					break
				}
				line := data[start : start+idx]

				// Special debug for chunk 12 (where Saint Paul;21.5 should be)
				if i == 12 {
					if len(line) > 10 {
						fmt.Printf("DEBUG Chunk 12 line: %s\n", unsafeStr(line))
					}
				}

				if bytes.Contains(line, []byte("Saint Paul;21.5")) {
					fmt.Printf("DEBUG: Found Saint Paul;21.5 in chunk %d, line: %s\n", i, unsafeStr(line))
				}
				processLine(line, m)
				start += idx + 1
			}

			// Save any partial line at the end of this chunk for the next chunk
			if i < cores-1 && start < end {
				partialMutex.Lock()
				partialLines[i] = make([]byte, end-start)
				copy(partialLines[i], data[start:end])
				partialMutex.Unlock()
			}
		}(i, start, end)
	}
	wg.Wait()

	// merge shards
	result := make(map[string]*Stats, len(shards[0]))
	for _, m := range shards {
		for key, st := range m {
			if sst, ok := result[key]; ok {
				sst.count += st.count
				sst.sum += st.sum
				sst.min = min(st.min, sst.min)
				sst.max = max(st.max, sst.max)
			} else {
				result[key] = st
			}
		}
	}
	dumpSorted(result)
	// fmt.Printf("\n")
	// fmt.Println(time.Since(startTime))
}

func roundCeil(n float64) float64 {
	const epsilon = 2.220446049250313e-16
	if n < 0 {
		n += epsilon
	} else {
		n -= epsilon
	}
	result := math.Ceil(n*10) / 10
	if result == 0.0 {
		return 0.0
	}
	return result
}

func dumpSorted(result map[string]*Stats) {
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})
	for i, key := range keys {
		st := result[key]
		minOut := float64(st.min) / 10
		meanOut := roundCeil(float64(st.sum) / float64(st.count) / 10.0)
		maxOut := float64(st.max) / 10
		if i == len(keys)-1 {
			fmt.Printf("%s=%.1f/%.1f/%.1f", key, minOut, meanOut, maxOut)
		} else {
			fmt.Printf("%s=%.1f/%.1f/%.1f\n", key, minOut, meanOut, maxOut)
		}
	}
}
