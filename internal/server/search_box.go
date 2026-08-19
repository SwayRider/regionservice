// search_box.go implements the SearchBox endpoint.

package server

import (
	"context"

	"github.com/paulmach/orb"
	regionv1 "github.com/swayrider/protos/region/v1"
	log "github.com/swayrider/swlib/logger"
)

// SearchBox finds all regions that intersect a bounding box.
//
// Parameters:
//   - Box: The bounding box defined by BottomLeft and TopRight corners (required)
//   - IncludeExtended: If true, also searches extended region boundaries
//
// Handles bounding boxes that cross the antimeridian (180° longitude).
//
// Returns:
//   - CoreRegions: Region names that intersect within the core boundary
//   - ExtendedRegions: Region names that only intersect the extended boundary
func (s *RegionServer) SearchBox(
	ctx context.Context,
	req *regionv1.SearchBoxRequest,
) (*regionv1.SearchBoxResponse, error) {
	lg := s.Logger().Derive(log.WithFunction("SearchBox"))

	if err := validateBox(req.Box); err != nil {
		return nil, err
	}

	res := s.RegionIndex().SearchByBox(
		orb.Point{req.Box.BottomLeft.Lon, req.Box.BottomLeft.Lat},
		orb.Point{req.Box.TopRight.Lon, req.Box.TopRight.Lat},
		req.IncludeExtended)
	if res == nil {
		lg.Infoln("No regions found")
		return &regionv1.SearchBoxResponse{}, nil
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

	return &regionv1.SearchBoxResponse{
		CoreRegions: coreRegions,
		ExtendedRegions: extRegions,
	}, nil
}
