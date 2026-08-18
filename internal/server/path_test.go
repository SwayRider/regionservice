package server

import (
	"context"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/protos/common_types/geo"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/index"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// FindRegionPath Tests
// =============================================================================

func TestFindRegionPath_MissingFrom(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.FindRegionPath(context.Background(), &regionv1.FindRegionPathRequest{
		ToRegion: "B",
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestFindRegionPath_MissingTo(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.FindRegionPath(context.Background(), &regionv1.FindRegionPathRequest{
		FromRegion: "A",
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestFindRegionPath_Success(t *testing.T) {
	bq := &mockBorderQuerier{
		findRegionPathFn: func(_ context.Context, from, to string) []string {
			return []string{"A", "B", "C"}
		},
	}
	s := newTestRegionServer(&mockRegionQuerier{}, bq)
	resp, err := s.FindRegionPath(context.Background(), &regionv1.FindRegionPathRequest{
		FromRegion: "A",
		ToRegion:   "C",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Path) != 3 || resp.Path[0] != "A" || resp.Path[2] != "C" {
		t.Errorf("Path = %v, want [A B C]", resp.Path)
	}
}

func TestFindRegionPath_NoPath(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	resp, err := s.FindRegionPath(context.Background(), &regionv1.FindRegionPathRequest{
		FromRegion: "A",
		ToRegion:   "Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Path) != 0 {
		t.Errorf("expected empty path, got %v", resp.Path)
	}
}

// =============================================================================
// FindRouteRegionPaths Tests
// =============================================================================

func TestFindRouteRegionPaths_TooFewWaypoints(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}},
		WidthKm:   10,
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestFindRouteRegionPaths_ZeroWidth(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}},
		WidthKm:   0,
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestFindRouteRegionPaths_NegativeWidth(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}},
		WidthKm:   -5,
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestFindRouteRegionPaths_NoStartRegion(t *testing.T) {
	// SearchByPoint returns empty for start → empty response
	rq := &mockRegionQuerier{
		searchByBoxFn: func(bl, tr orb.Point, ext bool) []*index.RegionResult {
			return []*index.RegionResult{fakeRegionResult("X", false)}
		},
		// searchByPointFn nil → returns nil
	}
	s := newTestRegionServer(rq, &mockBorderQuerier{})
	resp, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}},
		WidthKm:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Paths) != 0 {
		t.Errorf("expected empty paths, got %v", resp.Paths)
	}
}

func TestFindRouteRegionPaths_NoEndRegion(t *testing.T) {
	callCount := 0
	rq := &mockRegionQuerier{
		searchByBoxFn: func(bl, tr orb.Point, ext bool) []*index.RegionResult {
			return []*index.RegionResult{fakeRegionResult("X", false)}
		},
		searchByPointFn: func(p orb.Point, ext bool) []*index.RegionResult {
			callCount++
			if callCount == 1 {
				return []*index.RegionResult{fakeRegionResult("Start", false)}
			}
			return nil // end region not found
		},
	}
	s := newTestRegionServer(rq, &mockBorderQuerier{})
	resp, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}},
		WidthKm:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Paths) != 0 {
		t.Errorf("expected empty paths, got %v", resp.Paths)
	}
}

func TestFindRouteRegionPaths_Success(t *testing.T) {
	rq := &mockRegionQuerier{
		searchByBoxFn: func(bl, tr orb.Point, ext bool) []*index.RegionResult {
			return []*index.RegionResult{
				fakeRegionResult("A", false),
				fakeRegionResult("B", false),
				fakeRegionResult("C", false),
			}
		},
		searchByPointFn: func(p orb.Point, ext bool) []*index.RegionResult {
			if p[0] == 0 {
				return []*index.RegionResult{fakeRegionResult("A", false)}
			}
			return []*index.RegionResult{fakeRegionResult("C", false)}
		},
	}
	bq := &mockBorderQuerier{
		findRouteRegionPathsFn: func(_ context.Context, from, to string, allowed map[string]bool) ([][]string, error) {
			return [][]string{{"A", "B", "C"}}, nil
		},
	}
	s := newTestRegionServer(rq, bq)
	resp, err := s.FindRouteRegionPaths(context.Background(), &regionv1.FindRouteRegionPathsRequest{
		Waypoints: []*geo.Coordinate{{Lon: 0, Lat: 0}, {Lon: 1, Lat: 1}},
		WidthKm:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resp.Paths))
	}
	if len(resp.Paths[0].Regions) != 3 {
		t.Errorf("path length = %d, want 3", len(resp.Paths[0].Regions))
	}
}
