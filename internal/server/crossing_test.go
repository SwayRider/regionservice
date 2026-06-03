package server

import (
	"context"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/regionservice/internal/types"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/protos/common_types/geo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validCrossingReq returns a request with all required fields set.
func validCrossingReq() *regionv1.FindCrossingLocationsRequest {
	return &regionv1.FindCrossingLocationsRequest{
		FromRegion:   "A",
		ToRegion:     "B",
		FromLocation: &geo.Coordinate{Lon: 0, Lat: 0},
		ToLocation:   &geo.Coordinate{Lon: 1, Lat: 1},
		ConfigOneof: &regionv1.FindCrossingLocationsRequest_SimpleConfig{
			SimpleConfig: &regionv1.BorderCrossingSimpleConfig{},
		},
	}
}

// =============================================================================
// FindCrossingLocations Validation Tests
// =============================================================================

func TestFindCrossingLocations_Validation(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})

	tests := []struct {
		name    string
		mutate  func(*regionv1.FindCrossingLocationsRequest)
	}{
		{"missing FromRegion", func(r *regionv1.FindCrossingLocationsRequest) { r.FromRegion = "" }},
		{"missing ToRegion", func(r *regionv1.FindCrossingLocationsRequest) { r.ToRegion = "" }},
		{"missing FromLocation", func(r *regionv1.FindCrossingLocationsRequest) { r.FromLocation = nil }},
		{"missing ToLocation", func(r *regionv1.FindCrossingLocationsRequest) { r.ToLocation = nil }},
		{"missing Config", func(r *regionv1.FindCrossingLocationsRequest) { r.ConfigOneof = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCrossingReq()
			tt.mutate(req)
			_, err := s.FindCrossingLocations(context.Background(), req)
			if code := status.Code(err); code != codes.InvalidArgument {
				t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
			}
		})
	}
}

// =============================================================================
// FindCrossingLocations SimpleConfig Tests
// =============================================================================

func TestFindCrossingLocations_SimpleConfig_DefaultsApplied(t *testing.T) {
	var gotLimit int
	var gotDelta, gotDrop float64
	bq := &mockBorderQuerier{
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, limit int, delta, drop float64) []*index.BorderCrossingResult {
			gotLimit = limit
			gotDelta = delta
			gotDrop = drop
			return nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	req := &regionv1.FindCrossingLocationsRequest{
		FromRegion:   "A",
		ToRegion:     "B",
		FromLocation: &geo.Coordinate{Lon: 0, Lat: 0},
		ToLocation:   &geo.Coordinate{Lon: 1, Lat: 1},
		Limit:        0, // should default to 3
		ConfigOneof: &regionv1.FindCrossingLocationsRequest_SimpleConfig{
			SimpleConfig: &regionv1.BorderCrossingSimpleConfig{
				RoadTypeDelta: 0, // should default to 10000
				DropDistance:  0, // should default to delta * 0.1
			},
		},
	}
	_, err := s.FindCrossingLocations(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 3 {
		t.Errorf("limit = %d, want 3", gotLimit)
	}
	if gotDelta != 10000 {
		t.Errorf("roadTypeDelta = %v, want 10000", gotDelta)
	}
	if gotDrop != 10000*0.1 {
		t.Errorf("dropDistance = %v, want %v", gotDrop, 10000*0.1)
	}
}

func TestFindCrossingLocations_SimpleConfig_ResponseMapping(t *testing.T) {
	bq := &mockBorderQuerier{
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, _ int, _, _ float64) []*index.BorderCrossingResult {
			return []*index.BorderCrossingResult{
				{
					BorderCrossing: &index.BorderCrossing{
						FromRegion: "A",
						ToRegion:   "B",
						RoadType:   types.MOTORWAY,
						OsmId:      42,
						Location:   orb.Point{5, 10},
					},
				},
			}
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)
	resp, err := s.FindCrossingLocations(context.Background(), validCrossingReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Crossings) != 1 {
		t.Fatalf("expected 1 crossing, got %d", len(resp.Crossings))
	}
	c := resp.Crossings[0]
	if c.FromRegion != "A" || c.ToRegion != "B" {
		t.Errorf("regions = (%q, %q), want (A, B)", c.FromRegion, c.ToRegion)
	}
	if c.RoadType != regionv1.RoadType_MOTORWAY {
		t.Errorf("RoadType = %v, want MOTORWAY", c.RoadType)
	}
	if c.OsmId != 42 {
		t.Errorf("OsmId = %d, want 42", c.OsmId)
	}
	if c.Location.Lon != 5 || c.Location.Lat != 10 {
		t.Errorf("Location = %v, want {5 10}", c.Location)
	}
}

// =============================================================================
// FindCrossingLocations AdvancedConfig Tests
// =============================================================================

func TestFindCrossingLocations_AdvancedConfig_NoForwardCrossing(t *testing.T) {
	// FindClosestCrossing returns nil → NotFound
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})

	_, err := s.FindCrossingLocations(context.Background(), &regionv1.FindCrossingLocationsRequest{
		FromRegion:   "A",
		ToRegion:     "B",
		FromLocation: &geo.Coordinate{Lon: 0, Lat: 0},
		ToLocation:   &geo.Coordinate{Lon: 1, Lat: 1},
		ConfigOneof: &regionv1.FindCrossingLocationsRequest_AdvancedConfig{
			AdvancedConfig: &regionv1.BorderCrossingAdvancedConfig{},
		},
	})
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("code = %v, want %v", code, codes.NotFound)
	}
}

// =============================================================================
// findCrossingDefinition Tests
// =============================================================================

func TestFindCrossingDefinition(t *testing.T) {
	def50 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 50}
	def100 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 100}
	def200 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 200}

	tests := []struct {
		name        string
		refDistance float64
		defs        []*regionv1.BorderCrossingDefinition
		wantMax     float64
	}{
		// After sorting: [50, 100, 200]. Loop starts at i=1.
		// refDistance=60: 60 <= 100 → return definitions[1] (max=100)
		{"within second definition", 60, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, 100},
		// refDistance=150: 150 > 100, 150 <= 200 → return definitions[2] (max=200)
		{"within third definition", 150, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, 200},
		// refDistance=300: exceeds all in loop → fallback definitions[0] (max=50)
		{"exceeds all, fallback", 300, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, 50},
		// Single definition: loop [1..0] never runs → always definitions[0]
		{"single definition", 99999, []*regionv1.BorderCrossingDefinition{def100}, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCrossingDefinition(tt.refDistance, tt.defs)
			if got.MaxBorderDistance != tt.wantMax {
				t.Errorf("MaxBorderDistance = %v, want %v", got.MaxBorderDistance, tt.wantMax)
			}
		})
	}
}
