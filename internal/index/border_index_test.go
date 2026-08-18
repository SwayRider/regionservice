package index

import (
	"context"
	"fmt"
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

	path, err := idx.FindRegionPath(context.Background(), "A", "B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 2 || path[0] != "A" || path[1] != "B" {
		t.Errorf("expected [A B], got %v", path)
	}
}

func TestFindRegionPath_TwoHop(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})

	path, err := idx.FindRegionPath(context.Background(), "A", "C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 3 || path[0] != "A" || path[1] != "B" || path[2] != "C" {
		t.Errorf("expected [A B C], got %v", path)
	}
}

func TestFindRegionPath_NoPath(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	path, err := idx.FindRegionPath(context.Background(), "A", "C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != nil {
		t.Errorf("expected nil, got %v", path)
	}
}

func TestFindRegionPath_UnknownFrom(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	path, err := idx.FindRegionPath(context.Background(), "X", "B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "C", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "D", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

func TestFindRouteRegionPaths_FromNotAllowed(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})
	allowed := map[string]bool{"B": true} // A not included

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "B", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "C", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil when path goes through disallowed region, got %v", paths)
	}
}

func TestFindRouteRegionPaths_PreservesLongerPaths(t *testing.T) {
	// Shortest is A→D, but the corridor may favor the longer A→B→C→D detour;
	// both must be returned (within 2× shortest).
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "D", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
		{FromRegion: "C", ToRegion: "D", RoadType: types.PRIMARY, Lon: 3, Lat: 3},
	})
	allowed := map[string]bool{"A": true, "B": true, "C": true, "D": true}

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "D", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsPath(paths, []string{"A", "D"}) {
		t.Errorf("shortest path [A D] missing from %v", paths)
	}
	if !containsPath(paths, []string{"A", "B", "C", "D"}) {
		t.Errorf("longer path [A B C D] missing from %v", paths)
	}
}

func TestFindRouteRegionPaths_Cap(t *testing.T) {
	// 150 distinct shortest paths A→Xi→D; the result must be capped.
	crossings := make(types.BorderCrossingCollection, 0, 300)
	allowed := map[string]bool{"A": true, "D": true}
	for i := 0; i < 150; i++ {
		name := fmt.Sprintf("X%03d", i)
		allowed[name] = true
		crossings = append(crossings,
			types.BorderCrossing{FromRegion: "A", ToRegion: name, RoadType: types.PRIMARY, Lon: 1, Lat: 1},
			types.BorderCrossing{FromRegion: name, ToRegion: "D", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
		)
	}
	idx := newTestBorderIndex(crossings)

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "D", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != maxRouteRegionPaths {
		t.Errorf("got %d paths, want capped at %d", len(paths), maxRouteRegionPaths)
	}
}

func TestFindRouteRegionPaths_ContextCancelled(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})
	allowed := map[string]bool{"A": true, "B": true, "C": true}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	paths, err := idx.FindRouteRegionPaths(ctx, "A", "C", allowed)
	if err != context.Canceled {
		t.Errorf("err = %v, want %v", err, context.Canceled)
	}
	if paths != nil {
		t.Errorf("paths = %v, want nil", paths)
	}
}

func TestFindRouteRegionPaths_DeterministicOrder(t *testing.T) {
	// A→B→D and A→C→D both valid; lexicographic neighbor order must yield
	// [A B D] before [A C D] deterministically.
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "A", ToRegion: "C", RoadType: types.PRIMARY, Lon: 1, Lat: 2},
		{FromRegion: "B", ToRegion: "D", RoadType: types.PRIMARY, Lon: 2, Lat: 1},
		{FromRegion: "C", ToRegion: "D", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})
	allowed := map[string]bool{"A": true, "B": true, "C": true, "D": true}

	paths, err := idx.FindRouteRegionPaths(context.Background(), "A", "D", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 || !equalPath(paths[0], []string{"A", "B", "D"}) || !equalPath(paths[1], []string{"A", "C", "D"}) {
		t.Errorf("paths = %v, want [[A B D] [A C D]]", paths)
	}
}

func containsPath(paths [][]string, want []string) bool {
	for _, p := range paths {
		if equalPath(p, want) {
			return true
		}
	}
	return false
}

func equalPath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// =============================================================================
// FindCrossingLocations Tests
// =============================================================================

func TestFindCrossingLocations_NoCrossings(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5, Lat: 5},
	})

	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 10}}
	res, err := idx.FindCrossingLocations(
		context.Background(), "X", "Y",
		line, []string{"MOTORWAY", "PRIMARY"}, 3, 10000, 1000,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, 2, 100000, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"MOTORWAY"}, 3, 10000, 1000,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, -1, 100000, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, 3, 100000, 1000000, // very large dropDistance
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	res, err := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{1.1, 1.1}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result, got nil")
	}
	if res.BorderCrossing.OsmId != 1 {
		t.Errorf("expected OsmId 1 (closer crossing), got %d", res.BorderCrossing.OsmId)
	}
}

func TestFindClosestCrossing_NoCrossings(t *testing.T) {
	idx := NewBorderIndex()

	res, err := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{5, 5}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	res, err := idx.FindClosestCrossing(
		context.Background(), "A", "B",
		orb.Point{1, 1}, []string{"MOTORWAY"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result, got nil")
	}
	if res.BorderCrossing.OsmId != 2 {
		t.Errorf("expected OsmId 2 (MOTORWAY crossing), got %d", res.BorderCrossing.OsmId)
	}
}

// =============================================================================
// FindCrossingLocations Ranking Tests (point 6)
// =============================================================================

// assertOsmIds verifies the OsmId order of a result list.
func assertOsmIds(t *testing.T, res []*BorderCrossingResult, want []int) {
	t.Helper()
	if len(res) != len(want) {
		t.Fatalf("got %d results, want %d", len(res), len(want))
	}
	for i, r := range res {
		if r.BorderCrossing.OsmId != want[i] {
			t.Errorf("result[%d] OsmId = %d, want %d", i, r.BorderCrossing.OsmId, want[i])
		}
	}
}

func TestFindCrossingLocations_SameRoadTypeSortedByDistance(t *testing.T) {
	// Both PRIMARY, at ~111m and ~222m from the line; nearer must sort first
	// regardless of insertion order.
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5, Lat: 0.002, OsmId: 2},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 5, Lat: 0.001, OsmId: 1},
	})
	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 0}}
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"PRIMARY"}, 10, 1000, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOsmIds(t, res, []int{1, 2})
}

func TestFindCrossingLocations_RoadTypePrecedenceWithinDelta(t *testing.T) {
	// MOTORWAY (~167m) and SECONDARY (~111m) are in the same bucket, so road
	// type decides: the farther MOTORWAY must rank before the nearer SECONDARY.
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.SECONDARY, Lon: 5, Lat: 0.001, OsmId: 2},
		{FromRegion: "A", ToRegion: "B", RoadType: types.MOTORWAY, Lon: 5, Lat: 0.0015, OsmId: 1},
	})
	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 0}}
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"MOTORWAY", "SECONDARY"}, 10, 1000, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOsmIds(t, res, []int{1, 2})
}

func TestFindCrossingLocations_DistanceBeatsRoadTypeBeyondDelta(t *testing.T) {
	// delta=100m: SECONDARY at ~11m (bucket 0) vs MOTORWAY at ~111m (bucket 1).
	// The much nearer SECONDARY must rank first despite lower road priority.
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.MOTORWAY, Lon: 5, Lat: 0.001, OsmId: 1},
		{FromRegion: "A", ToRegion: "B", RoadType: types.SECONDARY, Lon: 5, Lat: 0.0001, OsmId: 2},
	})
	line := orb.LineString{orb.Point{0, 0}, orb.Point{10, 0}}
	res, err := idx.FindCrossingLocations(
		context.Background(), "A", "B",
		line, []string{"MOTORWAY", "SECONDARY"}, 10, 100, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOsmIds(t, res, []int{2, 1})
}

func TestFindCrossingLocations_DeterministicOrdering(t *testing.T) {
	// Three crossings form a cycle under the old pairwise comparator:
	//   A(MOTORWAY,~111m) vs B(PRIMARY,~222m): within delta -> road type A<B
	//   B vs C(SECONDARY,~55m):              within delta -> road type B<C
	//   A vs C:                              beyond delta -> distance C<A
	// i.e. A<B<C<A, so sort.Slice order depended on input order. The total
	// order key must yield the same ranking for forward and reversed input.
	crossings := types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.SECONDARY, Lon: 0.04, Lat: 0.0005, OsmId: 3},
		{FromRegion: "A", ToRegion: "B", RoadType: types.MOTORWAY, Lon: 0.06, Lat: 0.001, OsmId: 1},
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 0.05, Lat: 0.002, OsmId: 2},
	}
	line := orb.LineString{orb.Point{0, 0}, orb.Point{0.1, 0}}
	roadOrder := []string{"MOTORWAY", "PRIMARY", "SECONDARY"}

	// All three fall in the same 2000m bucket, so road priority decides.
	want := []int{1, 2, 3}

	forward := newTestBorderIndex(crossings)
	res, err := forward.FindCrossingLocations(
		context.Background(), "A", "B", line, roadOrder, 10, 2000, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOsmIds(t, res, want)

	reversed := make(types.BorderCrossingCollection, len(crossings))
	for i := range crossings {
		reversed[len(crossings)-1-i] = crossings[i]
	}
	backward := newTestBorderIndex(reversed)
	res, err = backward.FindCrossingLocations(
		context.Background(), "A", "B", line, roadOrder, 10, 2000, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOsmIds(t, res, want)
}

// =============================================================================
// Context cancellation tests (point 10)
// =============================================================================

func TestFindCrossingLocations_ContextCancelled(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})
	line := orb.LineString{orb.Point{0, 0}, orb.Point{1, 0}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := idx.FindCrossingLocations(
		ctx, "A", "B", line, []string{"PRIMARY"}, 3, 10000, 1000)
	if err != context.Canceled {
		t.Errorf("err = %v, want %v", err, context.Canceled)
	}
	if res != nil {
		t.Errorf("res = %v, want nil", res)
	}
}

func TestFindClosestCrossing_ContextCancelled(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := idx.FindClosestCrossing(ctx, "A", "B", orb.Point{1, 1}, nil)
	if err != context.Canceled {
		t.Errorf("err = %v, want %v", err, context.Canceled)
	}
	if res != nil {
		t.Errorf("res = %v, want nil", res)
	}
}

func TestFindRegionPath_ContextCancelled(t *testing.T) {
	idx := newTestBorderIndex(types.BorderCrossingCollection{
		{FromRegion: "A", ToRegion: "B", RoadType: types.PRIMARY, Lon: 1, Lat: 1},
		{FromRegion: "B", ToRegion: "C", RoadType: types.PRIMARY, Lon: 2, Lat: 2},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	path, err := idx.FindRegionPath(ctx, "A", "C")
	if err != context.Canceled {
		t.Errorf("err = %v, want %v", err, context.Canceled)
	}
	if path != nil {
		t.Errorf("path = %v, want nil", path)
	}
}
