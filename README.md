# GIS Test - SLD Parser Implementation

This project implements a WMS-compatible service that renders maps using SLD (Styled Layer Descriptor) styling.

## SLD Parsing Implementation

We provide multiple approaches for SLD parsing:

1. **XML-based Parser**: Uses Go's XML parsing libraries to handle SLD files properly
2. **GDAL Adapter**: Uses GDAL command-line tools for SLD processing and rendering

## Usage

Run the service with:

```bash
# Use the default XML-based parser
go run cmd/main.go

# Use the GDAL CLI adapter
go run cmd/main.go --use-gdal-cli
```

## WMS Endpoint

The service exposes a WMS endpoint at `/wms` that accepts standard WMS parameters:

- `SERVICE`: Must be "WMS"
- `VERSION`: WMS version
- `REQUEST`: Must be "GetMap"
- `LAYERS`: Layer name to render
- `SLD` or `STYLES`: SLD styling to apply (URL, file path, or inline XML)
- `BBOX`: Geographic bounding box (minX,minY,maxX,maxY)
- `WIDTH`: Output image width
- `HEIGHT`: Output image height
- `FORMAT`: Output image format

## GDAL CLI Tools Integration

For better SLD support, the system can leverage GDAL command line tools:

- **ogr2ogr**: For vector data conversion with styling
- **qgis_process**: For advanced rasterization with styling

## Testing SLD Parsers

### Shell Script for GDAL Tools

A test script is provided to demonstrate using GDAL tools for SLD processing:

```bash
./test_gdal_sld.sh
```

### Test Program

A Go test program is provided to demonstrate how to use the different SLD parsers:

```bash
# Test with XML-based parser
go run cmd/test_sld/main.go --styles-dir=./styles --sld-file=blue_cities_correct.sld

# Test with GDAL CLI adapter
go run cmd/test_sld/main.go --styles-dir=./styles --sld-file=blue_cities_correct.sld --use-gdal

# Test with output file (requires GDAL CLI adapter)
go run cmd/test_sld/main.go --styles-dir=./styles --sld-file=blue_cities_correct.sld --use-gdal --output=./outputs/test_output.geojson
```

## Best Practices

As recommended, we've moved away from string/regex-based SLD parsing in favor of:

1. **Proper XML parsing libraries** like etree for structured SLD handling
2. **Specialized GIS libraries** that understand SLD semantics
3. **GDAL tools** for complex styling scenarios

This approach is more robust for handling complex SLD documents and avoids issues with malformed XML.
