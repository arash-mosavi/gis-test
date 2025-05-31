package sld

import (
	"testing"
)

func BenchmarkImprovedParserPolygon(b *testing.B) {
	parser := NewImprovedParser("", false)
	polygonSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cities</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <PolygonSymbolizer>
            <Fill>
              <CssParameter name="fill">#0000FF</CssParameter>
              <CssParameter name="fill-opacity">0.7</CssParameter>
            </Fill>
            <Stroke>
              <CssParameter name="stroke">#000000</CssParameter>
              <CssParameter name="stroke-width">2</CssParameter>
            </Stroke>
          </PolygonSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ProcessStyles(polygonSLD, "cities")
		if err != nil {
			b.Fatalf("ProcessStyles failed: %v", err)
		}
	}
}

func BenchmarkImprovedParserLine(b *testing.B) {
	parser := NewImprovedParser("", false)
	lineSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>roads</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <LineSymbolizer>
            <Stroke>
              <CssParameter name="stroke">#FF0000</CssParameter>
              <CssParameter name="stroke-width">3</CssParameter>
              <CssParameter name="stroke-dasharray">5 2</CssParameter>
            </Stroke>
          </LineSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ProcessStyles(lineSLD, "roads")
		if err != nil {
			b.Fatalf("ProcessStyles failed: %v", err)
		}
	}
}

func BenchmarkImprovedParserConditional(b *testing.B) {
	parser := NewImprovedParser("", false)
	conditionalSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>cinema</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <Filter>
            <PropertyIsEqualTo>
              <PropertyName>category</PropertyName>
              <Literal>multiplex</Literal>
            </PropertyIsEqualTo>
          </Filter>
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ProcessStyles(conditionalSLD, "cinema")
		if err != nil {
			b.Fatalf("ProcessStyles failed: %v", err)
		}
	}
}
