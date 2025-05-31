package sld

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDALAdapterProcessStyles(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-adapter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test SLD file
	redSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0" 
    xmlns="http://www.opengis.net/sld" 
    xmlns:ogc="http://www.opengis.net/ogc">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <Title>Red Cities</Title>
      <FeatureTypeStyle>
        <Rule>
          <n>All Cities</n>
          <Title>All Cities Red Fill</Title>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#FF0000</CssParameter>
            </Fill>
            <Stroke>
              <CssParameter name="stroke">#000000</CssParameter>
              <CssParameter name="stroke-width">1</CssParameter>
            </Stroke>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	sldPath := filepath.Join(tempDir, "red_test.sld")
	err = os.WriteFile(sldPath, []byte(redSLD), 0644)
	if err != nil {
		t.Fatalf("Failed to write test SLD: %v", err)
	}

	// Create the adapter
	adapter := NewGDALAdapter(tempDir)
	adapter.TempDir = tempDir // Use the same temp dir for output

	// Test cases
	tests := []struct {
		name        string
		sldParam    string
		layerName   string
		expectError bool
	}{
		{
			name:        "Empty SLD Parameter",
			sldParam:    "",
			layerName:   "test_layer",
			expectError: false,
		},
		{
			name:        "Valid SLD File",
			sldParam:    "red_test.sld",
			layerName:   "cities",
			expectError: false,
		},
		{
			name:        "Non-existent SLD File",
			sldParam:    "non_existent.sld",
			layerName:   "test_layer",
			expectError: false, // Should not error, just use default white
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adapter.ProcessStyles(tc.sldParam, tc.layerName)

			// Check error handling
			if (err != nil) != tc.expectError {
				t.Errorf("ProcessStyles() error = %v, expectError %v", err, tc.expectError)
			}
		})
	}
}

func TestGDALAdapterGetSLDPath(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-adapter-path-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test SLD file
	testSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0"></StyledLayerDescriptor>`

	sldPath := filepath.Join(tempDir, "test.sld")
	err = os.WriteFile(sldPath, []byte(testSLD), 0644)
	if err != nil {
		t.Fatalf("Failed to write test SLD: %v", err)
	}

	// Create the adapter
	adapter := NewGDALAdapter(tempDir)
	adapter.TempDir = tempDir

	// Test cases
	tests := []struct {
		name        string
		sldParam    string
		expectError bool
		expectFile  bool // whether we expect a file path back
	}{
		{
			name:        "Local File",
			sldParam:    "test.sld",
			expectError: false,
			expectFile:  true,
		},
		{
			name:        "Non-existent File",
			sldParam:    "non_existent.sld",
			expectError: true,
			expectFile:  false,
		},
		{
			name:        "Inline XML",
			sldParam:    testSLD,
			expectError: false,
			expectFile:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath, err := adapter.getSLDPath(tc.sldParam)

			// Check error handling
			if (err != nil) != tc.expectError {
				t.Errorf("getSLDPath() error = %v, expectError %v", err, tc.expectError)
			}

			// Check file path result
			if tc.expectFile && filePath == "" {
				t.Errorf("getSLDPath() expected file path but got empty string")
			}

			// Check if file exists (when expected)
			if tc.expectFile && err == nil {
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("getSLDPath() returned path to non-existent file: %s", filePath)
				}
			}
		})
	}
}
