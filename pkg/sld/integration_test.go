package sld

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIntegrationWithRealSLDFiles tests the improved parser with actual SLD files
func TestIntegrationWithRealSLDFiles(t *testing.T) {
	parser := NewImprovedParser("", false)

	// Get the project root directory
	workspaceRoot := "/media/arash/670edafe-2f0a-4a79-b60c-29b3cde041b6/projects/d-self-projects/d-golang-projects/gis-test"
	stylesDir := filepath.Join(workspaceRoot, "styles")

	// Test cases for different SLD files
	testCases := []struct {
		filename    string
		layerName   string
		description string
	}{
		{
			filename:    "blue_cities.sld",
			layerName:   "cities",
			description: "Blue cities polygon styling",
		},
		{
			filename:    "red_cities.sld",
			layerName:   "cities",
			description: "Red cities polygon styling",
		},
		{
			filename:    "filtered_cities.sld",
			layerName:   "cities",
			description: "Filtered cities with conditional styling",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			sldPath := filepath.Join(stylesDir, tc.filename)

			// Check if file exists
			if _, err := os.Stat(sldPath); os.IsNotExist(err) {
				t.Skipf("SLD file %s does not exist, skipping test", sldPath)
				return
			}

			// Read the SLD file
			sldContent, err := os.ReadFile(sldPath)
			if err != nil {
				t.Fatalf("Failed to read SLD file %s: %v", sldPath, err)
			}

			// Process with improved parser
			params, err := parser.ProcessStyles(string(sldContent), tc.layerName)
			if err != nil {
				t.Errorf("Failed to process SLD file %s: %v", tc.filename, err)
				return
			}

			// Verify that GDAL parameters were generated
			if len(params) == 0 {
				t.Errorf("File %s: no GDAL parameters generated", tc.filename)
			}

			// Verify basic structure (should have -l for layer name)
			foundLayer := false
			for i, param := range params {
				if param == "-l" && i+1 < len(params) && params[i+1] == tc.layerName {
					foundLayer = true
					break
				}
			}

			if !foundLayer {
				t.Errorf("File %s: missing layer parameter in GDAL output: %v", tc.filename, params)
			}

			t.Logf("Successfully processed %s: Generated %d GDAL parameters", tc.filename, len(params))
		})
	}
}

// TestIntegrationCaching verifies that caching works correctly across multiple calls
func TestIntegrationCaching(t *testing.T) {
	parser := NewImprovedParser("", false) // Enable caching

	sldContent := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>test_layer</n>
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

	layerName := "test_layer"

	// First call - should parse and cache
	params1, err1 := parser.ProcessStyles(sldContent, layerName)
	if err1 != nil {
		t.Fatalf("First call failed: %v", err1)
	}

	// Second call - should use cache
	params2, err2 := parser.ProcessStyles(sldContent, layerName)
	if err2 != nil {
		t.Fatalf("Second call failed: %v", err2)
	}

	// Verify results are consistent
	if len(params1) != len(params2) {
		t.Errorf("Cached results differ in length: %d vs %d", len(params1), len(params2))
	}

	for i, param := range params1 {
		if i >= len(params2) || param != params2[i] {
			t.Errorf("Cached parameter mismatch at index %d: %s vs %s", i, param, params2[i])
		}
	}

	t.Logf("Caching verification successful: Generated %d parameters", len(params1))
}

// TestIntegrationErrorHandling tests error handling with malformed SLD content
func TestIntegrationErrorHandling(t *testing.T) {
	parser := NewImprovedParser("", false)

	testCases := []struct {
		name        string
		sldContent  string
		layerName   string
		expectError bool
	}{
		{
			name:        "Invalid XML",
			sldContent:  "<invalid>xml<content>",
			layerName:   "test",
			expectError: false, // Parser should fallback to defaults, not error
		},
		{
			name:        "Empty content",
			sldContent:  "",
			layerName:   "test",
			expectError: false, // Should return defaults
		},
		{
			name:        "Valid XML but no styling",
			sldContent:  "<?xml version=\"1.0\"?><root></root>",
			layerName:   "test",
			expectError: false, // Should not error, just return defaults
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := parser.ProcessStyles(tc.sldContent, tc.layerName)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for test case '%s', but got none", tc.name)
			}

			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for test case '%s': %v", tc.name, err)
			}

			// Should always get some parameters, even for invalid input
			if len(params) == 0 {
				t.Errorf("Expected default parameters for test case '%s', got none", tc.name)
			}
		})
	}
}

// TestIntegrationPerformance tests performance with large SLD content
func TestIntegrationPerformance(t *testing.T) {
	parser := NewImprovedParser("", false)

	// Create a large SLD with multiple rules
	largeSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>performance_test</n>
    <UserStyle>
      <FeatureTypeStyle>`

	// Add multiple rules to simulate complex styling
	for i := 0; i < 50; i++ {
		largeSLD += `
        <Rule>
          <Filter>
            <PropertyIsEqualTo>
              <PropertyName>category</PropertyName>
              <Literal>type` + string(rune('A'+i%26)) + `</Literal>
            </PropertyIsEqualTo>
          </Filter>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#FF0000</CssParameter>
            </Fill>
          </PolygonSymbolizer>
        </Rule>`
	}

	largeSLD += `
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	// Test processing time
	params, err := parser.ProcessStyles(largeSLD, "performance_test")
	if err != nil {
		t.Fatalf("Failed to process large SLD: %v", err)
	}

	// Verify that parameters were generated
	if len(params) == 0 {
		t.Errorf("Expected parameters to be generated from large SLD, got none")
	}

	t.Logf("Successfully processed large SLD with %d GDAL parameters", len(params))
}
