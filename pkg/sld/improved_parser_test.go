package sld

import (
	"strings"
	"testing"
)

func TestImprovedParserShapeTypes(t *testing.T) {
	parser := NewImprovedParser("", false)

	// Test polygon SLD
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

	fixedContent := parser.fixSLDContent([]byte(polygonSLD))
	options, err := parser.extractStyleOptionsFromXML(fixedContent)

	if err != nil {
		t.Errorf("Failed to parse polygon SLD: %v", err)
	}

	if options.ShapeType != "polygon" {
		t.Errorf("Expected shape type 'polygon', got '%s'", options.ShapeType)
	}

	if options.FillColor != "#0000FF" {
		t.Errorf("Expected fill color '#0000FF', got '%s'", options.FillColor)
	}

	if options.StrokeColor != "#000000" {
		t.Errorf("Expected stroke color '#000000', got '%s'", options.StrokeColor)
	}

	if options.StrokeWidth != "2" {
		t.Errorf("Expected stroke width '2', got '%s'", options.StrokeWidth)
	}

	if options.Opacity != "0.7" {
		t.Errorf("Expected opacity '0.7', got '%s'", options.Opacity)
	}

	// Test line SLD
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

	fixedContent = parser.fixSLDContent([]byte(lineSLD))
	options, err = parser.extractStyleOptionsFromXML(fixedContent)

	if err != nil {
		t.Errorf("Failed to parse line SLD: %v", err)
	}

	if options.ShapeType != "line" {
		t.Errorf("Expected shape type 'line', got '%s'", options.ShapeType)
	}

	if options.StrokeColor != "#FF0000" {
		t.Errorf("Expected stroke color '#FF0000', got '%s'", options.StrokeColor)
	}

	if options.StrokeWidth != "3" {
		t.Errorf("Expected stroke width '3', got '%s'", options.StrokeWidth)
	}

	if options.LineStyle != "dashed" {
		t.Errorf("Expected line style 'dashed', got '%s'", options.LineStyle)
	}

	// Test point SLD
	pointSLD := `<?xml version="1.0" encoding="UTF-8"?>
<StyledLayerDescriptor version="1.0.0">
  <NamedLayer>
    <n>points</n>
    <UserStyle>
      <FeatureTypeStyle>
        <Rule>
          <PointSymbolizer>
            <Graphic>
              <Mark>
                <WellKnownName>circle</WellKnownName>
                <Fill>
                  <CssParameter name="fill">#00FF00</CssParameter>
                </Fill>
              </Mark>
              <Size>8</Size>
            </Graphic>
          </PointSymbolizer>
        </Rule>
      </FeatureTypeStyle>
    </UserStyle>
  </NamedLayer>
</StyledLayerDescriptor>`

	fixedContent = parser.fixSLDContent([]byte(pointSLD))
	options, err = parser.extractStyleOptionsFromXML(fixedContent)

	if err != nil {
		t.Errorf("Failed to parse point SLD: %v", err)
	}

	if options.ShapeType != "point" {
		t.Errorf("Expected shape type 'point', got '%s'", options.ShapeType)
	}

	if options.FillColor != "#00FF00" {
		t.Errorf("Expected fill color '#00FF00', got '%s'", options.FillColor)
	}

	if options.PointSymbol != "circle" {
		t.Errorf("Expected point symbol 'circle', got '%s'", options.PointSymbol)
	}

	if options.PointSize != "8" {
		t.Errorf("Expected point size '8', got '%s'", options.PointSize)
	}
}

func TestImprovedParserConditionalStyling(t *testing.T) {
	parser := NewImprovedParser("", false)

	// Test conditional SLD (cinema example)
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
        <Rule>
          <Filter>
            <PropertyIsEqualTo>
              <PropertyName>category</PropertyName>
              <Literal>indie</Literal>
            </PropertyIsEqualTo>
          </Filter>
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

	fixedContent := parser.fixSLDContent([]byte(conditionalSLD))
	options, err := parser.extractStyleOptionsFromXML(fixedContent)

	if err != nil {
		t.Errorf("Failed to parse conditional SLD: %v", err)
	}

	if options.ShapeType != "polygon" {
		t.Errorf("Expected shape type 'polygon', got '%s'", options.ShapeType)
	}

	if len(options.Conditions) == 0 {
		t.Errorf("Expected at least one condition, found none")
	} else {
		condition := options.Conditions[0]
		if condition.Attribute != "category" {
			t.Errorf("Expected condition attribute 'category', got '%s'", condition.Attribute)
		}

		if condition.Operator != "=" {
			t.Errorf("Expected condition operator '=', got '%s'", condition.Operator)
		}

		if condition.Value != "multiplex" && condition.Value != "indie" {
			t.Errorf("Expected condition value 'multiplex' or 'indie', got '%s'", condition.Value)
		}
	}
}

func TestFixSLDContentImproved(t *testing.T) {
	parser := NewImprovedParser("", false)

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

	result := parser.fixSLDContent(input)
	resultStr := string(result)

	if !strings.Contains(resultStr, "<Name>cities</Name>") {
		t.Errorf("fixSLDContent() failed to replace <n> tags with <Name> tags, expected to find <Name>cities</Name> but didn't")
	}

	if !strings.Contains(resultStr, "<Name>All Cities</Name>") {
		t.Errorf("fixSLDContent() failed to replace <n> tags with <Name> tags, expected to find <Name>All Cities</Name> but didn't")
	}
}
