// package sld provides functionality for parsing and handling SLD (Styled Layer Descriptor) files.
package sld

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GDALAdapter handles SLD parsing using GDAL command line tools
type GDALAdapter struct {
	StylesDir string
	TempDir   string
}

// NewGDALAdapter creates a new SLD adapter that uses GDAL command line tools
func NewGDALAdapter(stylesDir string) *GDALAdapter {
	return &GDALAdapter{
		StylesDir: stylesDir,
		TempDir:   os.TempDir(), // Use system temp directory by default
	}
}

// ProcessStyles processes an SLD file using GDAL tools and returns options for rendering
func (g *GDALAdapter) ProcessStyles(sldParam, layerName string) ([]string, error) {
	log.Printf("Processing SLD with GDAL adapter: '%s' for layer: '%s'", sldParam, layerName)

	s := strings.TrimSpace(sldParam)
	if s == "" {
		// Default white burn
		log.Println("No SLD parameter provided, using default white")
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	var sldPath string
	var err error

	// Handle different SLD sources (URL, inline XML, file path)
	sldPath, err = g.getSLDPath(s)
	if err != nil {
		log.Printf("Error getting SLD: %v, using default style", err)
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	// Extract color information using GDAL's ogrinfo
	colorInfo, err := g.extractColorInfoWithGDAL(sldPath)
	if err != nil {
		log.Printf("Error extracting color info: %v, using default style", err)
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	// Convert color info to GDAL parameters
	options := g.colorInfoToGDALParams(colorInfo, layerName)
	log.Printf("Generated GDAL options: %v", options)

	return options, nil
}

// getSLDPath resolves the SLD parameter to a local file path
func (g *GDALAdapter) getSLDPath(sldParam string) (string, error) {
	if strings.HasPrefix(sldParam, "http://") || strings.HasPrefix(sldParam, "https://") {
		// Download URL to temp file
		log.Printf("Downloading SLD from URL: %s", sldParam)
		resp, err := http.Get(sldParam)
		if err != nil {
			return "", fmt.Errorf("failed to fetch SLD URL: %v", err)
		}
		defer resp.Body.Close()

		// Create a timestamp-based unique file name
		timestamp := time.Now().Format("20060102150405")
		tmpFile := filepath.Join(g.TempDir, fmt.Sprintf("sld-%s.xml", timestamp))

		out, err := os.Create(tmpFile)
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %v", err)
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to write SLD to temp file: %v", err)
		}

		return tmpFile, nil

	} else if strings.HasPrefix(sldParam, "<?xml") {
		// Inline XML to temp file
		log.Println("Processing inline SLD XML")

		// Create a timestamp-based unique file name
		timestamp := time.Now().Format("20060102150405")
		tmpFile := filepath.Join(g.TempDir, fmt.Sprintf("sld-%s.xml", timestamp))

		err := os.WriteFile(tmpFile, []byte(sldParam), 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write SLD to temp file: %v", err)
		}

		return tmpFile, nil

	} else {
		// Local file
		sldPath := filepath.Join(g.StylesDir, sldParam)
		log.Printf("Using local SLD file: %s", sldPath)

		if _, err := os.Stat(sldPath); os.IsNotExist(err) {
			return "", fmt.Errorf("SLD file does not exist: %s", sldPath)
		}

		return sldPath, nil
	}
}

// extractColorInfoWithGDAL extracts color information from an SLD file using GDAL tools
func (g *GDALAdapter) extractColorInfoWithGDAL(sldPath string) (map[string]string, error) {
	// This is a placeholder implementation
	// In a real-world scenario, you would use GDAL's ogrinfo or a similar tool
	// to extract styling information from the SLD file

	// For now, we'll use a simple XML parser to get basic styling info
	xmlData, err := os.ReadFile(sldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SLD file: %v", err)
	}

	// Use a simple approach to extract fill colors
	// A more robust approach would be to use a proper XML parser or GDAL utilities
	result := make(map[string]string)

	// Simple string search for fill color (this is not robust but serves as an example)
	fillColorStart := strings.Index(string(xmlData), "<CssParameter name=\"fill\">")
	if fillColorStart > 0 {
		fillColorStart += len("<CssParameter name=\"fill\">")
		fillColorEnd := strings.Index(string(xmlData)[fillColorStart:], "</CssParameter>")
		if fillColorEnd > 0 {
			fillColor := string(xmlData)[fillColorStart : fillColorStart+fillColorEnd]
			result["fillColor"] = fillColor
		}
	}

	// Alternative search with single quotes
	if _, ok := result["fillColor"]; !ok {
		fillColorStart := strings.Index(string(xmlData), "<CssParameter name='fill'>")
		if fillColorStart > 0 {
			fillColorStart += len("<CssParameter name='fill'>")
			fillColorEnd := strings.Index(string(xmlData)[fillColorStart:], "</CssParameter>")
			if fillColorEnd > 0 {
				fillColor := string(xmlData)[fillColorStart : fillColorStart+fillColorEnd]
				result["fillColor"] = fillColor
			}
		}
	}

	return result, nil
}

// colorInfoToGDALParams converts color information to GDAL parameters
func (g *GDALAdapter) colorInfoToGDALParams(colorInfo map[string]string, layerName string) []string {
	result := []string{}

	// Handle fill color if available
	if fillColor, ok := colorInfo["fillColor"]; ok && strings.HasPrefix(fillColor, "#") {
		// Convert hex color to RGB values
		// Remove the '#' prefix
		hex := strings.TrimPrefix(fillColor, "#")
		if len(hex) == 6 {
			// Parse each color component
			var r, g, b uint8
			_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
			if err == nil {
				// Add RGB burn values (removed -3d as it conflicts with multiple -burn parameters)
				result = append(result,
					"-burn", fmt.Sprintf("%d", r),
					"-burn", fmt.Sprintf("%d", g),
					"-burn", fmt.Sprintf("%d", b))
				log.Printf("Using RGB values: %d,%d,%d from color %s", r, g, b, fillColor)
			} else {
				log.Printf("Failed to parse color '%s': %v", fillColor, err)
				// Default to white
				result = append(result, "-burn", "255")
			}
		} else {
			log.Printf("Invalid hex color format: %s", fillColor)
			// Default to white
			result = append(result, "-burn", "255")
		}
	} else {
		// Default to white
		result = append(result, "-burn", "255")
	}

	// Add layer name and other fixed parameters
	result = append(result, "-l", layerName, "-at")

	return result
}

// RasterizeWithOgr2Ogr performs rasterization using ogr2ogr with SLD styling
func (g *GDALAdapter) RasterizeWithOgr2Ogr(sldPath, inputPath, outputPath string, width, height int, bbox [4]float64) error {
	// Prepare the command
	args := []string{
		"-f", "GTiff",
		outputPath,
		inputPath,
		"-t_srs", "EPSG:4326",
	}

	if sldPath != "" {
		args = append(args, "-style_table", sldPath)
	}

	cmd := exec.Command("ogr2ogr", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ogr2ogr error: %v\nOutput: %s", err, string(output))
	}

	log.Printf("Successfully processed with ogr2ogr: %s", string(output))
	return nil
}

// RasterizeWithQgisProcess performs rasterization using qgis_process with SLD styling
func (g *GDALAdapter) RasterizeWithQgisProcess(sldPath, inputPath, outputPath string, width, height int, bbox [4]float64) error {
	// Format the extent string
	extentStr := fmt.Sprintf("%f,%f,%f,%f [EPSG:4326]", bbox[0], bbox[1], bbox[2], bbox[3])

	// Prepare the command
	args := []string{
		"run",
		"native:rasterize",
		fmt.Sprintf("--INPUT=%s", inputPath),
		"--FIELD=",
		"--BURN=1",
		fmt.Sprintf("--WIDTH=%d", width),
		fmt.Sprintf("--HEIGHT=%d", height),
		fmt.Sprintf("--EXTENT=%s", extentStr),
		fmt.Sprintf("--OUTPUT=%s", outputPath),
	}

	cmd := exec.Command("qgis_process", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qgis_process error: %v\nOutput: %s", err, string(output))
	}

	log.Printf("Successfully processed with qgis_process: %s", string(output))
	return nil
}
