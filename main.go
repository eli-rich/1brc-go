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

type Leftover struct {
	buffer [20]byte
	size   int
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

func processLine(line []byte, m map[string]*Stats) map[string]*Stats {
	for i := range line {
		if line[i] == ';' {
			key := unsafeStr(line[:i])
			temp := parseFloatAsInt(line[i+1 : bytes.LastIndexByte(line, '\n')])

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
	return m
}

func processChunk(index int, chunk []byte, first bool, m map[string]*Stats) (*Leftover, map[string]*Stats) {
	leftover := &Leftover{
		buffer: [20]byte{},
		size:   0,
	}
	pos := 0
	if !first {
		appendIdx := bytes.IndexByte(chunk, '\n')
		for i := range appendIdx {
			leftover.buffer[leftover.size+i] = chunk[i]
		}
		m = processLine(leftover.buffer[:leftover.size+appendIdx], m)
		leftover.size = 0
		pos = appendIdx + 1
	}

	lineBuf := make([]byte, 20)
	lineIdx := 0
	for ; pos < len(chunk); pos++ {
		lineBuf[lineIdx] = chunk[pos]
		if chunk[pos] == '\n' {
			m = processLine(lineBuf[:lineIdx+1], m)
			lineIdx = 0
			continue
		}
		lineIdx++
	}
	if lineIdx > 0 {
		copy(leftover.buffer[:], lineBuf[:lineIdx])
		leftover.size = lineIdx
	}
	return leftover, m
}

func unsafeStr(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func main() {
	debug.SetGCPercent(-1)
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <file_path>\n", os.Args[0])
	}
	filePath := os.Args[1]
	data, err := loadData(filePath)
	if err != nil {
		log.Fatalf("error mmaping file: %v\n", err)
	}
	defer func() {
		if err := syscall.Munmap(data); err != nil {
			log.Fatalf("munmap error: %v\n", err)
		}
	}()

	cores := runtime.NumCPU()
	shards := make([]map[string]*Stats, cores)

	partialLines := make([][]byte, cores)
	var partialMutex sync.Mutex

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
