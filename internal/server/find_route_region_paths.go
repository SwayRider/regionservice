// find_route_region_paths.go implements the FindRouteRegionPaths endpoint.

package server

import (
	"context"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	regionv1 "github.com/swayrider/protos/region/v1"
	log "github.com/swayrider/swlib/logger"
)

// FindRouteRegionPaths finds all region paths through a geographic corridor.
//
// The corridor is defined by a polyline of waypoints expanded by width_km on
// each side. For each segment, the axis-aligned bounding box of the expanded
// corridor is computed, and all core regions intersecting any box are collected.
// A DFS through the border crossing graph, restricted to those regions, returns
// every acyclic path from the start region to the end region.
//
// Parameters:
//   - Waypoints: Ordered list of coordinates defining the route (minimum 2)
//   - WidthKm: Total corridor width in km (expanded width_km/2 on each side)
//
// Returns:
//   - Paths: All acyclic region paths through the corridor
//   - Empty response if no path exists or start/end point is outside all regions
func (s *RegionServer) FindRouteRegionPaths(
	ctx context.Context,
	req *regionv1.FindRouteRegionPathsRequest,
) (*regionv1.FindRouteRegionPathsResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("FindRouteRegionPaths"))

	if len(req.Waypoints) < 2 {
		return nil, status.Error(codes.InvalidArgument, "at least 2 waypoints required")
	}
	if req.WidthKm <= 0 {
		return nil, status.Error(codes.InvalidArgument, "width_km must be positive")
	}

	allowedRegions := make(map[string]bool)
	for idx := 0; idx < len(req.Waypoints)-1; idx++ {
		p1 := orb.Point{req.Waypoints[idx].Lon, req.Waypoints[idx].Lat}
		p2 := orb.Point{req.Waypoints[idx+1].Lon, req.Waypoints[idx+1].Lat}
		bl, tr := computeCorridorBox(p1, p2, req.WidthKm)
		for _, r := range s.RegionIndex().SearchByBox(bl, tr, false) {
			if !r.IsExtended {
				allowedRegions[r.Region.Name()] = true
			}
		}
	}

	first := req.Waypoints[0]
	last := req.Waypoints[len(req.Waypoints)-1]

	startRes := s.RegionIndex().SearchByPoint(orb.Point{first.Lon, first.Lat}, false)
	if len(startRes) == 0 {
		lg.Infoln("No start region found")
		return &regionv1.FindRouteRegionPathsResponse{}, nil
	}

	endRes := s.RegionIndex().SearchByPoint(orb.Point{last.Lon, last.Lat}, false)
	if len(endRes) == 0 {
		lg.Infoln("No end region found")
		return &regionv1.FindRouteRegionPathsResponse{}, nil
	}

	fromRegion := startRes[0].Region.Name()
	toRegion := endRes[0].Region.Name()

	rawPaths := s.BorderIndex().FindRouteRegionPaths(ctx, fromRegion, toRegion, allowedRegions)
	if len(rawPaths) == 0 {
		lg.Infoln("No route region paths found")
		return &regionv1.FindRouteRegionPathsResponse{}, nil
	}

	paths := make([]*regionv1.RegionPath, 0, len(rawPaths))
	for _, p := range rawPaths {
		paths = append(paths, &regionv1.RegionPath{Regions: p})
	}
	return &regionv1.FindRouteRegionPathsResponse{Paths: paths}, nil
}

// computeCorridorBox returns the axis-aligned bounding box for the corridor
// around the line segment from p1 to p2, expanded by widthKm/2 on each side.
// Mirrors the pattern used in RegionIndex.SearchByRadius.
func computeCorridorBox(p1, p2 orb.Point, widthKm float64) (bottomLeft, topRight orb.Point) {
	r := (widthKm / 2) * 1000
	minLon := min(geo.PointAtBearingAndDistance(p1, 270, r)[0],
		geo.PointAtBearingAndDistance(p2, 270, r)[0])
	maxLon := max(geo.PointAtBearingAndDistance(p1, 90, r)[0],
		geo.PointAtBearingAndDistance(p2, 90, r)[0])
	minLat := min(geo.PointAtBearingAndDistance(p1, 180, r)[1],
		geo.PointAtBearingAndDistance(p2, 180, r)[1])
	maxLat := max(geo.PointAtBearingAndDistance(p1, 0, r)[1],
		geo.PointAtBearingAndDistance(p2, 0, r)[1])
	return orb.Point{minLon, minLat}, orb.Point{maxLon, maxLat}
}
