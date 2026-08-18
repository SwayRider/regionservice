package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
