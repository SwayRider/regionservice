// find_crossing_locations.go implements the FindCrossingLocations endpoint.

package server

import (
	"context"
	"slices"
	"strings"

	"github.com/paulmach/orb"
	"github.com/swayrider/protos/common_types/geo"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/regionservice/internal/types"
	log "github.com/swayrider/swlib/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if err := validateCoordinate(req.FromLocation, "from_location"); err != nil {
		return nil, err
	}
	if err := validateCoordinate(req.ToLocation, "to_location"); err != nil {
		return nil, err
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
// Uses fixed road type priority and distance thresholds. Defaults are computed
// into locals so the request's config is never mutated.
func (s *RegionServer) findCrossingLocationsSimple(
	ctx context.Context,
	req *regionv1.FindCrossingLocationsRequest,
	cfg *regionv1.BorderCrossingSimpleConfig,
) (*regionv1.FindCrossingLocationsResponse, error) {
	roadTypeOrder := cfg.RoadTypeOrder
	if roadTypeOrder == nil {
		roadTypeOrder = []regionv1.RoadType{
			regionv1.RoadType_MOTORWAY,
			regionv1.RoadType_TRUNK,
			regionv1.RoadType_PRIMARY,
			regionv1.RoadType_SECONDARY,
		}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}
	roadTypeDelta := cfg.RoadTypeDelta
	if roadTypeDelta <= 0 {
		roadTypeDelta = 10000
	}
	dropDistance := cfg.DropDistance
	if dropDistance <= 0 {
		dropDistance = roadTypeDelta * 0.1
	}

	line := orb.LineString{
		orb.Point{req.FromLocation.Lon, req.FromLocation.Lat},
		orb.Point{req.ToLocation.Lon, req.ToLocation.Lat},
	}

	res, err := s.BorderIndex().FindCrossingLocations(
		ctx,
		req.FromRegion, req.ToRegion,
		line, roadTypeOrderStrings(roadTypeOrder), int(limit),
		roadTypeDelta, dropDistance)
	if err != nil {
		return nil, err
	}

	return buildCrossingsResponse(res), nil
}

// findCrossingLocationsAdvanced handles advanced config border crossing searches.
// Selects configuration based on distance to the closest crossing. Defaults are
// computed into locals so the request's definitions are never mutated.
func (s *RegionServer) findCrossingLocationsAdvanced(
	ctx context.Context,
	req *regionv1.FindCrossingLocationsRequest,
	cfg *regionv1.BorderCrossingAdvancedConfig,
) (*regionv1.FindCrossingLocationsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 3
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

	roadTypeDelta := cfgDef.RoadTypeDelta
	if roadTypeDelta <= 0 {
		roadTypeDelta = refCrossing.Distance
	}
	dropDistance := cfgDef.DropDistance
	if dropDistance <= 0 {
		dropDistance = roadTypeDelta * 0.1
	}

	res, err := s.BorderIndex().FindCrossingLocations(
		ctx,
		req.FromRegion, req.ToRegion,
		line, roadTypeOrderStrings(cfgDef.RoadTypeOrder), int(limit),
		roadTypeDelta, dropDistance)
	if err != nil {
		return nil, err
	}

	return buildCrossingsResponse(res), nil
}

// closestCrossing finds the closest border crossing to either endpoint of a line.
// Returns the crossing that is closer to the line's start or end point.
func closestCrossing(
	ctx context.Context,
	borderIndex BorderQuerier,
	fromRegion, toRegion string,
	line orb.LineString,
) (*index.ClosestBorderCrossing, error) {
	closestForwardCrossing, err := borderIndex.FindClosestCrossing(
		ctx, fromRegion, toRegion, line[0], nil)
	if err != nil {
		return nil, err
	}
	if closestForwardCrossing == nil {
		return nil, status.Error(
			codes.NotFound, "No forward crossing found")
	}
	closestBackwardCrossing, err := borderIndex.FindClosestCrossing(
		ctx, fromRegion, toRegion, line[1], nil)
	if err != nil {
		return nil, err
	}
	if closestBackwardCrossing == nil {
		return nil, status.Error(
			codes.NotFound, "No backward crossing found")
	}

	if closestForwardCrossing.Distance < closestBackwardCrossing.Distance {
		return closestForwardCrossing, nil
	}
	return closestBackwardCrossing, nil
}

// findCrossingDefinition selects the crossing definition that applies at the
// given reference distance. Definitions are ranked by MaxBorderDistance; a
// definition applies when the reference distance is at or below its
// MaxBorderDistance. A definition with MaxBorderDistance == 0 is the explicit
// fallback for distances beyond every non-zero definition (see the proto
// comment); when no such fallback is present and the distance exceeds all
// non-zero definitions, nil is returned so the caller can report NotFound.
//
// The input slice is not mutated: it is cloned before sorting.
func findCrossingDefinition(
	refDistance float64,
	definitions []*regionv1.BorderCrossingDefinition,
) *regionv1.BorderCrossingDefinition {
	defs := slices.Clone(definitions)
	slices.SortFunc(defs, func(a, b *regionv1.BorderCrossingDefinition) int {
		switch {
		case a.MaxBorderDistance < b.MaxBorderDistance:
			return -1
		case a.MaxBorderDistance > b.MaxBorderDistance:
			return 1
		default:
			return 0
		}
	})

	// First non-zero definition whose max covers the reference distance.
	for _, d := range defs {
		if d.MaxBorderDistance == 0 {
			continue // 0 marks the fallback, never a band selection.
		}
		if refDistance <= d.MaxBorderDistance {
			return d
		}
	}

	// Distance exceeds every non-zero definition: use the explicit 0-max
	// fallback when present, otherwise nil (caller reports NotFound).
	for _, d := range defs {
		if d.MaxBorderDistance == 0 {
			return d
		}
	}
	return nil
}

// roadTypeOrderStrings converts a proto road-type priority list into the
// string form expected by the border index, skipping any unknown enum values
// (which would otherwise map to an empty string and silently match crossings
// with no road type).
func roadTypeOrderStrings(order []regionv1.RoadType) []string {
	result := make([]string, 0, len(order))
	for _, item := range order {
		if name, ok := regionv1.RoadType_name[int32(item)]; ok {
			result = append(result, name)
		}
	}
	return result
}

// roadTypeToProto converts an index road type to the proto enum, reporting
// whether the road type is recognized. Unknown road types (which previously
// fell through to the MOTORWAY zero value) are reported as !ok so callers can
// skip them instead of mislabeling them.
func roadTypeToProto(rt types.RoadType) (regionv1.RoadType, bool) {
	name := strings.ToUpper(types.RoadTypeFromString(rt.String()).String())
	v, ok := regionv1.RoadType_value[name]
	if !ok {
		return 0, false
	}
	return regionv1.RoadType(v), true
}

// buildCrossingsResponse maps index crossing results to the proto response,
// skipping crossings whose road type is not a known proto enum.
func buildCrossingsResponse(
	res []*index.BorderCrossingResult,
) *regionv1.FindCrossingLocationsResponse {
	resp := &regionv1.FindCrossingLocationsResponse{
		Crossings: make([]*regionv1.BorderCrossing, 0, len(res)),
	}
	for _, item := range res {
		roadType, ok := roadTypeToProto(item.BorderCrossing.RoadType)
		if !ok {
			continue
		}
		resp.Crossings = append(resp.Crossings, &regionv1.BorderCrossing{
			FromRegion: item.BorderCrossing.FromRegion,
			ToRegion:   item.BorderCrossing.ToRegion,
			RoadType:   roadType,
			OsmId:      int64(item.BorderCrossing.OsmId),
			Location: &geo.Coordinate{
				Lon: item.BorderCrossing.Location.X(),
				Lat: item.BorderCrossing.Location.Y(),
			},
		})
	}
	return resp
}
