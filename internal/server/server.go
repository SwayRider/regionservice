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
//
// All endpoints are registered as public (no authentication required).
package server

import (
	regionv1 "github.com/swayrider/protos/region/v1"
	healthv1 "github.com/swayrider/protos/health/v1"
	log "github.com/swayrider/swlib/logger"
	"github.com/swayrider/swlib/security"
	"github.com/swayrider/regionservice/internal/index"
)

// init registers all endpoints as public (no authentication required).
func init() {
	security.PublicEndpoint("/region.v1.RegionService/SearchPoint")
	security.PublicEndpoint("/region.v1.RegionService/SearchBox")
	security.PublicEndpoint("/region.v1.RegionService/SearchRadius")
	security.PublicEndpoint("/region.v1.RegionService/FindCrossingLocations")
	security.PublicEndpoint("/region.v1.RegionService/FindRegionPath")

	security.PublicEndpoint("/health.v1.HealthService/Ping")
}

// RegionServer implements the RegionService gRPC interface.
type RegionServer struct {
	regionv1.UnimplementedRegionServiceServer
	regionIndex *index.RegionIndex // Spatial index for region lookups
	borderIndex *index.BorderIndex // Index for border crossing lookups
	l           *log.Logger        // Logger instance
}

// NewRegionServer creates a new RegionServer with the given indices.
func NewRegionServer(
	regionIndex *index.RegionIndex,
	borderIndex *index.BorderIndex,
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
func (s RegionServer) RegionIndex() *index.RegionIndex {
	return s.regionIndex
}

// BorderIndex returns the server's border crossing index.
func (s RegionServer) BorderIndex() *index.BorderIndex {
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
