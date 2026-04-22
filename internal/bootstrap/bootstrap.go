// Package bootstrap handles the initialization of geodata indices from the filesystem.
//
// This package is responsible for reading region contours and border crossing
// data from local geodata files during service startup, and populating the spatial
// indices used for geospatial queries.
package bootstrap

import (
	"context"

	"github.com/swayrider/regionservice/internal/geodata"
	"github.com/swayrider/regionservice/internal/index"
)

// Bootstrap loads geodata from the filesystem and populates the spatial indices.
//
// The function performs the following steps:
//  1. Retrieves the manifest file from the geodata directory
//  2. For each region in the manifest, reads core and extended contours
//  3. Adds each region's contours to the RegionIndex
//  4. Reads border crossing data and adds it to the BorderIndex
//
// Parameters:
//   - reader: GeoDataReader for retrieving geodata files
//   - regionIndex: Index to populate with region geometries
//   - borderIndex: Index to populate with border crossings
func Bootstrap(
	reader *geodata.GeoDataReader,
	regionIndex *index.RegionIndex,
	borderIndex *index.BorderIndex,
) error {
	ctx := context.Background()
	manifest, err := reader.GetManifest(ctx)
	if err != nil {
		return err
	}

	for regionName, region := range manifest.Regions {
		coreDesc := region.Contour.Core
		coreFC, err := reader.GetContour(ctx, coreDesc)
		if err != nil {
			return err
		}

		extendedDesc := region.Contour.Extended
		extendedFC, err := reader.GetContour(ctx, extendedDesc)
		if err != nil {
			return err
		}

		regionIndex.Add(regionName, coreFC, extendedFC)
	}

	for _, crossing := range manifest.Shared.BorderCrossings {
		border, err := reader.GetBorderCrossing(ctx, crossing)
		if err != nil {
			return err
		}
		borderIndex.Add(border)
	}

	return nil
}
