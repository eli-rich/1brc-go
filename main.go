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
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type Stats struct {
	min   float64
	max   float64
	sum   float64
	count uint64
}

func unsafeStr(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func parseFloat(b []byte) float64 {
	i := 0
	neg := false
	if b[0] == '-' {
		neg = true
		i++
	}

	// integer part
	var ip int
	for ; i < len(b); i++ {
		c := b[i]
		if c < '0' || c > '9' {
			break
		}
		ip = ip*10 + int(c-'0')
	}

	// fractional part
	var fp int
	div := 1.0
	if i < len(b) && b[i] == '.' {
		i++
		for ; i < len(b); i++ {
			c := b[i]
			if c < '0' || c > '9' {
				break
			}
			fp = fp*10 + int(c-'0')
			div *= 10
		}
	}

	// combine the parts
	f := float64(ip) + float64(fp)/div
	if neg {
		return -f
	}
	return f
}

func main() {
	startTime := time.Now()
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

			// ensure start is always on valid new line
			if start != 0 {
				for start < end && data[start-1] != '\n' {
					start++
				}
			}

			// main parse loop
			for start < end {
				idx := bytes.IndexByte(data[start:end], '\n')
				if idx == -1 {
					break
				}
				line := data[start : start+idx]
				start += idx + 1

				// split on semicolon
				sep := bytes.LastIndexByte(line, ';')
				if sep == -1 {
					log.Fatalf("no seperator (;) found on line:\n%s\n", line)
				}
				keyBytes := line[:sep]
				numBytes := line[sep+1:]

				key := unsafeStr(keyBytes)
				val := parseFloat(numBytes)

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
					if val < st.min {
						st.min = val
					}
					if val > st.max {
						st.max = val
					}
				}
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
				if st.min < sst.min {
					sst.min = st.min
				}
				if st.max > sst.max {
					sst.max = st.max
				}
			} else {
				result[key] = st
			}
		}
	}
	dumpSorted(result)
	fmt.Printf("\n")
	fmt.Println(time.Since(startTime))
}

func roundCeil(n float64) float64 {
	return math.Ceil(n*10) / 10
}

func dumpSorted(result map[string]*Stats) {
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		st := result[key]
		minOut := roundCeil(st.min)
		meanOut := roundCeil(st.sum / float64(st.count))
		maxOut := roundCeil(st.max)
		if i == len(keys)-1 {
			fmt.Printf("%s=%.1f/%.1f/%.1f", key, minOut, meanOut, maxOut)
		} else {
			fmt.Printf("%s=%.1f/%.1f/%.1f\n", key, minOut, meanOut, maxOut)
		}
	}
}
