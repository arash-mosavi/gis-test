package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/airbusgeo/godal"
	"github.com/labstack/echo/v4"

	"gis-test/pkg/sld"
)

const (
	// GDAL PostGIS connection string
	pgConnStr   = "PG:host=127.0.0.1 port=5432 dbname=GIS user=TestDev_User password=123456789AAAA sslmode=disable"
	layerIndex  = 0
	defaultSRID = "EPSG:4326"
	stylesDir   = "./styles" // Directory containing SLD files
)

var (
	// We'll use an interface to allow different SLD parser implementations
	sldProcessor SLDProcessor
	useGDALCLI   bool
)

// SLDProcessor interface defines the common functionality for SLD processing
type SLDProcessor interface {
	ProcessStyles(sldParam, layerName string) ([]string, error)
}

func main() {
	// Parse command-line flags
	flag.BoolVar(&useGDALCLI, "use-gdal-cli", false, "Use GDAL CLI tools for SLD processing")
	flag.Parse()

	// Register all GDAL drivers
	godal.RegisterAll()

	// Initialize the appropriate SLD processor based on flags
	if useGDALCLI {
		log.Println("Using GDAL CLI adapter for SLD processing")
		sldProcessor = sld.NewGDALAdapter(stylesDir)
	} else {
		log.Println("Using XML-based SLD parser")
		sldProcessor = sld.NewGDALParser(stylesDir, false)
	}

	e := echo.New()
	e.HideBanner = true
	e.GET("/wms", getMapHandler)
	log.Println("Starting server on :8080")
	log.Fatal(e.Start(":8080"))
}

func getMapHandler(c echo.Context) error {
	// 1. Validate WMS params
	required := []string{"SERVICE", "VERSION", "REQUEST", "LAYERS", "BBOX", "WIDTH", "HEIGHT", "FORMAT"}
	for _, k := range required {
		if c.QueryParam(k) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("missing %s", k)})
		}
	}
	if c.QueryParam("REQUEST") != "GetMap" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "REQUEST must be GetMap"})
	}

	// 2. Parse numeric and bbox params
	width, _ := strconv.Atoi(c.QueryParam("WIDTH"))
	height, _ := strconv.Atoi(c.QueryParam("HEIGHT"))
	bboxParts := strings.Split(c.QueryParam("BBOX"), ",")

	// 3. Open the PostGIS datasource in GDAL
	// Use VectorOnly option to ensure proper layer detection
	srcDS, err := godal.Open(pgConnStr, godal.VectorOnly())
	if err != nil {
		return c.String(http.StatusInternalServerError, "GDAL Open error: "+err.Error())
	}
	defer srcDS.Close()

	// Get the specific layer by name
	layerName := c.QueryParam("LAYERS")
	layer := srcDS.LayerByName(layerName)
	if layer == nil {
		// List available layers for debugging
		layers := srcDS.Layers()
		var layerNames []string
		for _, l := range layers {
			layerNames = append(layerNames, l.Name())
		}
		return c.String(http.StatusInternalServerError,
			fmt.Sprintf("layer not found: %s. Available layers: %v", layerName, layerNames))
	}
	// We'll use the first layer for rasterization

	// 4. Process SLD styling with selected processor
	// Check both SLD and STYLES parameters for compatibility
	sldParam := c.QueryParam("SLD")
	if sldParam == "" {
		sldParam = c.QueryParam("STYLES") // Fallback to STYLES for backward compatibility
	}
	rasterizeOptions, err := sldProcessor.ProcessStyles(sldParam, layerName)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	log.Printf("[DEBUG] Rasterize options for %s: %v", sldParam, rasterizeOptions)

	// 5. Create an in-memory raster (3 bands: RGB)
	outDS, err := godal.Create("MEM", "", 3, godal.Byte, width, height)
	if err != nil {
		return c.String(http.StatusInternalServerError, "create MEM error: "+err.Error())
	}
	defer outDS.Close()

	// 6. Set geotransform and projection
	minX, _ := strconv.ParseFloat(bboxParts[0], 64)
	minY, _ := strconv.ParseFloat(bboxParts[1], 64)
	maxX, _ := strconv.ParseFloat(bboxParts[2], 64)
	maxY, _ := strconv.ParseFloat(bboxParts[3], 64)
	gt := [6]float64{
		minX,
		(maxX - minX) / float64(width),
		0,
		maxY,
		0,
		-(maxY - minY) / float64(height),
	}
	if err := outDS.SetGeoTransform(gt); err != nil {
		return c.String(http.StatusInternalServerError, "set geotransform error: "+err.Error())
	}
	if err := outDS.SetProjection(defaultSRID); err != nil {
		return c.String(http.StatusInternalServerError, "set projection error: "+err.Error())
	}

	// 7. Rasterize the vector layer into the output dataset using SLD styles
	// Using RasterizeInto method to rasterize vector data into raster
	// Options are dynamically built from SLD or use defaults
	if err := outDS.RasterizeInto(srcDS, rasterizeOptions); err != nil {
		return c.String(http.StatusInternalServerError, "rasterize error: "+err.Error())
	}

	// 8. Export to PNG and stream back
	_, exists := godal.RasterDriver("PNG")
	if !exists {
		return c.String(http.StatusInternalServerError, "PNG driver not found")
	}

	// Create a temporary PNG dataset in memory
	pngDS, err := outDS.Translate("/vsimem/out.png", []string{"-of", "PNG"})
	if err != nil {
		return c.String(http.StatusInternalServerError, "PNG creation error: "+err.Error())
	}
	defer pngDS.Close()

	// Read the PNG data from VSIMEM
	vsiFile, err := godal.VSIOpen("/vsimem/out.png")
	if err != nil {
		return c.String(http.StatusInternalServerError, "read PNG error: "+err.Error())
	}
	defer vsiFile.Close()

	// Stream the PNG data back to client
	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	_, err = io.Copy(c.Response(), vsiFile)
	return err
}
