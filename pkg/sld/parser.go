package sld

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Parser handles SLD parsing and processing
type Parser struct {
	StylesDir string // Directory containing SLD files
}

// NewParser creates a new SLD parser with the specified styles directory
func NewParser(stylesDir string) *Parser {
	return &Parser{
		StylesDir: stylesDir,
	}
}

// ProcessStyles returns GDAL Rasterize switches based on STYLES param
// supports empty (default burn), filename (in StylesDir), URL or inline XML
func (p *Parser) ProcessStyles(sldParam, layerName string) ([]string, error) {
	s := strings.TrimSpace(sldParam)
	if s == "" {
		// default white burn
		return []string{"-burn", "255", "-l", layerName}, nil
	}
	var sldFile string
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		// download URL
		resp, err := http.Get(s)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch SLD URL: %v", err)
		}
		defer resp.Body.Close()
		tmp, err := os.CreateTemp("", "sld-*.xml")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp SLD: %v", err)
		}
		defer tmp.Close()
		if _, err := io.Copy(tmp, resp.Body); err != nil {
			return nil, fmt.Errorf("failed to write temp SLD: %v", err)
		}
		sldFile = tmp.Name()
	} else if strings.HasPrefix(s, "<?xml") {
		// inline XML
		tmp, err := os.CreateTemp("", "sld-*.xml")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp SLD: %v", err)
		}
		defer tmp.Close()
		if _, err := tmp.WriteString(s); err != nil {
			return nil, fmt.Errorf("failed to write temp SLD: %v", err)
		}
		sldFile = tmp.Name()
	} else {
		// local file in directory
		sldFile = filepath.Join(p.StylesDir, s)
		if _, err := os.Stat(sldFile); err != nil {
			return nil, fmt.Errorf("SLD file not found: %s", sldFile)
		}
	}
	// return -sld switch
	opts := []string{"-sld", sldFile, "-l", layerName}
	return opts, nil
}
