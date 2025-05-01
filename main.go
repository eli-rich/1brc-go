package main

import (
	"bufio"
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
	"time"
	"unicode"
)

type Stats struct {
	min   int
	max   int
	sum   int
	count int
}

func loadData(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fStat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	size := fStat.Size()
	if size == 0 {
		log.Fatalln("file is empty")
	}

	data, err := syscall.Mmap(
		int(file.Fd()),
		0,
		int(size),
		syscall.PROT_READ,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return nil, err
	}
	return data, nil
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

func processLine(line []byte, m map[string]*Stats) {
	for i := range line {
		if line[i] == ';' {
			key := string(line[:i])
			last := bytes.LastIndexByte(line, '\n')
			if last == -1 {
				last = len(line)
			}
			temp := parseFloatAsInt(line[i+1 : last])
			st, ok := m[key]
			if !ok {
				m[key] = &Stats{
					max:   temp,
					min:   temp,
					sum:   temp,
					count: 1,
				}
				break
			}
			st.count++
			st.sum += temp
			st.min = min(st.min, temp)
			st.max = max(st.max, temp)
			break
		}
	}
}

func findChunkBounds(data []byte, cores int) []int {
	size := len(data)
	bounds := make([]int, cores+1)

	bounds[0] = 0
	bounds[cores] = size

	chunkSize := size / cores

	for i := 1; i < cores; i++ {
		pos := i * chunkSize

		// Find next newline
		for pos < size && data[pos] != '\n' {
			pos++
		}
		if pos < size {
			pos++
		}
		bounds[i] = pos
	}
	return bounds
}

func processChunk(chunk []byte, start, end int) map[string]*Stats {
	localMap := make(map[string]*Stats)
	pos := start
	for pos < end {
		nextNewline := bytes.IndexByte(chunk[pos:end], '\n')
		if nextNewline == -1 {
			if end == len(chunk) {
				processLine(chunk[pos:end], localMap)
			}
			break
		}
		lineEnd := pos + nextNewline
		processLine(chunk[pos:lineEnd+1], localMap)
		pos = lineEnd + 1
	}
	return localMap
}

func main() {
	startTotal := time.Now()
	debug.SetGCPercent(-1)
	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <file_path>\n", os.Args[0])
	}
	filePath := os.Args[1]
	outpath := os.Args[2]
	startLoad := time.Now()
	data, err := loadData(filePath)
	if err != nil {
		log.Fatalf("error mmaping file: %v\n", err)
	}
	defer func() {
		if err := syscall.Munmap(data); err != nil {
			log.Fatalf("munmap error: %v\n", err)
		}
	}()

	loadDuration := time.Since(startLoad)
	startProcess := time.Now()

	cores := runtime.NumCPU()
	if len(data) < cores*1000 {
		cores = 1
	}

	bounds := findChunkBounds(data, cores)

	var wg sync.WaitGroup
	resultsChan := make(chan map[string]*Stats, cores)

	for i := range cores {
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			result := processChunk(data, start, end)
			resultsChan <- result
		}(bounds[i], bounds[i+1])
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	workerResults := make([]map[string]*Stats, 0, cores)
	for result := range resultsChan {
		workerResults = append(workerResults, result)
	}
	finalResult := mergeMaps(workerResults)

	processDuration := time.Since(startProcess)
	startOutput := time.Now()

	outfile, err := os.Create(outpath)
	if err != nil {
		log.Fatalf("error creating output file: %v\n", err)
	}
	defer outfile.Close()
	writer := bufio.NewWriter(outfile)
	dumpSorted(finalResult, writer)

	outputDuration := time.Since(startOutput)
	totalDuration := time.Since(startTotal)

	PrintPerformanceReport(loadDuration, processDuration, outputDuration, totalDuration, len(data), cores)
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

func sanitize(s string) string {
	var result strings.Builder
	for _, r := range s {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if isLower || isUpper || isDigit {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

func mergeStats(target, source *Stats) {
	target.count += source.count
	target.sum += source.sum
	target.min = min(target.min, source.min)
	target.max = max(target.max, source.max)
}

func mergeMaps(workerMaps []map[string]*Stats) map[string]*Stats {
	finalMap := make(map[string]*Stats)
	for _, workerMap := range workerMaps {
		for key, stats := range workerMap {
			if current, ok := finalMap[key]; ok {
				mergeStats(current, stats)
			} else {
				finalMap[key] = &Stats{
					min:   stats.min,
					max:   stats.max,
					sum:   stats.sum,
					count: stats.count,
				}
			}
		}
	}
	return finalMap
}

func dumpSorted(result map[string]*Stats, w *bufio.Writer) {
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		cleanFirst := sanitize(keys[i])
		cleanSecond := sanitize(keys[j])

		if cleanFirst != cleanSecond {
			return cleanFirst < cleanSecond
		}

		return keys[i] < keys[j]
	})
	for i, key := range keys {
		st := result[key]
		minOut := float64(st.min) / 10
		meanOut := roundCeil(float64(st.sum) / float64(st.count) / 10.0)
		maxOut := float64(st.max) / 10
		if i == len(keys)-1 {
			fmt.Fprintf(w, "%s=%.1f/%.1f/%.1f", key, minOut, meanOut, maxOut)
		} else {
			fmt.Fprintf(w, "%s=%.1f/%.1f/%.1f\n", key, minOut, meanOut, maxOut)
		}
	}
	w.Flush()
}
