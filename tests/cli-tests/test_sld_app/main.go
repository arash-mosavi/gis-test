package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"gis-test/pkg/sld"
)

func main() {
	// Parse command-line arguments
	stylesDir := flag.String("styles-dir", "./styles", "Directory containing SLD files")
	sldFile := flag.String("sld-file", "blue_cities_correct.sld", "SLD file to parse")
	layerName := flag.String("layer", "cities", "Layer name for styling")
	useGDALCLI := flag.Bool("use-gdal", false, "Use GDAL CLI adapter")
	outputFile := flag.String("output", "", "Output file for rendered result")
	flag.Parse()

	// Ensure styles directory exists
	absStylesDir, err := filepath.Abs(*stylesDir)
	if err != nil {
		log.Fatalf("Failed to resolve styles directory path: %v", err)
	}

	log.Printf("Using styles directory: %s", absStylesDir)
	log.Printf("Processing SLD file: %s", *sldFile)

	var processor interface {
		ProcessStyles(sldParam, layerName string) ([]string, error)
	}

	// Create the appropriate parser based on flags
	if *useGDALCLI {
		log.Println("Using GDAL CLI adapter")
		processor = sld.NewGDALAdapter(absStylesDir)

		// If GDAL adapter is used and output file is specified, demonstrate ogr2ogr usage
		if *outputFile != "" {
			adapter := processor.(*sld.GDALAdapter)
			inputVector := "PG:host=127.0.0.1 port=5432 dbname=GIS user=TestDev_User password=123456789AAAA sslmode=disable"
			sldPath := filepath.Join(absStylesDir, *sldFile)

			log.Printf("Testing ogr2ogr with SLD: %s", sldPath)
			err := adapter.RasterizeWithOgr2Ogr(sldPath, inputVector, *outputFile, 1000, 1000, [4]float64{-180, -90, 180, 90})
			if err != nil {
				log.Printf("ogr2ogr error: %v", err)
			} else {
				log.Printf("Successfully created output file: %s", *outputFile)
			}
		}
	} else {
		log.Println("Using XML-based SLD parser")
		processor = sld.NewGDALParser(absStylesDir, false)
	}

	// Process the SLD file
	options, err := processor.ProcessStyles(*sldFile, *layerName)
	if err != nil {
		log.Fatalf("Failed to process SLD: %v", err)
	}

	// Display the resulting GDAL options
	fmt.Println("\nGDAL Options:")
	for i := 0; i < len(options); i += 2 {
		if i+1 < len(options) {
			fmt.Printf("  %s: %s\n", options[i], options[i+1])
		} else {
			fmt.Printf("  %s\n", options[i])
		}
	}
}
