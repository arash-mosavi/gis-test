package sld

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDALParserWithCache(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sld-cache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test SLD file
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

	// Create the parser
	parser := NewGDALParser(tempDir, false)

	// Test caching functionality
	t.Run("Cache Operations", func(t *testing.T) {
		// First run should parse the SLD
		params1, err := parser.ProcessStyles("blue_test.sld", "cities")
		if err != nil {
			t.Fatalf("ProcessStyles() error = %v", err)
		}

		// Second run should use cache
		params2, err := parser.ProcessStyles("blue_test.sld", "cities")
		if err != nil {
			t.Fatalf("ProcessStyles() error = %v", err)
		}

		// Parameters should be identical
		if len(params1) != len(params2) {
			t.Errorf("Cached parameters length mismatch: %v vs %v", params1, params2)
		}

		// Clear cache
		parser.ClearCache()

		// Disable cache
		parser.SetCacheOptions(false, 0)

		// Run again - should parse again
		_, err = parser.ProcessStyles("blue_test.sld", "cities")
		if err != nil {
			t.Fatalf("ProcessStyles() error = %v", err)
		}
	})

	// Test non-standard tag handling
	t.Run("Non-standard Tag Handling", func(t *testing.T) {
		// Create SLD with non-standard tags
		nonStandardSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cities</n>  <!-- Non-standard tag should be converted to <Name> -->
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <n>Rule1</n>  <!-- Non-standard tag should be converted to <Name> -->
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

		sldPath := filepath.Join(tempDir, "nonstandard_test.sld")
		if err := os.WriteFile(sldPath, []byte(nonStandardSLD), 0644); err != nil {
			t.Fatalf("Failed to write test SLD: %v", err)
		}

		// Process the SLD
		params, err := parser.ProcessStyles("nonstandard_test.sld", "cities")
		if err != nil {
			t.Fatalf("ProcessStyles() error = %v", err)
		}

		// Check that it was processed correctly (should have red color: 255,0,0)
		foundRed := false
		for i := 0; i < len(params)-2; i++ {
			if params[i] == "-burn" && params[i+1] == "255" &&
				i+3 < len(params) && params[i+2] == "-burn" && params[i+3] == "0" {
				foundRed = true
				break
			}
		}

		if !foundRed {
			t.Errorf("Non-standard tag handling failed; did not get expected color: %v", params)
		}
	})
}
