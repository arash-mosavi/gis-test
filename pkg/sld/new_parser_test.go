package sld

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDALParserProcessStyles(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test SLD file
	blueSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0" 
    xmlns="http://www.opengis.net/sld" 
    xmlns:ogc="http://www.opengis.net/ogc" 
    xmlns:xlink="http://www.w3.org/1999/xlink" 
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <Title>Blue Cities</Title>
      <FeatureTypeStyle>
        <Rule>
          <n>All Cities</n>
          <Title>All Cities Blue Fill</Title>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#0000FF</CssParameter>
            </Fill>
            <Stroke>
              <CssParameter name="stroke">#FFFFFF</CssParameter>
              <CssParameter name="stroke-width">2</CssParameter>
            </Stroke>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	sldPath := filepath.Join(tempDir, "blue_test.sld")
	err = os.WriteFile(sldPath, []byte(blueSLD), 0644)
	if err != nil {
		t.Fatalf("Failed to write test SLD: %v", err)
	}

	// Create the parser
	parser := NewGDALParser(tempDir, false)

	// Test cases
	tests := []struct {
		name         string
		sldParam     string
		layerName    string
		expectColor  string
		expectError  bool
		expectParams []string
	}{
		{
			name:        "Empty SLD Parameter",
			sldParam:    "",
			layerName:   "test_layer",
			expectColor: "255",
			expectError: false,
			expectParams: []string{
				"-burn", "255", "-l", "test_layer", "-at",
			},
		},
		{
			name:        "Valid SLD File",
			sldParam:    "blue_test.sld",
			layerName:   "cities",
			expectColor: "0",
			expectError: false,
			// We expect RGB values in the parameters
			expectParams: []string{
				"-burn", "0", "-burn", "0", "-burn", "255", "-3d", "-l", "cities", "-at",
			},
		},
		{
			name:        "Non-existent SLD File",
			sldParam:    "non_existent.sld",
			layerName:   "test_layer",
			expectColor: "255",
			expectError: false,
			expectParams: []string{
				"-burn", "255", "-l", "test_layer", "-at",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params, err := parser.ProcessStyles(tc.sldParam, tc.layerName)

			// Check error
			if (err != nil) != tc.expectError {
				t.Errorf("ProcessStyles() error = %v, expectError %v", err, tc.expectError)
				return
			}

			// Check parameters
			if len(params) != len(tc.expectParams) {
				t.Errorf("ProcessStyles() parameters count mismatch: got %d, want %d", len(params), len(tc.expectParams))
				t.Logf("Got params: %v", params)
				t.Logf("Expected params: %v", tc.expectParams)
				return
			}

			// Check specific parameter values
			containsExpectedColor := false
			for i := 0; i < len(params); i++ {
				if i+1 < len(params) && params[i] == "-burn" && params[i+1] == tc.expectColor {
					containsExpectedColor = true
					break
				}
			}

			if !containsExpectedColor && tc.expectColor != "" {
				t.Errorf("ProcessStyles() missing expected color %s in parameters: %v", tc.expectColor, params)
			}
		})
	}
}

func TestGDALParserWithInlineXML(t *testing.T) {
	// Create a parser with no styles directory
	parser := NewGDALParser("", false)

	// Inline XML SLD content
	inlineXML := `<?xml version="1.0" encoding="UTF-8"?>
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
</StyledLayerDescriptor>`

	// Process the inline XML
	params, err := parser.ProcessStyles(inlineXML, "cities")

	// Check for errors
	if err != nil {
		t.Errorf("ProcessStyles() with inline XML error = %v", err)
		return
	}

	// Check that we got the expected red color (255,0,0)
	found := false
	for i := 0; i < len(params)-2; i++ {
		if params[i] == "-burn" && params[i+1] == "255" &&
			i+3 < len(params) && params[i+2] == "-burn" && params[i+3] == "0" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("ProcessStyles() with inline XML did not produce expected red color. Got: %v", params)
	}
}

func TestHexColorToRGBA(t *testing.T) {
	tests := []struct {
		name      string
		hex       string
		wantR     int
		wantG     int
		wantB     int
		wantA     int
		wantError bool
	}{
		{
			name:      "Valid Blue",
			hex:       "#0000FF",
			wantR:     0,
			wantG:     0,
			wantB:     255,
			wantA:     255,
			wantError: false,
		},
		{
			name:      "Valid Red",
			hex:       "#FF0000",
			wantR:     255,
			wantG:     0,
			wantB:     0,
			wantA:     255,
			wantError: false,
		},
		{
			name:      "Without Hash",
			hex:       "00FF00",
			wantR:     0,
			wantG:     255,
			wantB:     0,
			wantA:     255,
			wantError: false,
		},
		{
			name:      "Invalid Format",
			hex:       "#XYZ",
			wantR:     0,
			wantG:     0,
			wantB:     0,
			wantA:     0,
			wantError: true,
		},
		{
			name:      "Too Short",
			hex:       "#123",
			wantR:     0,
			wantG:     0,
			wantB:     0,
			wantA:     0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, a, err := hexColorToRGBA(tt.hex)

			if (err != nil) != tt.wantError {
				t.Errorf("hexColorToRGBA() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if r != tt.wantR || g != tt.wantG || b != tt.wantB || a != tt.wantA {
					t.Errorf("hexColorToRGBA() = (%v,%v,%v,%v), want (%v,%v,%v,%v)",
						r, g, b, a, tt.wantR, tt.wantG, tt.wantB, tt.wantA)
				}
			}
		})
	}
}
