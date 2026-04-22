// find_crossing_locations.go implements the FindCrossingLocations endpoint.

package server

import (
	"context"
	"slices"
	"strings"

	"github.com/paulmach/orb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/swayrider/protos/common_types/geo"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/index"
	log "github.com/swayrider/swlib/logger"
)

// FindCrossingLocations finds border crossings between two regions.
//
// The endpoint supports two configuration modes:
//   - SimpleConfig: Fixed road type priority and distance thresholds
//   - AdvancedConfig: Distance-based configuration with multiple threshold definitions
//
// Parameters:
//   - FromRegion, ToRegion: The region pair to find crossings for
//   - FromLocation, ToLocation: Start and end coordinates of the route
//   - Limit: Maximum number of crossings to return
//   - ConfigOneof: Either SimpleConfig or AdvancedConfig
//
// Returns a list of border crossings sorted by road type priority and distance.
func (s *RegionServer) FindCrossingLocations(
	ctx context.Context,
	req *regionv1.FindCrossingLocationsRequest,
) (*regionv1.FindCrossingLocationsResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("FindCrossingLocations"))

	if req.FromRegion == "" {
		lg.Debugln("from_region must be set")
		return nil, status.Error(
			codes.InvalidArgument, "from_region must be set",
		)
	}
	if req.ToRegion == "" {
		lg.Debugln("to_region must be set")
		return nil, status.Error(
			codes.InvalidArgument, "to_region must be set",
		)
	}
	if req.FromLocation == nil {
		lg.Debugln("from_coordinate must be set")
		return nil, status.Error(
			codes.InvalidArgument, "from_coordinate must be set",
		)
	}
	if req.ToLocation == nil {
		lg.Debugln("to_coordinate must be set")
		return nil, status.Error(
			codes.InvalidArgument, "to_coordinate must be set",
		)
	}

	if req.ConfigOneof == nil {
		lg.Debugln("Config must be set")
		return nil, status.Error(
			codes.InvalidArgument, "Config must be set",
		)
	}

	switch v := req.ConfigOneof.(type) {
	case *regionv1.FindCrossingLocationsRequest_SimpleConfig:
		return s.findCrossingLocationsSimple(ctx, req, v.SimpleConfig)
	case *regionv1.FindCrossingLocationsRequest_AdvancedConfig:
		return s.findCrossingLocationsAdvanced(ctx, req, v.AdvancedConfig)
	default:
		lg.Debugln("Config must be set")
		return nil, status.Error(
			codes.InvalidArgument, "Config must be set",
		)
	}
}

// findCrossingLocationsSimple handles simple config border crossing searches.
// Uses fixed road type priority and distance thresholds.
func (s *RegionServer) findCrossingLocationsSimple(
	ctx context.Context,
	req *regionv1.FindCrossingLocationsRequest,
	cfg *regionv1.BorderCrossingSimpleConfig,
) (*regionv1.FindCrossingLocationsResponse, error) {
	if cfg.RoadTypeOrder == nil {
		cfg.RoadTypeOrder = []regionv1.RoadType{
			regionv1.RoadType_MOTORWAY,
			regionv1.RoadType_TRUNK,
			regionv1.RoadType_PRIMARY,
			regionv1.RoadType_SECONDARY,
		}
	}
	if req.Limit == 0 {
		req.Limit = 3
	}
	if cfg.RoadTypeDelta <= 0 {
		cfg.RoadTypeDelta = 10000
	}
	if cfg.DropDistance <= 0 {
		cfg.DropDistance = cfg.RoadTypeDelta * 0.1
	}

	line := orb.LineString{
		orb.Point{req.FromLocation.Lon, req.FromLocation.Lat},
		orb.Point{req.ToLocation.Lon, req.ToLocation.Lat},
	}

	roadTypeOrder := make([]string, 0, len(cfg.RoadTypeOrder))
	for _, item := range cfg.RoadTypeOrder {
		roadTypeOrder = append(
			roadTypeOrder, regionv1.RoadType_name[int32(item)])
	}
	res := s.BorderIndex().FindCrossingLocations(
		ctx,
		req.FromRegion, req.ToRegion,
		line, roadTypeOrder, int(req.Limit),
		cfg.RoadTypeDelta, cfg.DropDistance)
	
	resp := &regionv1.FindCrossingLocationsResponse{
		Crossings: make([]*regionv1.BorderCrossing, 0, len(res)),
	}
	for _, item := range res {
		resp.Crossings = append(resp.Crossings, &regionv1.BorderCrossing{
			FromRegion: item.BorderCrossing.FromRegion,
			ToRegion: item.BorderCrossing.ToRegion,
			RoadType: regionv1.RoadType(
				regionv1.RoadType_value[strings.ToUpper(
					item.BorderCrossing.RoadType.String())]),
			OsmId: int64(item.BorderCrossing.OsmId),
			Location: &geo.Coordinate {
				Lon: item.BorderCrossing.Location.X(),
				Lat: item.BorderCrossing.Location.Y(),
			},
		})
	}
	return resp, nil
}

// findCrossingLocationsAdvanced handles advanced config border crossing searches.
// Selects configuration based on distance to the closest crossing.
func (s *RegionServer) findCrossingLocationsAdvanced(
	ctx context.Context,
	req *regionv1.FindCrossingLocationsRequest,
	cfg *regionv1.BorderCrossingAdvancedConfig,
) (*regionv1.FindCrossingLocationsResponse, error) {
	if req.Limit == 0 {
		req.Limit = 3
	}

	line := orb.LineString{
		orb.Point{req.FromLocation.Lon, req.FromLocation.Lat},
		orb.Point{req.ToLocation.Lon, req.ToLocation.Lat},
	}
	refCrossing, err := closestCrossing(
		ctx, s.BorderIndex(), req.FromRegion, req.ToRegion, line)
	if err != nil {
		return nil, err
	}
	cfgDef := findCrossingDefinition(
		refCrossing.Distance, cfg.Definitions)
	if cfgDef == nil {
		return nil, status.Error(
			codes.NotFound, "No definition found")
	}

	if cfgDef.RoadTypeDelta <= 0 {
		cfgDef.RoadTypeDelta = refCrossing.Distance
	}
	if cfgDef.DropDistance <= 0 {
		cfgDef.DropDistance = cfgDef.RoadTypeDelta * 0.1
	}

	roadTypeOrder := make([]string, 0, len(cfgDef.RoadTypeOrder))
	for _, item := range cfgDef.RoadTypeOrder {
		roadTypeOrder = append(
			roadTypeOrder, regionv1.RoadType_name[int32(item)])
	}

	res := s.BorderIndex().FindCrossingLocations(
		ctx,
		req.FromRegion, req.ToRegion,
		line, roadTypeOrder, int(req.Limit),
		cfgDef.RoadTypeDelta, cfgDef.DropDistance)
	
	resp := &regionv1.FindCrossingLocationsResponse{
		Crossings: make([]*regionv1.BorderCrossing, 0, len(res)),
	}
	for _, item := range res {
		resp.Crossings = append(resp.Crossings, &regionv1.BorderCrossing{
			FromRegion: item.BorderCrossing.FromRegion,
			ToRegion: item.BorderCrossing.ToRegion,
			RoadType: regionv1.RoadType(
				regionv1.RoadType_value[strings.ToUpper(
					item.BorderCrossing.RoadType.String())]),
			OsmId: int64(item.BorderCrossing.OsmId),
			Location: &geo.Coordinate {
				Lon: item.BorderCrossing.Location.X(),
				Lat: item.BorderCrossing.Location.Y(),
			},
		})
	}
	return resp, nil
}

// closestCrossing finds the closest border crossing to either endpoint of a line.
// Returns the crossing that is closer to the line's start or end point.
func closestCrossing(
	ctx context.Context,
	borderIndex *index.BorderIndex,
	fromRegion, toRegion string,
	line orb.LineString,
) (*index.ClosestBorderCrossing, error) {
	closesForwardCrossing := borderIndex.FindClosestCrossing(
		ctx, fromRegion, toRegion, line[0], nil)
	if closesForwardCrossing == nil {
		return nil, status.Error(
			codes.NotFound, "No forward crossing found")
	}
	closesBackwardCrossing := borderIndex.FindClosestCrossing(
		ctx, fromRegion, toRegion, line[1], nil)
	if closesBackwardCrossing == nil {
		return nil, status.Error(
			codes.NotFound, "No backward crossing found")
	}

	if closesForwardCrossing.Distance < closesBackwardCrossing.Distance {
		return closesForwardCrossing, nil
	}
	return closesBackwardCrossing, nil
}

// findCrossingDefinition selects the appropriate crossing definition based on distance.
// Definitions are sorted by MaxBorderDistance and the first matching definition is returned.
func findCrossingDefinition(
	refDistance float64,
	definitions []*regionv1.BorderCrossingDefinition,
) *regionv1.BorderCrossingDefinition {
	slices.SortFunc(
		definitions, func(a, b *regionv1.BorderCrossingDefinition) int {
			return int(a.MaxBorderDistance - b.MaxBorderDistance)
		})

	for i := 1; i < len(definitions); i++ {
		if refDistance <= definitions[i].MaxBorderDistance {
			return definitions[i]
		}
	}
	return definitions[0]

}
