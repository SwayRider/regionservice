package geodata

import (
	"testing"

	"github.com/go-spatial/geom/encoding/geojson"
)

func TestToFeatureCollection_FeatureCollection(t *testing.T) {
	in := geojson.FeatureCollection{
		Features: []geojson.Feature{{}, {}},
	}
	fc, err := toFeatureCollection(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc == nil {
		t.Fatal("expected non-nil FeatureCollection")
	}
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(fc.Features))
	}
}

func TestToFeatureCollection_Feature(t *testing.T) {
	in := geojson.Feature{}
	fc, err := toFeatureCollection(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc == nil {
		t.Fatal("expected non-nil FeatureCollection")
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}
}

func TestToFeatureCollection_Nil(t *testing.T) {
	fc, err := toFeatureCollection(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
	if fc != nil {
		t.Fatalf("expected nil FeatureCollection, got %v", fc)
	}
}

func TestToFeatureCollection_UnsupportedType(t *testing.T) {
	fc, err := toFeatureCollection("not a geojson value")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if fc != nil {
		t.Fatalf("expected nil FeatureCollection, got %v", fc)
	}
}
