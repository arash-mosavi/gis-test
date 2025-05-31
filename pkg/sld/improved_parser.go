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

// ImprovedStylingOptions extends styling parameters for more SLD types
type ImprovedStylingOptions struct {
	FillColor      string
	StrokeColor    string
	StrokeWidth    string
	Opacity        string
	ShapeType      string // "point", "line", "polygon", etc.
	PointSize      string
	PointSymbol    string // circle, square, triangle, etc.
	LineStyle      string // solid, dashed, dotted, etc.
	Conditions     []StyleCondition
	Label          string
	LabelFont      string
	LabelSize      string
	LabelPlacement string
}

// StyleCondition represents a conditional styling rule
type StyleCondition struct {
	Attribute   string
	Operator    string // =, !=, <, >, etc.
	Value       string
	FillColor   string
	StrokeColor string
	Opacity     string
}

// improvedCacheEntry represents a cached styling result
type improvedCacheEntry struct {
	options   *ImprovedStylingOptions
	timestamp time.Time
	params    []string
}

// ImprovedParser handles SLD parsing with support for more style types
type ImprovedParser struct {
	StylesDir    string
	UseCLI       bool // Whether to use command-line GDAL tools
	cacheEnabled bool
	cacheTimeout time.Duration // How long to keep cache entries
	cache        map[string]improvedCacheEntry
	cacheMutex   sync.RWMutex
}

// NewImprovedParser creates a new SLD parser with enhanced features
func NewImprovedParser(stylesDir string, useCLI bool) *ImprovedParser {
	return &ImprovedParser{
		StylesDir:    stylesDir,
		UseCLI:       useCLI,
		cacheEnabled: true,
		cacheTimeout: 5 * time.Minute, // Default 5 minutes cache timeout
		cache:        make(map[string]improvedCacheEntry),
	}
}

// SetCacheOptions configures the caching behavior
func (p *ImprovedParser) SetCacheOptions(enabled bool, timeout time.Duration) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.cacheEnabled = enabled
	if timeout > 0 {
		p.cacheTimeout = timeout
	}
}

// ClearCache removes all cached entries
func (p *ImprovedParser) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.cache = make(map[string]improvedCacheEntry)
	log.Println("XML parser cache cleared")
}

// ProcessStyles processes SLD styles and returns GDAL options
func (p *ImprovedParser) ProcessStyles(sldParam, layerName string) ([]string, error) {
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

// fixSLDContent fixes common issues with SLD files
func (p *ImprovedParser) fixSLDContent(content []byte) []byte {
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

// checkCache checks if a valid cached result exists for the given key
func (p *ImprovedParser) checkCache(key string) ([]string, bool) {
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
func (p *ImprovedParser) cacheResult(key string, options *ImprovedStylingOptions, params []string) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	p.cache[key] = improvedCacheEntry{
		options:   options,
		params:    params,
		timestamp: time.Now(),
	}

	log.Printf("Cached SLD processing result for '%s'", key)
}

// processWithGDALCLI uses GDAL command line tools to process the SLD
func (p *ImprovedParser) processWithGDALCLI(sldPath, layerName string) ([]string, error) {
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
	outputStr := string(output)
	params := []string{}

	// Look for color information
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
func (p *ImprovedParser) extractStyleOptionsFromXML(content []byte) (*ImprovedStylingOptions, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(content); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	options := &ImprovedStylingOptions{}

	// Find all CssParameter elements with appropriate names
	root := doc.SelectElement("StyledLayerDescriptor")
	if root == nil {
		return nil, fmt.Errorf("missing StyledLayerDescriptor element")
	}

	// Navigate through the XML structure to find styling parameters
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

	// Process all feature type styles
	for _, featureTypeStyle := range featureTypeStyles {
		rules := featureTypeStyle.FindElements("//Rule")
		if len(rules) == 0 {
			continue // Skip if no rules
		}

		// Process all rules
		for _, rule := range rules {
			// Check for conditional rule (OGC Filter)
			filterElement := rule.FindElement(".//Filter")
			if filterElement != nil {
				styleCondition, err := p.extractCondition(filterElement)
				if err == nil && styleCondition.Attribute != "" {
					options.Conditions = append(options.Conditions, *styleCondition)
					log.Printf("Found style condition on attribute %s", styleCondition.Attribute)
				}
			}

			// Check for <TextSymbolizer> for labels
			if textSymbolizer := rule.FindElement(".//TextSymbolizer"); textSymbolizer != nil {
				if label := textSymbolizer.FindElement(".//Label"); label != nil {
					options.Label = label.Text()
					log.Printf("Found label: %s", options.Label)
				}

				if font := textSymbolizer.FindElement(".//Font"); font != nil {
					for _, param := range font.FindElements(".//CssParameter") {
						if name := param.SelectAttrValue("name", ""); name == "font-family" {
							options.LabelFont = param.Text()
						} else if name == "font-size" {
							options.LabelSize = param.Text()
						}
					}
				}

				if placement := textSymbolizer.FindElement(".//LabelPlacement"); placement != nil {
					options.LabelPlacement = "point" // Default
					if placement.FindElement(".//LinePlacement") != nil {
						options.LabelPlacement = "line"
					} else if placement.FindElement(".//PointPlacement") != nil {
						options.LabelPlacement = "point"
					}
				}
			}

			// Process different types of symbolizers

			// First check for PolygonSymbolizer
			if symbolizer := rule.FindElement(".//PolygonSymbolizer"); symbolizer != nil {
				options.ShapeType = "polygon"
				p.extractSymbolizerOptions(symbolizer, options)
			}

			// Check for LineSymbolizer
			if symbolizer := rule.FindElement(".//LineSymbolizer"); symbolizer != nil {
				options.ShapeType = "line"
				p.extractSymbolizerOptions(symbolizer, options)

				// Extract line style
				if stroke := symbolizer.FindElement(".//Stroke"); stroke != nil {
					for _, param := range stroke.FindElements(".//CssParameter") {
						if name := param.SelectAttrValue("name", ""); name == "stroke-dasharray" {
							options.LineStyle = "dashed"
							log.Printf("Found line style: dashed")
						} else if name == "stroke-dashoffset" {
							options.LineStyle = "dashed"
							log.Printf("Found line style: dashed")
						}
					}
				}

				// If no explicit style specified, default to solid
				if options.LineStyle == "" {
					options.LineStyle = "solid"
					log.Printf("Using default line style: solid")
				}
			}

			// Check for PointSymbolizer
			if symbolizer := rule.FindElement(".//PointSymbolizer"); symbolizer != nil {
				options.ShapeType = "point"
				p.extractSymbolizerOptions(symbolizer, options)

				// Extract point specifics
				if graphic := symbolizer.FindElement(".//Graphic"); graphic != nil {
					if size := graphic.FindElement(".//Size"); size != nil {
						options.PointSize = size.Text()
						log.Printf("Found point size: %s", options.PointSize)
					}

					// Try to determine symbol type
					if mark := graphic.FindElement(".//Mark"); mark != nil {
						if wellKnownName := mark.FindElement(".//WellKnownName"); wellKnownName != nil {
							options.PointSymbol = wellKnownName.Text()
							log.Printf("Found point symbol: %s", options.PointSymbol)
						}
					} else if externalGraphic := graphic.FindElement(".//ExternalGraphic"); externalGraphic != nil {
						options.PointSymbol = "icon"
						log.Printf("Found point symbol: icon (external graphic)")
					}
				}
			}

			// If we found styling info, we can stop processing
			if options.FillColor != "" || options.StrokeColor != "" {
				return options, nil
			}
		}
	}

	// If no styling found after checking all rules, return an error
	return nil, fmt.Errorf("no valid styling found in SLD")
}

// extractSymbolizerOptions extracts common styling options from any symbolizer
func (p *ImprovedParser) extractSymbolizerOptions(symbolizer *etree.Element, options *ImprovedStylingOptions) {
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
}

// extractCondition parses an OGC Filter element to extract style conditions
func (p *ImprovedParser) extractCondition(filterElement *etree.Element) (*StyleCondition, error) {
	condition := &StyleCondition{}

	// Check for PropertyIsEqualTo
	if equalTo := filterElement.FindElement(".//PropertyIsEqualTo"); equalTo != nil {
		condition.Operator = "="
		if propertyName := equalTo.FindElement(".//PropertyName"); propertyName != nil {
			condition.Attribute = propertyName.Text()
		}
		if literal := equalTo.FindElement(".//Literal"); literal != nil {
			condition.Value = literal.Text()
		}
	}

	// Check for PropertyIsNotEqualTo
	if notEqual := filterElement.FindElement(".//PropertyIsNotEqualTo"); notEqual != nil {
		condition.Operator = "!="
		if propertyName := notEqual.FindElement(".//PropertyName"); propertyName != nil {
			condition.Attribute = propertyName.Text()
		}
		if literal := notEqual.FindElement(".//Literal"); literal != nil {
			condition.Value = literal.Text()
		}
	}

	// Check for PropertyIsGreaterThan
	if greaterThan := filterElement.FindElement(".//PropertyIsGreaterThan"); greaterThan != nil {
		condition.Operator = ">"
		if propertyName := greaterThan.FindElement(".//PropertyName"); propertyName != nil {
			condition.Attribute = propertyName.Text()
		}
		if literal := greaterThan.FindElement(".//Literal"); literal != nil {
			condition.Value = literal.Text()
		}
	}

	// Check for PropertyIsLessThan
	if lessThan := filterElement.FindElement(".//PropertyIsLessThan"); lessThan != nil {
		condition.Operator = "<"
		if propertyName := lessThan.FindElement(".//PropertyName"); propertyName != nil {
			condition.Attribute = propertyName.Text()
		}
		if literal := lessThan.FindElement(".//Literal"); literal != nil {
			condition.Value = literal.Text()
		}
	}

	// If no condition found or incomplete information
	if condition.Attribute == "" || condition.Operator == "" || condition.Value == "" {
		return nil, fmt.Errorf("invalid or incomplete filter condition")
	}

	return condition, nil
}

// styleOptionsToGDALParams converts styling options to GDAL command line parameters
func (p *ImprovedParser) styleOptionsToGDALParams(options *ImprovedStylingOptions, layerName string) []string {
	result := []string{}

	// Handle fill color for rasterization
	if options.FillColor != "" && strings.HasPrefix(options.FillColor, "#") {
		// Convert hex color to RGB values
		r, g, b, _, err := hexColorToRGBA(options.FillColor)
		if err == nil {
			// Use RGB burn values (removed -3d as it conflicts with multiple -burn parameters)
			result = append(result,
				"-burn", fmt.Sprintf("%d", r),
				"-burn", fmt.Sprintf("%d", g),
				"-burn", fmt.Sprintf("%d", b))
			log.Printf("Using RGB values: %d,%d,%d", r, g, b)
		} else {
			log.Printf("Failed to parse color '%s': %v", options.FillColor, err)
			// Default to white
			result = append(result, "-burn", "255")
		}
	} else {
		// Default to white
		result = append(result, "-burn", "255")
	}

	// Handle special options based on shape type
	if options.ShapeType == "line" {
		// Add line specific options
		if options.StrokeWidth != "" {
			result = append(result, "-burn-width", options.StrokeWidth)
		}
	} else if options.ShapeType == "point" {
		// Add point specific options
		if options.PointSize != "" {
			result = append(result, "-point-size", options.PointSize)
		}
	}

	// Add layer name and other fixed parameters
	result = append(result, "-l", layerName, "-at")

	// Add label if present
	if options.Label != "" {
		result = append(result, "-label", options.Label)

		if options.LabelFont != "" {
			result = append(result, "-label-font", options.LabelFont)
		}

		if options.LabelSize != "" {
			result = append(result, "-label-size", options.LabelSize)
		}
	}

	return result
}

// ApplySLDToGeoJson applies an SLD styling to a GeoJSON dataset using GDAL
func (p *ImprovedParser) ApplySLDToGeoJson(sldPath, geoJsonPath, outputPath string) error {
	// First fix the SLD content
	sldContent, err := os.ReadFile(sldPath)
	if err != nil {
		return fmt.Errorf("failed to read SLD file: %v", err)
	}

	fixedContent := p.fixSLDContent(sldContent)

	// Save fixed content to a temporary file
	tmpFile, err := os.CreateTemp("", "fixed-sld-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(fixedContent); err != nil {
		return fmt.Errorf("failed to write fixed SLD to temp file: %v", err)
	}
	tmpFile.Close()

	// Use ogr2ogr to apply styling
	cmd := exec.Command(
		"ogr2ogr",
		"-f", "GeoJSON",
		outputPath,
		geoJsonPath,
		"-style_table", tmpFile.Name(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ogr2ogr error: %v - Output: %s", err, string(output))
	}

	log.Printf("Successfully applied SLD to GeoJSON: %s", string(output))
	return nil
}

// ProcessWithQgisProcess processes an SLD file using the qgis_process tool with advanced options
func (p *ImprovedParser) ProcessWithQgisProcess(sldPath, layerPath, outputPath string) error {
	// Extract SLD options first
	sldContent, err := os.ReadFile(sldPath)
	if err != nil {
		return fmt.Errorf("failed to read SLD file: %v", err)
	}

	fixedContent := p.fixSLDContent(sldContent)
	options, err := p.extractStyleOptionsFromXML(fixedContent)
	if err != nil {
		return fmt.Errorf("failed to extract styling options: %v", err)
	}

	// Build QGIS process command based on shape type
	args := []string{"run"}

	switch options.ShapeType {
	case "polygon":
		args = append(args, "native:rasterize")
	case "line":
		args = append(args, "native:rasterizelinesbywidth")
	case "point":
		args = append(args, "native:rasterizepoints")
	default:
		args = append(args, "native:rasterize") // Default to polygon
	}

	// Add common parameters
	args = append(args,
		"--INPUT="+layerPath,
		"--OUTPUT="+outputPath,
	)

	// Add type-specific parameters
	if options.ShapeType == "line" && options.StrokeWidth != "" {
		args = append(args, "--WIDTH="+options.StrokeWidth)
	} else if options.ShapeType == "point" && options.PointSize != "" {
		args = append(args, "--SIZE="+options.PointSize)
	}

	// Handle conditions if present
	if len(options.Conditions) > 0 {
		condition := options.Conditions[0] // Use first condition
		args = append(args, "--EXPRESSION="+condition.Attribute+condition.Operator+condition.Value)
	}

	cmd := exec.Command("qgis_process", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qgis_process error: %v - Output: %s", err, string(output))
	}

	log.Printf("Successfully processed with qgis_process: %s", string(output))
	return nil
}
