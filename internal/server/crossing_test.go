package server

import (
	"context"
	"math"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/protos/common_types/geo"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/regionservice/internal/types"
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
		name   string
		mutate func(*regionv1.FindCrossingLocationsRequest)
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
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, limit int, delta, drop float64) ([]*index.BorderCrossingResult, error) {
			gotLimit = limit
			gotDelta = delta
			gotDrop = drop
			return nil, nil
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
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, _ int, _, _ float64) ([]*index.BorderCrossingResult, error) {
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
			}, nil
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

func TestFindCrossingLocations_AdvancedConfig_NoFallbackDefinition(t *testing.T) {
	// Reference distance (300m) exceeds every non-zero definition and there is
	// no 0-max fallback entry → findCrossingDefinition returns nil → NotFound.
	bq := &mockBorderQuerier{
		findClosestCrossingFn: func(_ context.Context, _, _ string, _ orb.Point, _ []string) (*index.ClosestBorderCrossing, error) {
			return &index.ClosestBorderCrossing{
				Distance: 300,
				BorderCrossing: &index.BorderCrossing{
					FromRegion: "A",
					ToRegion:   "B",
					Location:   orb.Point{1, 1},
				},
			}, nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	_, err := s.FindCrossingLocations(context.Background(), &regionv1.FindCrossingLocationsRequest{
		FromRegion:   "A",
		ToRegion:     "B",
		FromLocation: &geo.Coordinate{Lon: 0, Lat: 0},
		ToLocation:   &geo.Coordinate{Lon: 1, Lat: 1},
		ConfigOneof: &regionv1.FindCrossingLocationsRequest_AdvancedConfig{
			AdvancedConfig: &regionv1.BorderCrossingAdvancedConfig{
				Definitions: []*regionv1.BorderCrossingDefinition{
					{MaxBorderDistance: 50},
					{MaxBorderDistance: 100},
				},
			},
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
	def0 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 0}
	def50 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 50}
	def100 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 100}
	def200 := &regionv1.BorderCrossingDefinition{MaxBorderDistance: 200}

	tests := []struct {
		name        string
		refDistance float64
		defs        []*regionv1.BorderCrossingDefinition
		want        *regionv1.BorderCrossingDefinition // nil means no definition
	}{
		{"smallest covering band", 30, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, def50},
		{"exactly at band boundary", 50, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, def50},
		{"within second band", 60, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, def100},
		{"within third band", 150, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, def200},
		{"exceeds all without fallback", 300, []*regionv1.BorderCrossingDefinition{def50, def100, def200}, nil},
		{"exceeds all with fallback", 300, []*regionv1.BorderCrossingDefinition{def0, def50, def100, def200}, def0},
		{"unsorted input", 60, []*regionv1.BorderCrossingDefinition{def200, def50, def100}, def100},
		{"unsorted input with fallback", 300, []*regionv1.BorderCrossingDefinition{def100, def0, def200, def50}, def0},
		{"single definition within max", 50, []*regionv1.BorderCrossingDefinition{def100}, def100},
		{"single definition exceeds max", 99999, []*regionv1.BorderCrossingDefinition{def100}, nil},
		{"empty definitions", 60, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findCrossingDefinition(tt.refDistance, tt.defs)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindCrossingDefinition_DoesNotMutateInput(t *testing.T) {
	defs := []*regionv1.BorderCrossingDefinition{
		{MaxBorderDistance: 200},
		{MaxBorderDistance: 0},
		{MaxBorderDistance: 100},
		{MaxBorderDistance: 50},
	}
	before := append([]*regionv1.BorderCrossingDefinition(nil), defs...)

	got := findCrossingDefinition(60, defs)
	if got == nil || got.MaxBorderDistance != 100 {
		t.Errorf("MaxBorderDistance = %v, want 100", got)
	}
	for i := range defs {
		if defs[i] != before[i] {
			t.Fatalf("input slice mutated at index %d", i)
		}
	}
}

// =============================================================================
// Negative limit regression tests (point 2)
// =============================================================================

func TestFindCrossingLocations_SimpleConfig_NegativeLimitDefaults(t *testing.T) {
	var gotLimit int
	bq := &mockBorderQuerier{
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, limit int, _, _ float64) ([]*index.BorderCrossingResult, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	req := validCrossingReq()
	req.Limit = -1
	if _, err := s.FindCrossingLocations(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 3 {
		t.Errorf("limit = %d, want 3", gotLimit)
	}
}

func TestFindCrossingLocations_AdvancedConfig_NegativeLimitDefaults(t *testing.T) {
	var gotLimit int
	bq := &mockBorderQuerier{
		findClosestCrossingFn: func(_ context.Context, _, _ string, _ orb.Point, _ []string) (*index.ClosestBorderCrossing, error) {
			return &index.ClosestBorderCrossing{
				Distance: 100,
				BorderCrossing: &index.BorderCrossing{
					FromRegion: "A",
					ToRegion:   "B",
					Location:   orb.Point{1, 1},
				},
			}, nil
		},
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, limit int, _, _ float64) ([]*index.BorderCrossingResult, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	req := validCrossingReq()
	req.Limit = -1
	req.ConfigOneof = &regionv1.FindCrossingLocationsRequest_AdvancedConfig{
		AdvancedConfig: &regionv1.BorderCrossingAdvancedConfig{
			Definitions: []*regionv1.BorderCrossingDefinition{
				{MaxBorderDistance: 0, RoadTypeOrder: []regionv1.RoadType{regionv1.RoadType_MOTORWAY}},
				{MaxBorderDistance: 50, RoadTypeOrder: []regionv1.RoadType{regionv1.RoadType_MOTORWAY}},
			},
		},
	}
	if _, err := s.FindCrossingLocations(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 3 {
		t.Errorf("limit = %d, want 3", gotLimit)
	}
}

// =============================================================================
// Coordinate validation tests (point 3)
// =============================================================================

func TestFindCrossingLocations_InvalidCoordinates(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})

	tests := []struct {
		name   string
		mutate func(*regionv1.FindCrossingLocationsRequest)
	}{
		{"out-of-range latitude", func(r *regionv1.FindCrossingLocationsRequest) { r.FromLocation.Lat = 91 }},
		{"NaN longitude", func(r *regionv1.FindCrossingLocationsRequest) { r.ToLocation.Lon = math.NaN() }},
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

func TestFindCrossingLocations_IndexError(t *testing.T) {
	// The handler must propagate an index error (e.g. context cancellation).
	bq := &mockBorderQuerier{
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, _ int, _, _ float64) ([]*index.BorderCrossingResult, error) {
			return nil, context.Canceled
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	_, err := s.FindCrossingLocations(context.Background(), validCrossingReq())
	if err != context.Canceled {
		t.Errorf("err = %v, want %v", err, context.Canceled)
	}
}

// =============================================================================
// Request-mutation regression tests (point 16)
// =============================================================================

func TestFindCrossingLocations_SimpleConfig_DoesNotMutateConfig(t *testing.T) {
	cfg := &regionv1.BorderCrossingSimpleConfig{
		RoadTypeOrder: nil,
		RoadTypeDelta: 0,
		DropDistance:  0,
	}
	req := validCrossingReq()
	req.ConfigOneof = &regionv1.FindCrossingLocationsRequest_SimpleConfig{SimpleConfig: cfg}

	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	if _, err := s.FindCrossingLocations(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RoadTypeOrder != nil {
		t.Errorf("RoadTypeOrder mutated: %v", cfg.RoadTypeOrder)
	}
	if cfg.RoadTypeDelta != 0 {
		t.Errorf("RoadTypeDelta mutated: %v", cfg.RoadTypeDelta)
	}
	if cfg.DropDistance != 0 {
		t.Errorf("DropDistance mutated: %v", cfg.DropDistance)
	}
}

func TestFindCrossingLocations_AdvancedConfig_DoesNotMutateDefinitions(t *testing.T) {
	def := &regionv1.BorderCrossingDefinition{
		MaxBorderDistance: 100,
		RoadTypeDelta:     0,
		DropDistance:      0,
		RoadTypeOrder:     []regionv1.RoadType{regionv1.RoadType_MOTORWAY},
	}
	bq := &mockBorderQuerier{
		findClosestCrossingFn: func(_ context.Context, _, _ string, _ orb.Point, _ []string) (*index.ClosestBorderCrossing, error) {
			return &index.ClosestBorderCrossing{
				Distance: 50,
				BorderCrossing: &index.BorderCrossing{
					FromRegion: "A",
					ToRegion:   "B",
					Location:   orb.Point{1, 1},
				},
			}, nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	req := validCrossingReq()
	req.ConfigOneof = &regionv1.FindCrossingLocationsRequest_AdvancedConfig{
		AdvancedConfig: &regionv1.BorderCrossingAdvancedConfig{
			Definitions: []*regionv1.BorderCrossingDefinition{def},
		},
	}
	if _, err := s.FindCrossingLocations(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.RoadTypeDelta != 0 {
		t.Errorf("definition RoadTypeDelta mutated: %v", def.RoadTypeDelta)
	}
	if def.DropDistance != 0 {
		t.Errorf("definition DropDistance mutated: %v", def.DropDistance)
	}
}

// =============================================================================
// Unknown road-type regression tests (point 17)
// =============================================================================

func TestRoadTypeOrderStrings(t *testing.T) {
	got := roadTypeOrderStrings([]regionv1.RoadType{
		regionv1.RoadType_MOTORWAY,
		regionv1.RoadType(99), // out of range
		regionv1.RoadType_TRUNK,
		regionv1.RoadType(-1), // out of range
	})
	want := []string{"MOTORWAY", "TRUNK"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRoadTypeToProto(t *testing.T) {
	tests := []struct {
		rt   types.RoadType
		want regionv1.RoadType
		ok   bool
	}{
		{types.MOTORWAY, regionv1.RoadType_MOTORWAY, true},
		{types.TRUNK, regionv1.RoadType_TRUNK, true},
		{types.PRIMARY, regionv1.RoadType_PRIMARY, true},
		{types.SECONDARY, regionv1.RoadType_SECONDARY, true},
		{types.RoadType("MOTORWAY"), regionv1.RoadType_MOTORWAY, true}, // case-insensitive
		{types.RoadType("residential"), 0, false},
		{types.RoadType(""), 0, false},
	}

	for _, tt := range tests {
		got, ok := roadTypeToProto(tt.rt)
		if ok != tt.ok || got != tt.want {
			t.Errorf("roadTypeToProto(%q) = (%v, %v), want (%v, %v)", tt.rt, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFindCrossingLocations_UnknownRoadType_Skipped(t *testing.T) {
	bq := &mockBorderQuerier{
		findCrossingLocationsFn: func(_ context.Context, _, _ string, _ orb.LineString, _ []string, _ int, _, _ float64) ([]*index.BorderCrossingResult, error) {
			return []*index.BorderCrossingResult{
				{
					BorderCrossing: &index.BorderCrossing{
						FromRegion: "A",
						ToRegion:   "B",
						RoadType:   types.RoadType("residential"),
						OsmId:      1,
						Location:   orb.Point{1, 1},
					},
				},
				{
					BorderCrossing: &index.BorderCrossing{
						FromRegion: "A",
						ToRegion:   "B",
						RoadType:   types.MOTORWAY,
						OsmId:      2,
						Location:   orb.Point{2, 2},
					},
				},
			}, nil
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)

	resp, err := s.FindCrossingLocations(context.Background(), validCrossingReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Crossings) != 1 {
		t.Fatalf("expected 1 crossing (unknown road type skipped), got %d", len(resp.Crossings))
	}
	if resp.Crossings[0].OsmId != 2 {
		t.Errorf("OsmId = %d, want 2", resp.Crossings[0].OsmId)
	}
	if resp.Crossings[0].RoadType != regionv1.RoadType_MOTORWAY {
		t.Errorf("RoadType = %v, want MOTORWAY", resp.Crossings[0].RoadType)
	}
}
