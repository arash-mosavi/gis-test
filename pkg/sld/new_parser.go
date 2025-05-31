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
	"sync"
	"time"

	"github.com/beevik/etree"
)

// GDALStylingOptions contains styling parameters for GDAL
type GDALStylingOptions struct {
	FillColor   string
	StrokeColor string
	StrokeWidth string
	Opacity     string
}

// cacheEntry represents a cached styling result
type cacheEntry struct {
	options   *GDALStylingOptions
	timestamp time.Time
	params    []string
}

// GDALParser handles SLD parsing using GDAL and proper XML parsing
type GDALParser struct {
	StylesDir    string
	UseCLI       bool // Whether to use command-line GDAL tools
	cacheEnabled bool
	cacheTimeout time.Duration // How long to keep cache entries
	cache        map[string]cacheEntry
	cacheMutex   sync.RWMutex
}

// NewGDALParser creates a new SLD parser that uses proper XML parsing and optionally GDAL command line
func NewGDALParser(stylesDir string, useCLI bool) *GDALParser {
	return &GDALParser{
		StylesDir:    stylesDir,
		UseCLI:       useCLI,
		cacheEnabled: true,
		cacheTimeout: 5 * time.Minute, // Default 5 minutes cache timeout
		cache:        make(map[string]cacheEntry),
	}
}

// SetCacheOptions configures the caching behavior
func (p *GDALParser) SetCacheOptions(enabled bool, timeout time.Duration) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.cacheEnabled = enabled
	if timeout > 0 {
		p.cacheTimeout = timeout
	}
}

// ClearCache removes all cached entries
func (p *GDALParser) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.cache = make(map[string]cacheEntry)
	log.Println("XML parser cache cleared")
}

// ProcessStyles processes SLD styles and returns GDAL options
func (p *GDALParser) ProcessStyles(sldParam, layerName string) ([]string, error) {
	log.Printf("Processing SLD: '%s' for layer: '%s'", sldParam, layerName)

	s := strings.TrimSpace(sldParam)
	if s == "" {
		// Default white burn
		log.Println("No SLD parameter provided, using default white")
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	// Check cache first if enabled
	if p.cacheEnabled {
		cacheKey := fmt.Sprintf("%s:%s", sldParam, layerName)
		if params, found := p.checkCache(cacheKey); found {
			log.Printf("Using cached SLD processing results for '%s'", sldParam)
			return params, nil
		}
	}

	var sldContent []byte
	var sldPath string
	var err error

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		// Download URL
		log.Printf("Downloading SLD from URL: %s", s)
		resp, err := http.Get(s)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch SLD URL: %v", err)
		}
		defer resp.Body.Close()
		sldContent, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read SLD from URL: %v", err)
		}

		// Save to a temporary file for GDAL processing if needed
		tmpFile, err := os.CreateTemp("", "sld-*.xml")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(sldContent); err != nil {
			return nil, fmt.Errorf("failed to write SLD to temp file: %v", err)
		}
		tmpFile.Close()
		sldPath = tmpFile.Name()

	} else if strings.HasPrefix(s, "<?xml") {
		// Inline XML
		log.Println("Processing inline SLD XML")
		sldContent = []byte(s)

		// Save to a temporary file for GDAL processing if needed
		if p.UseCLI {
			tmpFile, err := os.CreateTemp("", "sld-*.xml")
			if err != nil {
				return nil, fmt.Errorf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write(sldContent); err != nil {
				return nil, fmt.Errorf("failed to write SLD to temp file: %v", err)
			}
			tmpFile.Close()
			sldPath = tmpFile.Name()
		}
	} else {
		// Local file
		sldPath = filepath.Join(p.StylesDir, s)
		log.Printf("Reading SLD file: %s", sldPath)
		sldContent, err = os.ReadFile(sldPath)
		if err != nil {
			log.Printf("Failed to read SLD file: %v, falling back to default", err)
			return []string{"-burn", "255", "-l", layerName, "-at"}, nil
		}
	}

	// If we're using GDAL CLI tools directly
	if p.UseCLI {
		params, err := p.processWithGDALCLI(sldPath, layerName)
		if err == nil && p.cacheEnabled {
			// Cache the results
			p.cacheResult(fmt.Sprintf("%s:%s", sldParam, layerName), nil, params)
		}
		return params, err
	}

	// Fix common SLD issues before parsing
	fixedContent := p.fixSLDContent(sldContent)

	// Otherwise parse the XML and extract styling information
	options, err := p.extractStyleOptionsFromXML(fixedContent)
	if err != nil {
		log.Printf("Error parsing SLD XML: %v, using default style", err)
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	// Convert the parsed options to GDAL parameters
	params := p.styleOptionsToGDALParams(options, layerName)

	// Cache the results if enabled
	if p.cacheEnabled {
		p.cacheResult(fmt.Sprintf("%s:%s", sldParam, layerName), options, params)
	}

	return params, nil
}

// checkCache checks if a valid cached result exists for the given key
func (p *GDALParser) checkCache(key string) ([]string, bool) {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	entry, exists := p.cache[key]
	if !exists {
		return nil, false
	}

	// Check if the entry has expired
	if time.Since(entry.timestamp) > p.cacheTimeout {
		return nil, false
	}

	return entry.params, true
}

// cacheResult stores a result in the cache
func (p *GDALParser) cacheResult(key string, options *GDALStylingOptions, params []string) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	p.cache[key] = cacheEntry{
		options:   options,
		params:    params,
		timestamp: time.Now(),
	}

	log.Printf("Cached SLD processing result for '%s'", key)
}

// fixSLDContent fixes common issues with SLD files
func (p *GDALParser) fixSLDContent(content []byte) []byte {
	s := string(content)

	originalContent := s

	// Replace known problematic tags with standards-compliant ones
	// SLD should use <Name> instead of <n> for layer and rule names
	s = strings.ReplaceAll(s, "<n>", "<Name>")
	s = strings.ReplaceAll(s, "</n>", "</Name>")

	// Check if any fixes were applied and log them
	if s != originalContent {
		log.Println("SLD was fixed - replaced <n> tags with <Name>")
	} else {
		log.Println("No SLD fixes needed")
	}

	return []byte(s)
}

// processWithGDALCLI uses GDAL command line tools to process the SLD
func (p *GDALParser) processWithGDALCLI(sldPath, layerName string) ([]string, error) {
	log.Printf("Using GDAL CLI to process SLD: %s", sldPath)

	// Read the SLD file
	sldContent, err := os.ReadFile(sldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SLD file: %v", err)
	}

	// Fix common SLD issues
	fixedContent := p.fixSLDContent(sldContent)

	// Create a temporary fixed SLD file for GDAL to use
	fixedSLDFile, err := os.CreateTemp("", "fixed-sld-*.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(fixedSLDFile.Name())

	if _, err := fixedSLDFile.Write(fixedContent); err != nil {
		return nil, fmt.Errorf("failed to write fixed SLD to temp file: %v", err)
	}
	fixedSLDFile.Close()

	// Use ogrinfo to extract styling information
	// This is a simplified approach - in a real implementation, you would parse the output
	cmd := exec.Command(
		"ogrinfo",
		"-so",
		"-ro",
		fixedSLDFile.Name(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fall back to default params if ogrinfo fails
		log.Printf("ogrinfo failed: %v, using default parameters", err)
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	// Extract color information from the ogrinfo output
	// This is a simplified approach - in a real implementation, you would parse the output properly
	outputStr := string(output)
	params := []string{}

	// Look for color information in the output
	// For example, if we find "FILL_COLOR=#0000FF", extract the blue color
	if strings.Contains(outputStr, "FILL_COLOR=#0000FF") {
		params = append(params, "-burn", "0", "-burn", "0", "-burn", "255", "-3d")
	} else if strings.Contains(outputStr, "FILL_COLOR=#FF0000") {
		params = append(params, "-burn", "255", "-burn", "0", "-burn", "0", "-3d")
	} else if strings.Contains(outputStr, "FILL_COLOR=#00FF00") {
		params = append(params, "-burn", "0", "-burn", "255", "-burn", "0", "-3d")
	} else {
		// Default to white if no color found
		params = append(params, "-burn", "255")
	}

	// Add layer name and other fixed parameters
	params = append(params, "-l", layerName, "-at")

	return params, nil
}

// extractStyleOptionsFromXML parses the SLD XML and extracts styling options
func (p *GDALParser) extractStyleOptionsFromXML(content []byte) (*GDALStylingOptions, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(content); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	options := &GDALStylingOptions{}

	// Find all CssParameter elements with appropriate names
	root := doc.SelectElement("StyledLayerDescriptor")
	if root == nil {
		return nil, fmt.Errorf("missing StyledLayerDescriptor element")
	}

	// Navigate through the XML structure to find styling parameters
	// Note: This is a simplified approach; a complete implementation would handle all SLD complexities
	namedLayer := root.FindElement("//NamedLayer")
	if namedLayer == nil {
		return nil, fmt.Errorf("missing NamedLayer element")
	}

	// Extract layer name (now using <Name> after our fix)
	layerName := namedLayer.FindElement("//Name")
	if layerName != nil {
		log.Printf("Found layer name: %s", layerName.Text())
	}

	userStyle := namedLayer.FindElement("//UserStyle")
	if userStyle == nil {
		return nil, fmt.Errorf("missing UserStyle element")
	}

	// Find FeatureTypeStyle (some SLDs might have multiple)
	featureTypeStyles := userStyle.FindElements("//FeatureTypeStyle")
	if len(featureTypeStyles) == 0 {
		return nil, fmt.Errorf("missing FeatureTypeStyle element")
	}

	// Process all feature type styles, but use the first one that has valid styling
	for _, featureTypeStyle := range featureTypeStyles {
		rules := featureTypeStyle.FindElements("//Rule")
		if len(rules) == 0 {
			continue // Skip if no rules
		}

		// Process all rules, but use the first one with valid styling
		for _, rule := range rules {
			// First look for PolygonSymbolizer
			symbolizer := rule.FindElement(".//PolygonSymbolizer")
			if symbolizer == nil {
				// Try LineSymbolizer as fallback
				symbolizer = rule.FindElement(".//LineSymbolizer")
				if symbolizer == nil {
					// Try PointSymbolizer as fallback
					symbolizer = rule.FindElement(".//PointSymbolizer")
					if symbolizer == nil {
						continue // Skip to next rule if no symbolizer found
					}
				}
			}

			// Extract opacity if available at the symbolizer level
			if opacity := symbolizer.SelectAttrValue("opacity", ""); opacity != "" {
				options.Opacity = opacity
				log.Printf("Found symbolizer opacity: %s", opacity)
			}

			// Extract Fill parameters if available
			if fill := symbolizer.FindElement(".//Fill"); fill != nil {
				// Check for opacity at the fill level
				if opacity := fill.SelectAttrValue("opacity", ""); opacity != "" {
					options.Opacity = opacity
					log.Printf("Found fill opacity: %s", opacity)
				}

				// Check all CssParameters
				for _, param := range fill.FindElements(".//CssParameter") {
					if name := param.SelectAttrValue("name", ""); name == "fill" {
						options.FillColor = param.Text()
						log.Printf("Found fill color: %s", options.FillColor)
					} else if name == "fill-opacity" {
						options.Opacity = param.Text()
						log.Printf("Found fill-opacity: %s", options.Opacity)
					}
				}
			}

			// Extract Stroke parameters if available
			if stroke := symbolizer.FindElement(".//Stroke"); stroke != nil {
				// Check for opacity at the stroke level
				if opacity := stroke.SelectAttrValue("opacity", ""); opacity != "" {
					if options.Opacity == "" { // Don't override fill opacity
						options.Opacity = opacity
						log.Printf("Found stroke opacity: %s", opacity)
					}
				}

				for _, param := range stroke.FindElements(".//CssParameter") {
					name := param.SelectAttrValue("name", "")
					if name == "stroke" {
						options.StrokeColor = param.Text()
						log.Printf("Found stroke color: %s", options.StrokeColor)
					} else if name == "stroke-width" {
						options.StrokeWidth = param.Text()
						log.Printf("Found stroke width: %s", options.StrokeWidth)
					} else if name == "stroke-opacity" {
						if options.Opacity == "" { // Don't override fill opacity
							options.Opacity = param.Text()
							log.Printf("Found stroke-opacity: %s", options.Opacity)
						}
					}
				}
			}

			// If we found at least fill or stroke color, we're good with this rule
			if options.FillColor != "" || options.StrokeColor != "" {
				return options, nil
			}
		}
	}

	// If no styling found after checking all rules, return an error
	return nil, fmt.Errorf("no valid styling found in SLD")
}

// styleOptionsToGDALParams converts styling options to GDAL command line parameters
func (p *GDALParser) styleOptionsToGDALParams(options *GDALStylingOptions, layerName string) []string {
	result := []string{}

	// Handle fill color for rasterization
	if options.FillColor != "" && strings.HasPrefix(options.FillColor, "#") {
		// Convert hex color to RGB values
		r, g, b, _, err := hexColorToRGBA(options.FillColor)
		if err == nil {
			// Use RGB burn values - note: removed -3d flag as it conflicts with multiple -burn parameters
			// Burn RGB and alpha (opaque)
			result = append(result,
				"-burn", fmt.Sprintf("%d", r),
				"-burn", fmt.Sprintf("%d", g),
				"-burn", fmt.Sprintf("%d", b),
				// alpha channel
				"-burn", "255")
			log.Printf("Using RGB values: %d,%d,%d", r, g, b)
		} else {
			log.Printf("Failed to parse color '%s': %v", options.FillColor, err)
			// Default to white
			result = append(result, "-burn", "255")
		}
	} else {
		// Default to white fill and opaque
		result = append(result,
			// R, G, B
			"-burn", "255", "-burn", "255", "-burn", "255",
			// Alpha
			"-burn", "255")
	}

	// Add layer name and other fixed parameters
	result = append(result, "-l", layerName, "-at")

	return result
}

// hexColorToRGBA converts a hex color string (#RRGGBB) to RGB integer values
func hexColorToRGBA(hex string) (r, g, b, a int, err error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, 0, fmt.Errorf("invalid hex color format")
	}

	// Parse the hex color
	var color uint64
	color, err = parseHexColor(hex)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Extract RGB components
	r = int((color >> 16) & 0xFF)
	g = int((color >> 8) & 0xFF)
	b = int(color & 0xFF)
	a = 255 // Default alpha

	return r, g, b, a, nil
}

// parseHexColor parses a hex string into a uint64
func parseHexColor(hex string) (uint64, error) {
	return parseHexUint64("0x" + hex)
}

// parseHexUint64 parses a hex string with 0x prefix into a uint64
func parseHexUint64(hex string) (uint64, error) {
	var result uint64
	_, err := fmt.Sscanf(hex, "0x%x", &result)
	return result, err
}

// ProcessWithQgisProcess processes an SLD file using the qgis_process tool
func (p *GDALParser) ProcessWithQgisProcess(sldPath, layerPath, outputPath string) error {
	// Example command, you'd customize based on your needs
	cmd := exec.Command(
		"qgis_process", "run",
		"native:rasterize",
		"--INPUT="+layerPath,
		"--FIELD=id",
		"--BURN=1",
		"--WIDTH=1000",
		"--HEIGHT=1000",
		"--EXTENT=0,0,1000,1000",
		"--OUTPUT="+outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qgis_process error: %v\nOutput: %s", err, string(output))
	}

	log.Printf("Successfully processed with qgis_process: %s", string(output))
	return nil
}

// ProcessWithOgr2Ogr processes vector data with SLD styling using ogr2ogr
func (p *GDALParser) ProcessWithOgr2Ogr(sldPath, inputPath, outputPath string) error {
	// Example command, you'd customize based on your needs
	cmd := exec.Command(
		"ogr2ogr",
		"-f", "GeoJSON",
		outputPath,
		inputPath,
		"-style_table", sldPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ogr2ogr error: %v\nOutput: %s", err, string(output))
	}

	log.Printf("Successfully processed with ogr2ogr: %s", string(output))
	return nil
}
