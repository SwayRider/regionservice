package server

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/paulmach/orb"
	log "github.com/swayrider/swlib/logger"
	"github.com/swayrider/regionservice/internal/index"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func newTestRegionServer(rq RegionQuerier, bq BorderQuerier) *RegionServer {
	return NewRegionServer(rq, bq, log.New())
}

func newTestHealthServer() *HealthServer {
	return NewHealthServer(log.New())
}

// =============================================================================
// mockRegionQuerier
// =============================================================================

type mockRegionQuerier struct {
	searchByPointFn  func(p orb.Point, extended bool) []*index.RegionResult
	searchByBoxFn    func(bottomLeft, topRight orb.Point, extended bool) []*index.RegionResult
	searchByRadiusFn func(center orb.Point, radiusKm float64, extended bool) []*index.RegionResult
}

func (m *mockRegionQuerier) SearchByPoint(p orb.Point, extended bool) []*index.RegionResult {
	if m.searchByPointFn != nil {
		return m.searchByPointFn(p, extended)
	}
	return nil
}

func (m *mockRegionQuerier) SearchByBox(bottomLeft, topRight orb.Point, extended bool) []*index.RegionResult {
	if m.searchByBoxFn != nil {
		return m.searchByBoxFn(bottomLeft, topRight, extended)
	}
	return nil
}

func (m *mockRegionQuerier) SearchByRadius(center orb.Point, radiusKm float64, extended bool) []*index.RegionResult {
	if m.searchByRadiusFn != nil {
		return m.searchByRadiusFn(center, radiusKm, extended)
	}
	return nil
}

// =============================================================================
// mockBorderQuerier
// =============================================================================

type mockBorderQuerier struct {
	findCrossingLocationsFn func(ctx context.Context, fromRegion, toRegion string, line orb.LineString, roadOrder []string, limit int, roadTypeDelta, dropDistance float64) []*index.BorderCrossingResult
	findClosestCrossingFn   func(ctx context.Context, fromRegion, toRegion string, location orb.Point, validRoadTypes []string) *index.ClosestBorderCrossing
	findRegionPathFn        func(ctx context.Context, fromRegion, toRegion string) []string
	findRouteRegionPathsFn  func(ctx context.Context, fromRegion, toRegion string, allowedRegions map[string]bool) [][]string
}

func (m *mockBorderQuerier) FindCrossingLocations(ctx context.Context, fromRegion, toRegion string, line orb.LineString, roadOrder []string, limit int, roadTypeDelta, dropDistance float64) []*index.BorderCrossingResult {
	if m.findCrossingLocationsFn != nil {
		return m.findCrossingLocationsFn(ctx, fromRegion, toRegion, line, roadOrder, limit, roadTypeDelta, dropDistance)
	}
	return nil
}

func (m *mockBorderQuerier) FindClosestCrossing(ctx context.Context, fromRegion, toRegion string, location orb.Point, validRoadTypes []string) *index.ClosestBorderCrossing {
	if m.findClosestCrossingFn != nil {
		return m.findClosestCrossingFn(ctx, fromRegion, toRegion, location, validRoadTypes)
	}
	return nil
}

func (m *mockBorderQuerier) FindRegionPath(ctx context.Context, fromRegion, toRegion string) []string {
	if m.findRegionPathFn != nil {
		return m.findRegionPathFn(ctx, fromRegion, toRegion)
	}
	return nil
}

func (m *mockBorderQuerier) FindRouteRegionPaths(ctx context.Context, fromRegion, toRegion string, allowedRegions map[string]bool) [][]string {
	if m.findRouteRegionPathsFn != nil {
		return m.findRouteRegionPathsFn(ctx, fromRegion, toRegion, allowedRegions)
	}
	return nil
}
