package sld

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDALCLIAdapter(t *testing.T) {
	// Skip this test if no GDAL installation is available
	_, err := os.Stat("/usr/bin/ogrinfo")
	if os.IsNotExist(err) {
		t.Skip("Skipping test: GDAL tools (ogrinfo) not found")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gdal-cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test SLD file with a blue fill
	blueSLD := `<?xml version="1.0" encoding="UTF-8"?>
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
</StyledLayerDescriptor>`

	sldPath := filepath.Join(tempDir, "blue_test.sld")
	if err := os.WriteFile(sldPath, []byte(blueSLD), 0644); err != nil {
		t.Fatalf("Failed to write test SLD: %v", err)
	}

	// Create the parser with CLI mode enabled
	parser := NewGDALParser(tempDir, true)

	// Process the SLD
	params, err := parser.ProcessStyles("blue_test.sld", "cities")
	if err != nil {
		t.Fatalf("ProcessStyles() error = %v", err)
	}

	// The params should include the layer name
	layerNameFound := false
	for i := 0; i < len(params)-1; i++ {
		if params[i] == "-l" && params[i+1] == "cities" {
			layerNameFound = true
			break
		}
	}

	if !layerNameFound {
		t.Errorf("ProcessStyles() did not include layer name in parameters: %v", params)
	}
}
