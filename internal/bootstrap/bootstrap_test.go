package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/regionservice/internal/geodata"
	"github.com/swayrider/regionservice/internal/index"
	log "github.com/swayrider/swlib/logger"
)

func bootstrapWithManifest(t *testing.T, manifest string) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	reader := geodata.NewGeoDataReader(dir, log.New())
	return Bootstrap(reader, index.NewRegionIndex(), index.NewBorderIndex())
}

func TestBootstrap_MissingContour(t *testing.T) {
	err := bootstrapWithManifest(t, "regions:\n  france: {}\n")
	if err == nil || !strings.Contains(err.Error(), "missing contour descriptor") {
		t.Fatalf("expected missing contour descriptor error, got %v", err)
	}
}

func TestBootstrap_MissingCoreContour(t *testing.T) {
	err := bootstrapWithManifest(t, "regions:\n  france:\n    contour: {}\n")
	if err == nil || !strings.Contains(err.Error(), "missing core contour descriptor") {
		t.Fatalf("expected missing core contour descriptor error, got %v", err)
	}
}

func TestBootstrap_MissingExtendedContour(t *testing.T) {
	err := bootstrapWithManifest(t, "regions:\n  france:\n    contour:\n      core:\n        remote-file: core.geojson\n")
	if err == nil || !strings.Contains(err.Error(), "missing extended contour descriptor") {
		t.Fatalf("expected missing extended contour descriptor error, got %v", err)
	}
}

func TestBootstrap_NilBorderCrossing(t *testing.T) {
	err := bootstrapWithManifest(t, "regions: {}\nshared:\n  border-crossings:\n    foo: null\n")
	if err == nil || !strings.Contains(err.Error(), "missing descriptor") {
		t.Fatalf("expected missing descriptor error, got %v", err)
	}
}

// bootstrapWithFiles writes the given relative-path -> content files into a
// temp dir, then runs Bootstrap against a fresh reader and indices.
func bootstrapWithFiles(t *testing.T, files map[string]string) (*index.RegionIndex, *index.BorderIndex, error) {
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

	reader := geodata.NewGeoDataReader(dir, log.New())
	ri := index.NewRegionIndex()
	bi := index.NewBorderIndex()
	return ri, bi, Bootstrap(reader, ri, bi)
}

const successManifest = `regions:
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

const coreContour = `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{},"geometry":{"type":"Polygon","coordinates":[[[2,2],[8,2],[8,8],[2,8],[2,2]]]}}]}`

const extendedContour = `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{},"geometry":{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]]]}}]}`

const crossingCSV = "osm_id,osm_type,from_region,to_region,lon,lat\n4757526,primary,france,germany,7.5,48.5\n"

func TestBootstrap_Success(t *testing.T) {
	ri, bi, err := bootstrapWithFiles(t, map[string]string{
		"manifest.yml":                        successManifest,
		"contours/france-core.geojson":        coreContour,
		"contours/france-extended.geojson":    extendedContour,
		"border-crossings/france_germany.csv": crossingCSV,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Region index populated: a point inside the core contour is found.
	res := ri.SearchByPoint(orb.Point{5, 5}, false)
	if len(res) != 1 || res[0].Region.Name() != "france" || res[0].IsExtended {
		t.Fatalf("core point: got %+v, want non-extended france", res)
	}

	// A point only inside the extended contour is found when extended=true.
	res = ri.SearchByPoint(orb.Point{9, 9}, true)
	if len(res) != 1 || res[0].Region.Name() != "france" || !res[0].IsExtended {
		t.Fatalf("extended point: got %+v, want extended france", res)
	}

	// Border index populated: the crossing is reachable between the regions.
	crossings, err := bi.FindCrossingLocations(
		context.Background(), "france", "germany",
		orb.LineString{{0, 0}, {10, 10}}, []string{"primary"}, 1, 10000, 1000,
	)
	if err != nil {
		t.Fatalf("FindCrossingLocations: %v", err)
	}
	if len(crossings) == 0 {
		t.Fatal("expected at least one border crossing")
	}
}

func TestBootstrap_MissingContourFile(t *testing.T) {
	_, _, err := bootstrapWithFiles(t, map[string]string{
		"manifest.yml": successManifest,
		// core contour is referenced by the manifest but absent.
		"contours/france-extended.geojson":    extendedContour,
		"border-crossings/france_germany.csv": crossingCSV,
	})
	if err == nil {
		t.Fatal("expected error when a contour file is missing")
	}
}

func TestBootstrap_MissingCrossingFile(t *testing.T) {
	_, _, err := bootstrapWithFiles(t, map[string]string{
		"manifest.yml":                     successManifest,
		"contours/france-core.geojson":     coreContour,
		"contours/france-extended.geojson": extendedContour,
		// crossing CSV is referenced by the manifest but absent.
	})
	if err == nil {
		t.Fatal("expected error when a crossing file is missing")
	}
}
