package index

import (
	"context"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/regionservice/internal/types"
)

// newTestBorderIndex builds a BorderIndex from a flat list of crossings.
func newTestBorderIndex(crossings types.BorderCrossingCollection) *BorderIndex {
	idx := NewBorderIndex()
	idx.Add(crossings)
	return idx
}

// =============================================================================
// regionInPath Tests
// =============================================================================

func TestRegionInPath(t *testing.T) {
	path := []string{"A", "B", "C"}

	tests := []struct {
		region string
		want   bool
	}{
		{"A", true},
		{"B", true},
		{"C", true},
		{"D", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := regionInPath(path, tt.region)
			if got != tt.want {
				t.Errorf("regionInPath(%q) = %v, want %v", tt.region, got, tt.want)
			}
		})
	}
}

// =============================================================================
// FindRegionPath Tests
// =============================================================================

func TestFindRegionPath_DirectNeighbour(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	path := idx.FindRegionPath(context.Background(), "A", "B")
	if len(path) != 2 || path[0] != "A" || path[1] != "B" {
		t.Errorf("expected [A B], got %v", path)
	}
}

func TestFindRegionPath_TwoHop(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})

	path := idx.FindRegionPath(context.Background(), "A", "C")
	if len(path) != 3 || path[0] != "A" || path[1] != "B" || path[2] != "C" {
		t.Errorf("expected [A B C], got %v", path)
	}
}

func TestFindRegionPath_NoPath(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	path := idx.FindRegionPath(context.Background(), "A", "C")
	if path != nil {
		t.Errorf("expected nil, got %v", path)
	}
}

func TestFindRegionPath_UnknownFrom(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	path := idx.FindRegionPath(context.Background(), "X", "B")
	if path != nil {
		t.Errorf("expected nil for unknown from-region, got %v", path)
	}
}

// =============================================================================
// FindRouteRegionPaths Tests
// =============================================================================

func TestFindRouteRegionPaths_SinglePath(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})
	allowed := map[string]bool{"A": true, "B": true, "C": true}

	paths := idx.FindRouteRegionPaths(context.Background(), "A", "C", allowed)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if len(paths[0]) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(paths[0]), paths[0])
	}
}

func TestFindRouteRegionPaths_MultiplePaths(t *testing.T) {
	// A→B→D and A→C→D are both valid
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "A", ToRegion: "C", RoadType: types.PRIMARY, Lon: 1, Lat: 2},
		{FromRegion: "B", ToRegion: "D", RoadType: types.PRIMARY, Lon: 2, Lat: 1},
		{FromRegion: "C", ToRegion: "D", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})
	allowed := map[string]bool{"A": true, "B": true, "C": true, "D": true}

	paths := idx.FindRouteRegionPaths(context.Background(), "A", "D", allowed)
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

func TestFindRouteRegionPaths_FromNotAllowed(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})
	allowed := map[string]bool{"B": true} // A not included

	paths := idx.FindRouteRegionPaths(context.Background(), "A", "B", allowed)
	if paths != nil {
		t.Errorf("expected nil when fromRegion not allowed, got %v", paths)
	}
}

func TestFindRouteRegionPaths_NoPathWithinAllowed(t *testing.T) {
	// A→B exists, but B is not in allowed set
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})
	allowed := map[string]bool{"A": true, "C": true} // B excluded

	paths := idx.FindRouteRegionPaths(context.Background(), "A", "C", allowed)
	if paths != nil {
		t.Errorf("expected nil when path goes through disallowed region, got %v", paths)
	}
}

// =============================================================================
// FindCrossingLocations Tests
// =============================================================================

func TestFindCrossingLocations_NoCrossings(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5, Lat: 5},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 10}}
	res := idx.FindCrossingLocations(
		context.Background(), "X", "Y",
		line, []string{"MOTORWAY", "PRIMARY"}, 3, 10000, 1000,
	)
	if res != nil {
		t.Errorf("expected nil for unknown region pair, got %v", res)
	}
}

func TestFindCrossingLocations_Limit(t *testing.T) {
	// Four crossings far apart; limit=2
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 0, Lat: 0},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 0},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 2, Lat: 0},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 3, Lat: 0},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{3, 0}}
	res := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, 2, 100000, 1,
	)
	if len(res) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(res))
	}
}

func TestFindCrossingLocations_RoadTypeFilter(t *testing.T) {
	// Only SECONDARY crossings; road order only includes MOTORWAY → no results
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.SECONDARY, Lon: 5, Lat: 5},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 10}}
	res := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"MOTORWAY"}, 3, 10000, 1000,
	)
	if len(res) != 0 {
		t.Errorf("expected 0 results when road type not in filter, got %d", len(res))
	}
}

func TestFindCrossingLocations_NegativeLimitDoesNotPanic(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 0, Lat: 0},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 0},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{1, 0}}
	res := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, -1, 100000, 1,
	)
	if len(res) != 2 {
		t.Errorf("expected 2 results (negative limit must not panic), got %d", len(res))
	}
}

func TestFindCrossingLocations_DropDistance(t *testing.T) {
	// Two crossings at the same spot; dropDistance larger than distance between them
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5.0, Lat: 5.0},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5.001, Lat: 5.001},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 10}}
	res := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, 3, 100000, 1000000, // very large dropDistance
	)
	if len(res) != 1 {
		t.Errorf("expected 1 result after dropDistance dedup, got %d", len(res))
	}
}

// =============================================================================
// FindClosestCrossing Tests
// =============================================================================

func TestFindClosestCrossing_ReturnsClosest(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1, OsmId: 1},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 10, Lat: 10, OsmId: 2},
	})

	res := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{1.1, 1.1}, nil,
	)
	if res == nil {
		t.Fatal("expected a result, got nil")
	}
	if res.BorderCrossing.OsmId != 1 {
		t.Errorf("expected OsmId 1 (closer crossing), got %d", res.BorderCrossing.OsmId)
	}
}

func TestFindClosestCrossing_NoCrossings(t *testing.T) {
	idx := NewBorderIndex()

	res := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{5, 5}, nil,
	)
	if res != nil {
		t.Errorf("expected nil for empty index, got %v", res)
	}
}

func TestFindClosestCrossing_RoadTypeFilter(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.SECONDARY, Lon: 1, Lat: 1, OsmId: 1},
		{FromRegion: "A", ToRegion: "B", RoadType: types.MOTORWAY, Lon: 9, Lat: 9, OsmId: 2},
	})

	// Filter to MOTORWAY only — should return the farther crossing
	res := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{1, 1}, []string{"MOTORWAY"},
	)
	if res == nil {
		t.Fatal("expected a result, got nil")
	}
	if res.BorderCrossing.OsmId != 2 {
		t.Errorf("expected OsmId 2 (MOTORWAY crossing), got %d", res.BorderCrossing.OsmId)
	}
}
