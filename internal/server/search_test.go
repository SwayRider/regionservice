package server

import (
	"context"
	"testing"

	"github.com/paulmach/orb"
	"github.com/swayrider/regionservice/internal/index"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/protos/common_types/geo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeRegion builds a minimal *index.Region with the given name.
// It avoids constructing real GeoShapes by using the exported NewRegion constructor.
func fakeRegion(name string) *index.Region {
	return index.NewRegion(name, nil, nil)
}

func fakeRegionResult(name string, isExtended bool) *index.RegionResult {
	return &index.RegionResult{
		Region:     fakeRegion(name),
		IsExtended: isExtended,
	}
}

// =============================================================================
// SearchPoint Tests
// =============================================================================

func TestSearchPoint_MissingLocation(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.SearchPoint(context.Background(), &regionv1.SearchPointRequest{})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestSearchPoint_NilIndexResult(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	resp, err := s.SearchPoint(context.Background(), &regionv1.SearchPointRequest{
		Location: &geo.Coordinate{Lon: 5, Lat: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CoreRegions) != 0 || len(resp.ExtendedRegions) != 0 {
		t.Errorf("expected empty response, got %v", resp)
	}
}

func TestSearchPoint_CoreAndExtendedSplit(t *testing.T) {
	rq := &mockRegionQuerier{
		searchByPointFn: func(p orb.Point, extended bool) []*index.RegionResult {
			return []*index.RegionResult{
				fakeRegionResult("France", false),
				fakeRegionResult("Germany", false),
				fakeRegionResult("Luxembourg", true),
			}
		},
	}
	s := newTestRegionServer(rq, &mockBorderQuerier{})
	resp, err := s.SearchPoint(context.Background(), &regionv1.SearchPointRequest{
		Location:        &geo.Coordinate{Lon: 5, Lat: 50},
		IncludeExtended: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CoreRegions) != 2 {
		t.Errorf("CoreRegions len = %d, want 2", len(resp.CoreRegions))
	}
	if len(resp.ExtendedRegions) != 1 {
		t.Errorf("ExtendedRegions len = %d, want 1", len(resp.ExtendedRegions))
	}
	if resp.ExtendedRegions[0] != "Luxembourg" {
		t.Errorf("ExtendedRegions[0] = %q, want %q", resp.ExtendedRegions[0], "Luxembourg")
	}
}

// =============================================================================
// SearchBox Tests
// =============================================================================

func TestSearchBox_MissingBox(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.SearchBox(context.Background(), &regionv1.SearchBoxRequest{})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestSearchBox_MissingBottomLeft(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.SearchBox(context.Background(), &regionv1.SearchBoxRequest{
		Box: &geo.BoundingBox{
			TopRight: &geo.Coordinate{Lon: 10, Lat: 10},
		},
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestSearchBox_MissingTopRight(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.SearchBox(context.Background(), &regionv1.SearchBoxRequest{
		Box: &geo.BoundingBox{
			BottomLeft: &geo.Coordinate{Lon: 0, Lat: 0},
		},
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestSearchBox_Success(t *testing.T) {
	rq := &mockRegionQuerier{
		searchByBoxFn: func(bottomLeft, topRight orb.Point, extended bool) []*index.RegionResult {
			return []*index.RegionResult{
				fakeRegionResult("Belgium", false),
				fakeRegionResult("Netherlands", true),
			}
		},
	}
	s := newTestRegionServer(rq, &mockBorderQuerier{})
	resp, err := s.SearchBox(context.Background(), &regionv1.SearchBoxRequest{
		Box: &geo.BoundingBox{
			BottomLeft: &geo.Coordinate{Lon: 3, Lat: 49},
			TopRight:   &geo.Coordinate{Lon: 7, Lat: 53},
		},
		IncludeExtended: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CoreRegions) != 1 || resp.CoreRegions[0] != "Belgium" {
		t.Errorf("CoreRegions = %v, want [Belgium]", resp.CoreRegions)
	}
	if len(resp.ExtendedRegions) != 1 || resp.ExtendedRegions[0] != "Netherlands" {
		t.Errorf("ExtendedRegions = %v, want [Netherlands]", resp.ExtendedRegions)
	}
}

// =============================================================================
// SearchRadius Tests
// =============================================================================

func TestSearchRadius_MissingLocation(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	_, err := s.SearchRadius(context.Background(), &regionv1.SearchRadiusRequest{
		RadiusKm: 100,
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", code, codes.InvalidArgument)
	}
}

func TestSearchRadius_Success(t *testing.T) {
	rq := &mockRegionQuerier{
		searchByRadiusFn: func(center orb.Point, radiusKm float64, extended bool) []*index.RegionResult {
			return []*index.RegionResult{
				fakeRegionResult("Austria", false),
			}
		},
	}
	s := newTestRegionServer(rq, &mockBorderQuerier{})
	resp, err := s.SearchRadius(context.Background(), &regionv1.SearchRadiusRequest{
		Location: &geo.Coordinate{Lon: 14, Lat: 48},
		RadiusKm: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CoreRegions) != 1 || resp.CoreRegions[0] != "Austria" {
		t.Errorf("CoreRegions = %v, want [Austria]", resp.CoreRegions)
	}
}

func TestSearchRadius_NilIndexResult(t *testing.T) {
	s := newTestRegionServer(&mockRegionQuerier{}, &mockBorderQuerier{})
	resp, err := s.SearchRadius(context.Background(), &regionv1.SearchRadiusRequest{
		Location: &geo.Coordinate{Lon: 0, Lat: 0},
		RadiusKm: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.CoreRegions) != 0 || len(resp.ExtendedRegions) != 0 {
		t.Errorf("expected empty response, got %v", resp)
	}
}
