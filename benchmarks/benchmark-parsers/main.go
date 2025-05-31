package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gis-test/pkg/sld"
)

func main() {
	// Define command line flags
	stylesDir := flag.String("styles", "./styles", "Directory containing SLD files")
	iterations := flag.Int("n", 100, "Number of iterations for performance testing")
	useCache := flag.Bool("cache", true, "Enable caching in the new parser")
	flag.Parse()

	// Check if styles directory exists
	if _, err := os.Stat(*stylesDir); os.IsNotExist(err) {
		log.Fatalf("Styles directory does not exist: %s", *stylesDir)
	}

	// Get a list of SLD files
	sldFiles, err := filepath.Glob(filepath.Join(*stylesDir, "*.sld"))
	if err != nil {
		log.Fatalf("Failed to list SLD files: %v", err)
	}

	if len(sldFiles) == 0 {
		log.Fatalf("No SLD files found in %s", *stylesDir)
	}

	// Create parsers
	oldParser := sld.NewParser(*stylesDir)
	newParser := sld.NewGDALParser(*stylesDir, false)
	newParser.SetCacheOptions(*useCache, 5*time.Minute)

	// Run performance tests
	fmt.Println("Running performance tests...")
	fmt.Printf("Number of iterations: %d\n", *iterations)
	fmt.Printf("Number of SLD files: %d\n", len(sldFiles))
	fmt.Printf("Caching enabled: %v\n", *useCache)
	fmt.Println("\nResults:")
	fmt.Printf("%-20s %-15s %-15s %-15s\n", "File", "Old Parser (ms)", "New Parser (ms)", "Improvement")
	fmt.Println(strings.Repeat("-", 70))

	// Track total times
	var totalOldTime, totalNewTime time.Duration

	// Test each SLD file
	for _, sldFile := range sldFiles {
		filename := filepath.Base(sldFile)

		// Run the old parser
		startOld := time.Now()
		for i := 0; i < *iterations; i++ {
			_, err := oldParser.ProcessStyles(filename, "cities")
			if err != nil {
				log.Printf("Error with old parser on %s: %v", filename, err)
				break
			}
		}
		oldTime := time.Since(startOld)
		totalOldTime += oldTime

		// Clear cache between tests
		newParser.ClearCache()

		// Run the new parser
		startNew := time.Now()
		for i := 0; i < *iterations; i++ {
			_, err := newParser.ProcessStyles(filename, "cities")
			if err != nil {
				log.Printf("Error with new parser on %s: %v", filename, err)
				break
			}
		}
		newTime := time.Since(startNew)
		totalNewTime += newTime

		// Calculate improvement
		improvement := float64(oldTime) / float64(newTime)

		fmt.Printf("%-20s %-15.2f %-15.2f %-15.2f\n",
			filename,
			float64(oldTime.Milliseconds())/float64(*iterations),
			float64(newTime.Milliseconds())/float64(*iterations),
			improvement)

		// Force garbage collection to get more accurate measurements
		runtime.GC()
	}

	// Print summary
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-20s %-15.2f %-15.2f %-15.2f\n",
		"Total Average",
		float64(totalOldTime.Milliseconds())/float64(*iterations*len(sldFiles)),
		float64(totalNewTime.Milliseconds())/float64(*iterations*len(sldFiles)),
		float64(totalOldTime)/float64(totalNewTime))

	fmt.Println("\nMemory Usage:")
	printMemUsage()
}

// printMemUsage outputs the current, total and OS memory usage
func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
