package sld

import (
	"testing"
)

func TestFixSLDContentDetailed(t *testing.T) {
	parser := NewGDALParser("", false)
	// Test with tag that needs to be replaced
	input := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <n>All Cities</n>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#0000FF</CssParameter>
            </Fill>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`)

	expected := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <Name>cities</Name>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <Name>All Cities</Name>
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

	result := parser.fixSLDContent(input)

	if string(result) != expected {
		t.Errorf("fixSLDContent() failed to replace <n> tags with <Name> tags.\nExpected:\n%s\n\nGot:\n%s", expected, string(result))
	}
}
