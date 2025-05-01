# One Billion Row Challenge (1BRC) - Go Implementation

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)

This repository contains a high-performance Go implementation of the [One Billion Row Challenge](https://github.com/gunnarmorling/1brc), a benchmark task that processes a large dataset of weather station measurements.

## 📖 What is the 1BRC?

The One Billion Row Challenge is a performance-oriented programming challenge where the goal is to process one billion rows of weather station measurements as efficiently as possible. Each row contains a city name and a temperature value, and the task is to calculate the minimum, mean, and maximum temperatures for each unique city.

### Data Format

- Input: Each line follows the format `<city name>;<temperature>`

  - City names are between 4-13 characters
  - Temperatures range from -99.9 to 99.9 with exactly one decimal place
  - Up to 20,102 unique cities

- Output: Results in the format `<city>=<min>/<mean>/<max>` sorted alphabetically

## 🚀 Key Features

- **Memory Mapping**: Uses `mmap` for efficient file I/O instead of standard file reading
- **Parallel Processing**: Utilizes all available CPU cores to process data concurrently
- **Custom Numeric Parsing**: Implements specialized number parsing for better performance
- **Memory Optimization**: Minimizes allocations and disables GC during processing
- **Detailed Performance Metrics**: Provides comprehensive execution statistics

## 🔧 Implementation Details

This implementation includes several optimizations:

- **Memory-Mapped Files**: Direct memory access to file contents without copying
- **Chunked Processing**: Divides the workload among multiple goroutines
- **Garbage Collection Control**: Disables GC during processing to prevent pauses
- **Custom Float Parsing**: Parses temperatures as integers (×10) to avoid floating-point operations
- **Reduced Memory Allocations**: Reuses data structures where possible

## 📊 Performance

The implementation includes detailed performance reporting that shows:

- File loading time
- Processing time
- Output writing time
- Total execution time
- Processing speed (million lines/second)
- Data throughput (MB/second)

## 🛠️ Usage

### Prerequisites

- Go 1.21 or higher

### Running the Program

```bash
go build
./1brc <input_file_path> <output_file_path>
```

### Example

```bash
./1brc measurements.txt results.txt
```

### Expected Output

The program writes results to the specified output file in the format:

```
Abu Dhabi=24.5/36.4/47.9
Abuja=13.2/27.3/41.5
Accra=16.8/28.0/39.2
...
```

Additionally, it prints a performance report to the console showing execution metrics.

## 🌱 Generating Test Data

You can generate test data using the companion generator tool:

```bash
go run github.com/eli-rich/1brc-gen@v1.1.0 [options]
```

Options:

- `-lc`: Line count (default: 1 billion)
- `-o`: Output file (default: `out.txt`)
- `-s`: Random seed (default: 2002)
- `-c`: Chunk size in lines (default: 100,000)
- `-b`: Buffers per worker (default: 2)

## 🧪 Validation

All output can be verified using the `results.txt` file generated with the default seed (2002).

## 🤓 Technical Notes

1. **Temperature Handling**:

   - Temperatures are stored as integers (multiplied by 10) to avoid floating-point operations
   - Mean values are rounded using a ceiling function with a small epsilon adjustment

2. **Memory Management**:

   - Memory mapping provides zero-copy access to file data
   - GC is disabled during processing to prevent pauses

3. **Sorting Logic**:
   - City names are sorted case-insensitively
   - Only alphanumeric characters are considered for sorting

## ⚠️ Limitations

- Large input files require sufficient RAM to memory-map the entire file
- The implementation is optimized for the specific 1BRC data format and may not be suitable for other data processing tasks without modification

## 📜 License

This project is open source and available under the [MIT License](LICENSE).
