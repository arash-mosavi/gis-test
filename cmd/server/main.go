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
	pgConnStr   = "PG:host=127.0.0.1 port=5432 dbname=GIS user=TestDev_User password=123456789AAAA sslmode=disable"
	layerIndex  = 0
	defaultSRID = "EPSG:4326"
	stylesDir   = "./styles"
)

var (
	sldProcessor SLDProcessor
	useGDALCLI   bool
)

type SLDProcessor interface {
	ProcessStyles(sldParam, layerName string) ([]string, error)
}

func main() {

	flag.BoolVar(&useGDALCLI, "use-gdal-cli", false, "Use GDAL CLI tools for SLD processing")
	flag.Parse()

	godal.RegisterAll()

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

	required := []string{"SERVICE", "VERSION", "REQUEST", "LAYERS", "BBOX", "WIDTH", "HEIGHT", "FORMAT"}
	for _, k := range required {
		if c.QueryParam(k) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("missing %s", k)})
		}
	}
	if c.QueryParam("REQUEST") != "GetMap" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "REQUEST must be GetMap"})
	}

	width, _ := strconv.Atoi(c.QueryParam("WIDTH"))
	height, _ := strconv.Atoi(c.QueryParam("HEIGHT"))
	bboxParts := strings.Split(c.QueryParam("BBOX"), ",")

	srcDS, err := godal.Open(pgConnStr, godal.VectorOnly())
	if err != nil {
		return c.String(http.StatusInternalServerError, "GDAL Open error: "+err.Error())
	}
	defer srcDS.Close()

	layerName := c.QueryParam("LAYERS")
	layer := srcDS.LayerByName(layerName)
	if layer == nil {

		layers := srcDS.Layers()
		var layerNames []string
		for _, l := range layers {
			layerNames = append(layerNames, l.Name())
		}
		return c.String(http.StatusInternalServerError,
			fmt.Sprintf("layer not found: %s. Available layers: %v", layerName, layerNames))
	}

	sldParam := c.QueryParam("SLD")
	if sldParam == "" {
		sldParam = c.QueryParam("STYLES")
	}
	rasterizeOptions, err := sldProcessor.ProcessStyles(sldParam, layerName)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	log.Printf("[DEBUG] Rasterize options for %s: %v", sldParam, rasterizeOptions)

	outDS, err := godal.Create("MEM", "", 3, godal.Byte, width, height)
	if err != nil {
		return c.String(http.StatusInternalServerError, "create MEM error: "+err.Error())
	}
	defer outDS.Close()

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

	if err := outDS.RasterizeInto(srcDS, rasterizeOptions); err != nil {
		return c.String(http.StatusInternalServerError, "rasterize error: "+err.Error())
	}

	_, exists := godal.RasterDriver("PNG")
	if !exists {
		return c.String(http.StatusInternalServerError, "PNG driver not found")
	}

	pngDS, err := outDS.Translate("/vsimem/out.png", []string{"-of", "PNG"})
	if err != nil {
		return c.String(http.StatusInternalServerError, "PNG creation error: "+err.Error())
	}
	defer pngDS.Close()

	vsiFile, err := godal.VSIOpen("/vsimem/out.png")
	if err != nil {
		return c.String(http.StatusInternalServerError, "read PNG error: "+err.Error())
	}
	defer vsiFile.Close()

	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	_, err = io.Copy(c.Response(), vsiFile)
	return err
}
