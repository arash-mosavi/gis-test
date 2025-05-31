package sld

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParserComparison(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-comparison-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test SLD files with different colors
	testCases := []struct {
		name     string
		sldFile  string
		content  string
		oldColor string
		newColor string
	}{
		{
			name:    "Red Style",
			sldFile: "red_test.sld",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#FF0000</CssParameter>
            </Fill>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`,
			oldColor: "255", // R component of RGB
			newColor: "255", // R component of RGB
		},
		{
			name:    "Blue Style",
			sldFile: "blue_test.sld",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#0000FF</CssParameter>
            </Fill>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`,
			oldColor: "255", // Default white in old parser
			newColor: "255", // B component of RGB in new parser
		},
	}

	// Create the old and new parsers
	oldParser := NewParser(tempDir)
	newParser := NewGDALParser(tempDir, false)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write the test SLD file
			sldPath := filepath.Join(tempDir, tc.sldFile)
			if err := os.WriteFile(sldPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write test SLD: %v", err)
			}

			// Process with old parser
			oldParams, err := oldParser.ProcessStyles(tc.sldFile, "cities")
			if err != nil {
				t.Fatalf("Old parser ProcessStyles() error = %v", err)
			}

			// Process with new parser
			newParams, err := newParser.ProcessStyles(tc.sldFile, "cities")
			if err != nil {
				t.Fatalf("New parser ProcessStyles() error = %v", err)
			}

			// Check if the old parser found the expected color
			oldFound := false
			for i := 0; i < len(oldParams); i++ {
				if i+1 < len(oldParams) && oldParams[i] == "-burn" && oldParams[i+1] == tc.oldColor {
					oldFound = true
					break
				}
			}

			// Check if the new parser found the expected color
			newFound := false
			for i := 0; i < len(newParams); i++ {
				if i+1 < len(newParams) && newParams[i] == "-burn" && newParams[i+1] == tc.newColor {
					newFound = true
					break
				}
			}

			if !oldFound {
				t.Errorf("Old parser did not find expected color component %s in %v", tc.oldColor, oldParams)
			}

			if !newFound {
				t.Errorf("New parser did not find expected color component %s in %v", tc.newColor, newParams)
			}

			// Check that both parsers included the layer name
			oldLayerFound := false
			newLayerFound := false

			for i := 0; i < len(oldParams)-1; i++ {
				if oldParams[i] == "-l" && oldParams[i+1] == "cities" {
					oldLayerFound = true
					break
				}
			}

			for i := 0; i < len(newParams)-1; i++ {
				if newParams[i] == "-l" && newParams[i+1] == "cities" {
					newLayerFound = true
					break
				}
			}

			if !oldLayerFound {
				t.Errorf("Old parser did not include layer name in parameters: %v", oldParams)
			}

			if !newLayerFound {
				t.Errorf("New parser did not include layer name in parameters: %v", newParams)
			}
		})
	}
}

func TestWithOpacityHandling(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-opacity-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an SLD file with opacity settings
	opacitySLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <Name>cities</Name>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <PolygonSymbolizer opacity="0.5">
            <Fill>
              <CssParameter name="fill">#FF0000</CssParameter>
              <CssParameter name="fill-opacity">0.7</CssParameter>
            </Fill>
            <Stroke>
              <CssParameter name="stroke">#000000</CssParameter>
              <CssParameter name="stroke-opacity">0.9</CssParameter>
            </Stroke>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	sldPath := filepath.Join(tempDir, "opacity_test.sld")
	if err := os.WriteFile(sldPath, []byte(opacitySLD), 0644); err != nil {
		t.Fatalf("Failed to write test SLD: %v", err)
	}

	// Only test with new parser since the old one doesn't handle opacity
	parser := NewGDALParser(tempDir, false)

	// Extract styling options directly for testing
	content, err := os.ReadFile(sldPath)
	if err != nil {
		t.Fatalf("Failed to read SLD file: %v", err)
	}

	options, err := parser.extractStyleOptionsFromXML(content)
	if err != nil {
		t.Fatalf("extractStyleOptionsFromXML() error = %v", err)
	}

	// Check if opacity is properly extracted (should be 0.7 from fill-opacity)
	if options.Opacity != "0.7" {
		t.Errorf("Expected opacity 0.7, got %s", options.Opacity)
	}

	// Check colors
	if options.FillColor != "#FF0000" {
		t.Errorf("Expected fill color #FF0000, got %s", options.FillColor)
	}

	if options.StrokeColor != "#000000" {
		t.Errorf("Expected stroke color #000000, got %s", options.StrokeColor)
	}
}
