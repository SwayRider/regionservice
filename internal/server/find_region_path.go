// find_region_path.go implements the FindRegionPath endpoint.

package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	regionv1 "github.com/swayrider/protos/region/v1"
	log "github.com/swayrider/swlib/logger"
)

// FindRegionPath finds a path of regions from source to destination.
//
// Uses breadth-first search through the border crossing graph to find
// the shortest sequence of regions that connect the two endpoints.
//
// Parameters:
//   - FromRegion: The starting region name (required)
//   - ToRegion: The destination region name (required)
//
// Returns:
//   - Path: Ordered list of region names from source to destination (inclusive)
//   - Empty response if no path exists
func (s *RegionServer) FindRegionPath(
	ctx context.Context,
	req *regionv1.FindRegionPathRequest,
) (*regionv1.FindRegionPathResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("FindRegionPath"))

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

	res := s.BorderIndex().FindRegionPath(ctx, req.FromRegion, req.ToRegion)
	if res == nil {
		lg.Infoln("No regions found")
		return &regionv1.FindRegionPathResponse{}, nil
	}

	return &regionv1.FindRegionPathResponse{
		Path: res,
	}, nil
}
