// border_crossing.go defines types for indexing border crossing points.

package index

import (
	"github.com/paulmach/orb"
	"github.com/swayrider/regionservice/internal/types"
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
