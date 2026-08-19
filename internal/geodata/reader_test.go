package geodata

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/swayrider/regionservice/internal/types"
	log "github.com/swayrider/swlib/logger"
)

// newTestReader builds a GeoDataReader rooted at a temp dir populated with the
// given relative-path -> content files.
func newTestReader(t *testing.T, files map[string]string) *GeoDataReader {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return NewGeoDataReader(dir, log.New())
}

const featureCollectionGeoJSON = `{
  "type": "FeatureCollection",
  "features": [
    { "type": "Feature", "properties": {}, "geometry": { "type": "Polygon", "coordinates": [[[2,2],[8,2],[8,8],[2,8],[2,2]]] } }
  ]
}`

const featureGeoJSON = `{
  "type": "Feature",
  "properties": {},
  "geometry": { "type": "Polygon", "coordinates": [[[2,2],[8,2],[8,8],[2,8],[2,2]]] }
}`

func TestGetManifest(t *testing.T) {
	manifest := `regions:
  france:
    contour:
      core:
        remote-file: contours/france-core.geojson
      extended:
        remote-file: contours/france-extended.geojson
shared:
  border-crossings:
    france_germany:
      remote-file: border-crossings/france_germany.csv
`
	r := newTestReader(t, map[string]string{"manifest.yml": manifest})

	m, err := r.GetManifest(context.Background())
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}

	fr, ok := m.Regions["france"]
	if !ok {
		t.Fatal("expected france region in manifest")
	}
	if fr.Contour == nil || fr.Contour.Core == nil || fr.Contour.Extended == nil {
		t.Fatal("expected france core+extended contour descriptors")
	}
	if got := fr.Contour.Core.RemoteFile; got != "contours/france-core.geojson" {
		t.Errorf("core remote-file = %q, want contours/france-core.geojson", got)
	}

	bc := m.Shared.BorderCrossings["france_germany"]
	if bc == nil {
		t.Fatal("expected france_germany border crossing descriptor")
	}
	if got := bc.RemoteFile; got != "border-crossings/france_germany.csv" {
		t.Errorf("crossing remote-file = %q, want border-crossings/france_germany.csv", got)
	}
}

func TestGetManifest_MissingFile(t *testing.T) {
	r := newTestReader(t, nil)
	if _, err := r.GetManifest(context.Background()); err == nil {
		t.Fatal("expected error for missing manifest.yml")
	}
}

func TestGetManifest_InvalidYAML(t *testing.T) {
	r := newTestReader(t, map[string]string{"manifest.yml": "regions: ["})
	if _, err := r.GetManifest(context.Background()); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestGetContour_FeatureCollection(t *testing.T) {
	r := newTestReader(t, map[string]string{"contours/x.geojson": featureCollectionGeoJSON})

	fc, err := r.GetContour(context.Background(), &ContourDesc{RemoteFile: "contours/x.geojson"})
	if err != nil {
		t.Fatalf("GetContour: %v", err)
	}
	if fc == nil || len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %+v", fc)
	}
}

func TestGetContour_Feature(t *testing.T) {
	r := newTestReader(t, map[string]string{"contours/x.geojson": featureGeoJSON})

	fc, err := r.GetContour(context.Background(), &ContourDesc{RemoteFile: "contours/x.geojson"})
	if err != nil {
		t.Fatalf("GetContour: %v", err)
	}
	if fc == nil || len(fc.Features) != 1 {
		t.Fatalf("expected single wrapped feature, got %+v", fc)
	}
}

func TestGetContour_MissingFile(t *testing.T) {
	r := newTestReader(t, nil)
	if _, err := r.GetContour(context.Background(), &ContourDesc{RemoteFile: "contours/nope.geojson"}); err == nil {
		t.Fatal("expected error for missing contour file")
	}
}

func TestGetContour_InvalidJSON(t *testing.T) {
	r := newTestReader(t, map[string]string{"contours/x.geojson": "not json"})
	if _, err := r.GetContour(context.Background(), &ContourDesc{RemoteFile: "contours/x.geojson"}); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetContour_UnsupportedGeoJSON(t *testing.T) {
	// A bare geometry (not a Feature/FeatureCollection) must be rejected
	// rather than returning a nil collection alongside a nil error.
	geoJSON := `{"type":"Polygon","coordinates":[[[2,2],[8,2],[8,8],[2,8],[2,2]]]}`
	r := newTestReader(t, map[string]string{"contours/x.geojson": geoJSON})

	fc, err := r.GetContour(context.Background(), &ContourDesc{RemoteFile: "contours/x.geojson"})
	if err == nil {
		t.Fatal("expected error for bare geometry GeoJSON")
	}
	if fc != nil {
		t.Fatalf("expected nil collection, got %+v", fc)
	}
}

const borderCrossingCSV = `osm_id,osm_type,from_region,to_region,lon,lat
4757526,primary,france,germany,7.5,48.5
4757527,motorway,germany,france,7.8,49.0
`

func TestGetBorderCrossing(t *testing.T) {
	r := newTestReader(t, map[string]string{"border-crossings/x.csv": borderCrossingCSV})

	bc, err := r.GetBorderCrossing(context.Background(), &BorderCrossingDesc{RemoteFile: "border-crossings/x.csv"})
	if err != nil {
		t.Fatalf("GetBorderCrossing: %v", err)
	}
	if len(bc) != 2 {
		t.Fatalf("expected 2 crossings, got %d", len(bc))
	}

	first := bc[0]
	if first.FromRegion != "france" || first.ToRegion != "germany" {
		t.Errorf("crossing[0] regions = %q -> %q, want france -> germany", first.FromRegion, first.ToRegion)
	}
	if first.OsmId != 4757526 {
		t.Errorf("crossing[0] osm_id = %d, want 4757526", first.OsmId)
	}
	if first.RoadType != types.PRIMARY {
		t.Errorf("crossing[0] road type = %q, want primary", first.RoadType)
	}
	if first.Lon != 7.5 || first.Lat != 48.5 {
		t.Errorf("crossing[0] lon/lat = %v/%v, want 7.5/48.5", first.Lon, first.Lat)
	}
}

func TestGetBorderCrossing_MissingFile(t *testing.T) {
	r := newTestReader(t, nil)
	if _, err := r.GetBorderCrossing(context.Background(), &BorderCrossingDesc{RemoteFile: "nope.csv"}); err == nil {
		t.Fatal("expected error for missing CSV file")
	}
}

func TestGetBorderCrossing_MalformedCSV(t *testing.T) {
	// Non-numeric osm_id must fail CSV unmarshaling.
	bad := "osm_id,osm_type,from_region,to_region,lon,lat\nnotanumber,primary,france,germany,7.5,48.5\n"
	r := newTestReader(t, map[string]string{"x.csv": bad})

	if _, err := r.GetBorderCrossing(context.Background(), &BorderCrossingDesc{RemoteFile: "x.csv"}); err == nil {
		t.Fatal("expected error for malformed CSV")
	}
}
