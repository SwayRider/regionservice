// border_crossing.go defines types for indexing border crossing points.

package index

import (
	"github.com/swayrider/regionservice/internal/types"
	"github.com/paulmach/orb"
	"github.com/dhconnelly/rtreego"
)

// BorderCrossing represents a road crossing point between two regions.
// Contains location and metadata for routing decisions.
type BorderCrossing struct {
	FromRegion string         // Source region name
	ToRegion   string         // Destination region name
	OsmId      int            // OpenStreetMap way ID
	RoadType   types.RoadType // Road classification (motorway, trunk, etc.)
	Location   orb.Point      // Geographic coordinates (lon, lat)
}

// SpatialborderCrossing wraps a BorderCrossing for R-tree spatial indexing.
type SpatialborderCrossing struct {
	Crossing *BorderCrossing // Reference to the border crossing
	bbox     *rtreego.Rect   // Point bounding box for the R-tree
}

// NewSpatialBorderCrossing creates a spatial index entry for a border crossing.
// The bounding box is a point-sized rectangle at the crossing location.
func NewSpatialBorderCrossing(
	crossing *BorderCrossing,
) *SpatialborderCrossing {
	bbox, _ := rtreego.NewRectFromPoints(
		rtreego.Point{crossing.Location.X(), crossing.Location.Y()},
		rtreego.Point{crossing.Location.X(), crossing.Location.Y()},
	)
	return &SpatialborderCrossing{
		Crossing: crossing,
		bbox: &bbox,
	}
}

// Bounds returns the bounding box for R-tree queries.
// Implements the rtreego.Spatial interface.
func (sbc SpatialborderCrossing) Bounds() rtreego.Rect {
	return *sbc.bbox
}
