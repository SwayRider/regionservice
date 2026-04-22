// search_point.go implements the SearchPoint endpoint.

package server

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/paulmach/orb"
	regionv1 "github.com/swayrider/protos/region/v1"
	log "github.com/swayrider/swlib/logger"
)

// SearchPoint finds all regions containing a given coordinate.
//
// Parameters:
//   - Location: The coordinate to search (required)
//   - IncludeExtended: If true, also searches extended region boundaries
//
// Returns:
//   - CoreRegions: Region names where the point is within the core boundary
//   - ExtendedRegions: Region names where the point is only in the extended boundary
func (s *RegionServer) SearchPoint(
	ctx context.Context,
	req *regionv1.SearchPointRequest,
) (*regionv1.SearchPointResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("SearchPoint"))

	if req.Location == nil {
		lg.Debugln("Location must be set")
		return nil, status.Error(
			codes.InvalidArgument, "Location must be set",
		)
	}

	res := s.RegionIndex().SearchByPoint(
		orb.Point{req.Location.Lon, req.Location.Lat},
		req.IncludeExtended)
	if res == nil {
		lg.Infoln("No regions found")
		return &regionv1.SearchPointResponse{}, nil
	}

	coreRegions := make([]string, 0, len(res))
	extRegions := make([]string, 0, len(res))
	for _, item := range res {
		region := item.Region

		if item.IsExtended {
			extRegions = append(extRegions, region.Name())
		} else {
			coreRegions = append(coreRegions, region.Name())
		}
	}

	return &regionv1.SearchPointResponse{
		CoreRegions: coreRegions,
		ExtendedRegions: extRegions,
	}, nil
		
}

