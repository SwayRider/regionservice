// search_radius.go implements the SearchRadius endpoint.

package server

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/paulmach/orb"
	regionv1 "github.com/swayrider/protos/region/v1"
	log "github.com/swayrider/swlib/logger"
)

// SearchRadius finds all regions within a radius of a center point.
//
// Parameters:
//   - Location: The center point to search from (required)
//   - RadiusKm: The search radius in kilometers
//   - IncludeExtended: If true, also searches extended region boundaries
//
// Returns:
//   - CoreRegions: Region names within radius of the core boundary
//   - ExtendedRegions: Region names within radius of only the extended boundary
func (s *RegionServer) SearchRadius(
	ctx context.Context,
	req *regionv1.SearchRadiusRequest,
) (*regionv1.SearchRadiusResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("SearchRadius"))

	if req.Location == nil {
		lg.Debugln("Location must be set")
		return nil, status.Error(
			codes.InvalidArgument, "Location must be set",
		)
	}

	res := s.RegionIndex().SearchByRadius(
		orb.Point{req.Location.Lon, req.Location.Lat},
		req.RadiusKm,
		req.IncludeExtended)
	if res == nil {
		lg.Infoln("No regions found")
		return &regionv1.SearchRadiusResponse{}, nil
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

	return &regionv1.SearchRadiusResponse{
		CoreRegions: coreRegions,
		ExtendedRegions: extRegions,
	}, nil
}
