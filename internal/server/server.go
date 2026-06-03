// Package server implements the gRPC server for the region service.
//
// # Endpoints
//
// The region service provides geospatial query endpoints:
//   - SearchPoint: Find regions containing a coordinate
//   - SearchBox: Find regions intersecting a bounding box
//   - SearchRadius: Find regions within a radius of a point
//   - FindCrossingLocations: Find border crossings between two regions
//   - FindRegionPath: Find a path of regions from source to destination
//   - FindRouteRegionPaths: Find all corridor-constrained region paths for a polyline
//
// All RegionService endpoints require a user JWT or a service client token with
// the "region:query" scope. The health ping endpoint is public.
package server

import (
	"context"

	"github.com/paulmach/orb"
	regionv1 "github.com/swayrider/protos/region/v1"
	healthv1 "github.com/swayrider/protos/health/v1"
	log "github.com/swayrider/swlib/logger"
	"github.com/swayrider/swlib/security"
	"github.com/swayrider/regionservice/internal/index"
)

// RegionQuerier abstracts spatial region lookups.
// *index.RegionIndex satisfies this interface.
type RegionQuerier interface {
	SearchByPoint(p orb.Point, extended bool) []*index.RegionResult
	SearchByBox(bottomLeft, topRight orb.Point, extended bool) []*index.RegionResult
	SearchByRadius(center orb.Point, radiusKm float64, extended bool) []*index.RegionResult
}

// BorderQuerier abstracts border crossing lookups.
// *index.BorderIndex satisfies this interface.
type BorderQuerier interface {
	FindCrossingLocations(ctx context.Context, fromRegion, toRegion string, line orb.LineString, roadOrder []string, limit int, roadTypeDelta, dropDistance float64) []*index.BorderCrossingResult
	FindClosestCrossing(ctx context.Context, fromRegion, toRegion string, location orb.Point, validRoadTypes []string) *index.ClosestBorderCrossing
	FindRegionPath(ctx context.Context, fromRegion, toRegion string) []string
	FindRouteRegionPaths(ctx context.Context, fromRegion, toRegion string, allowedRegions map[string]bool) [][]string
}

// init registers endpoint authorization. All RegionService endpoints require a
// valid user JWT or a service client token with the "region:query" scope.
func init() {
	security.UserOrServiceEndpoint("/region.v1.RegionService/SearchPoint", []string{"region:query"})
	security.UserOrServiceEndpoint("/region.v1.RegionService/SearchBox", []string{"region:query"})
	security.UserOrServiceEndpoint("/region.v1.RegionService/SearchRadius", []string{"region:query"})
	security.UserOrServiceEndpoint("/region.v1.RegionService/FindCrossingLocations", []string{"region:query"})
	security.UserOrServiceEndpoint("/region.v1.RegionService/FindRegionPath", []string{"region:query"})
	security.UserOrServiceEndpoint("/region.v1.RegionService/FindRouteRegionPaths", []string{"region:query"})

	security.PublicEndpoint("/health.v1.HealthService/Ping")
}

// RegionServer implements the RegionService gRPC interface.
type RegionServer struct {
	regionv1.UnimplementedRegionServiceServer
	regionIndex RegionQuerier // Spatial index for region lookups
	borderIndex BorderQuerier // Index for border crossing lookups
	l           *log.Logger   // Logger instance
}

// NewRegionServer creates a new RegionServer with the given indices.
func NewRegionServer(
	regionIndex RegionQuerier,
	borderIndex BorderQuerier,
	l *log.Logger,
) *RegionServer {
	return &RegionServer{
		regionIndex: regionIndex,
		borderIndex: borderIndex,
		l: l.Derive(
			log.WithComponent("RegionServer"),
			log.WithFunction("NewRegionServer"),
		),
	}
}

// RegionIndex returns the server's region spatial index.
func (s RegionServer) RegionIndex() RegionQuerier {
	return s.regionIndex
}

// BorderIndex returns the server's border crossing index.
func (s RegionServer) BorderIndex() BorderQuerier {
	return s.borderIndex
}

// Logger returns the server's logger instance.
func (s RegionServer) Logger() *log.Logger {
	return s.l
}

// HealthServer implements the HealthService gRPC interface.
type HealthServer struct {
	healthv1.UnimplementedHealthServiceServer
	l *log.Logger
}

// NewHealthServer creates a new HealthServer with the given logger.
func NewHealthServer(
	l *log.Logger,
) *HealthServer {
	return &HealthServer{
		l: l.Derive(
			log.WithComponent("HealthServer"),
			log.WithFunction("NewHealthServer"),
		),
	}
}

// Logger returns the server's logger instance.
func (s HealthServer) Logger() *log.Logger {
	return s.l
}
