package sld

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// StyleInfo contains extracted style information
type StyleInfo struct {
	FillColor   string
	StrokeColor string
	StrokeWidth string
	Opacity     string
}

// Parser handles SLD parsing using string operations (no XML parsers)
type Parser struct {
	StylesDir string
}

// NewParser creates a new SLD parser
func NewParser(stylesDir string) *Parser {
	return &Parser{
		StylesDir: stylesDir,
	}
}

// ProcessStyles processes SLD styles and returns GDAL options
func (p *Parser) ProcessStyles(sldParam, layerName string) ([]string, error) {
	log.Printf("Processing SLD: '%s' for layer: '%s'", sldParam, layerName)

	s := strings.TrimSpace(sldParam)
	if s == "" {
		// Default white burn
		log.Println("No SLD parameter provided, using default white")
		return []string{"-burn", "255", "-l", layerName, "-at"}, nil
	}

	var sldContent []byte
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
	} else if strings.HasPrefix(s, "<?xml") {
		// Inline XML
		log.Println("Processing inline SLD XML")
		sldContent = []byte(s)
	} else {
		// Local file
		sldFile := filepath.Join(p.StylesDir, s)
		log.Printf("Reading SLD file: %s", sldFile)
		sldContent, err = os.ReadFile(sldFile)
		if err != nil {
			log.Printf("Failed to read SLD file: %v, falling back to filename color", err)
			// Fallback to filename-based color extraction
			return p.getColorFromFilename(s, layerName), nil
		}
	}

	log.Printf("Original SLD content (first 200 chars): %s", string(sldContent[:min(200, len(sldContent))]))

	// Fix common SLD issues before parsing
	originalContent := string(sldContent)
	sldContent = p.fixSLD(sldContent)
	fixedContent := string(sldContent)

	if originalContent != fixedContent {
		log.Printf("SLD was fixed - replaced <n> tags with <Name>")
		log.Printf("Fixed SLD content (first 200 chars): %s", fixedContent[:min(200, len(fixedContent))])
	} else {
		log.Printf("No SLD fixes needed")
	}

	// Extract colors using string operations
	styleInfo := p.extractStyleInfo(string(sldContent))
	log.Printf("Extracted style info: FillColor=%s, StrokeColor=%s, StrokeWidth=%s",
		styleInfo.FillColor, styleInfo.StrokeColor, styleInfo.StrokeWidth)

	// Convert to GDAL options
	options := p.styleInfoToGDALOptions(styleInfo, layerName)
	log.Printf("Generated GDAL options: %v", options)

	return options, nil
}

// fixSLD fixes common SLD issues using string replacement
func (p *Parser) fixSLD(content []byte) []byte {
	s := string(content)

	// Fix broken tags: replace <n> with <Name>
	s = strings.ReplaceAll(s, "<n>", "<Name>")
	s = strings.ReplaceAll(s, "</n>", "</Name>")

	return []byte(s)
}

// extractStyleInfo extracts style information using regex and string operations
func (p *Parser) extractStyleInfo(sldContent string) *StyleInfo {
	style := &StyleInfo{}

	// Extract fill color
	if fillColor := extractColorValue(sldContent, "fill"); fillColor != "" {
		style.FillColor = fillColor
		log.Printf("Found fill color: %s", fillColor)
	}

	// Extract stroke color
	if strokeColor := extractColorValue(sldContent, "stroke"); strokeColor != "" {
		style.StrokeColor = strokeColor
		log.Printf("Found stroke color: %s", strokeColor)
	}

	// Extract stroke width
	if strokeWidth := extractStrokeWidth(sldContent); strokeWidth != "" {
		style.StrokeWidth = strokeWidth
		log.Printf("Found stroke width: %s", strokeWidth)
	}

	return style
}

// extractColorValue extracts color value for a given parameter name
func extractColorValue(content, paramName string) string {
	// Look for CssParameter with name="paramName" and extract the color value
	patterns := []string{
		fmt.Sprintf(`<CssParameter\s+name="%s"[^>]*>([^<]+)</CssParameter>`, paramName),
		fmt.Sprintf(`<CssParameter\s+name='%s'[^>]*>([^<]+)</CssParameter>`, paramName),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			color := strings.TrimSpace(matches[1])
			// Ensure color starts with #
			if !strings.HasPrefix(color, "#") && len(color) == 6 {
				color = "#" + color
			}
			log.Printf("Extracted %s color: %s using pattern: %s", paramName, color, pattern)
			return color
		}
	}

	log.Printf("No %s color found in content", paramName)
	return ""
}

// extractStrokeWidth extracts stroke width value
func extractStrokeWidth(content string) string {
	patterns := []string{
		`<CssParameter\s+name="stroke-width"[^>]*>([^<]+)</CssParameter>`,
		`<CssParameter\s+name='stroke-width'[^>]*>([^<]+)</CssParameter>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// styleInfoToGDALOptions converts StyleInfo to GDAL options
func (p *Parser) styleInfoToGDALOptions(style *StyleInfo, layerName string) []string {
	options := []string{}
	if style.FillColor != "" {
		rgb, err := hexToRGB(style.FillColor)
		if err == nil {
			log.Printf("Converted %s to RGB: %d, %d, %d", style.FillColor, rgb[0], rgb[1], rgb[2])
			// Use multiple -burn flags for RGB output (removed -3d as it conflicts)
			options = append(options, "-burn", fmt.Sprintf("%d", rgb[0]))
			options = append(options, "-burn", fmt.Sprintf("%d", rgb[1]))
			options = append(options, "-burn", fmt.Sprintf("%d", rgb[2]))
		} else {
			log.Printf("Failed to convert hex color %s to RGB: %v", style.FillColor, err)
		}
	}

	if len(options) == 0 {
		// Default white burn if no color found
		log.Println("No fill color found, using default white")
		options = append(options, "-burn", "255")
	}

	// Add layer and quality options
	options = append(options, "-l", layerName, "-at")

	return options
}

// getColorFromFilename extracts color from filename as fallback
func (p *Parser) getColorFromFilename(filename, layerName string) []string {
	filename = strings.ToLower(filename)
	log.Printf("Using filename-based color extraction for: %s", filename)

	var rgb [3]int

	// Color mapping based on filename
	if strings.Contains(filename, "red") {
		rgb = [3]int{255, 0, 0}
		log.Println("Filename contains 'red', using red color")
	} else if strings.Contains(filename, "blue") {
		rgb = [3]int{0, 0, 255}
		log.Println("Filename contains 'blue', using blue color")
	} else if strings.Contains(filename, "green") {
		rgb = [3]int{0, 255, 0}
		log.Println("Filename contains 'green', using green color")
	} else if strings.Contains(filename, "yellow") {
		rgb = [3]int{255, 255, 0}
		log.Println("Filename contains 'yellow', using yellow color")
	} else if strings.Contains(filename, "purple") {
		rgb = [3]int{128, 0, 128}
		log.Println("Filename contains 'purple', using purple color")
	} else if strings.Contains(filename, "orange") {
		rgb = [3]int{255, 165, 0}
		log.Println("Filename contains 'orange', using orange color")
	} else {
		// Default white
		rgb = [3]int{255, 255, 255}
		log.Println("No color found in filename, using default white")
	}

	return []string{
		"-burn", fmt.Sprintf("%d", rgb[0]),
		"-burn", fmt.Sprintf("%d", rgb[1]),
		"-burn", fmt.Sprintf("%d", rgb[2]),
		"-3d",
		"-l", layerName,
		"-at",
	}
}

// hexToRGB converts hex color to RGB values
func hexToRGB(hex string) ([3]int, error) {
	if !strings.HasPrefix(hex, "#") {
		return [3]int{}, fmt.Errorf("invalid hex color format")
	}

	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return [3]int{}, fmt.Errorf("invalid hex color length")
	}

	r, err := strconv.ParseInt(hex[0:2], 16, 0)
	if err != nil {
		return [3]int{}, err
	}

	g, err := strconv.ParseInt(hex[2:4], 16, 0)
	if err != nil {
		return [3]int{}, err
	}

	b, err := strconv.ParseInt(hex[4:6], 16, 0)
	if err != nil {
		return [3]int{}, err
	}

	return [3]int{int(r), int(g), int(b)}, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
