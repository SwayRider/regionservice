package index

import (
	"github.com/paulmach/orb"
	"github.com/dhconnelly/rtreego"
)

// GeoShape represents a geographic shape with its geometry and bounding boxes.
// The bounding boxes are split by world quadrant for antimeridian handling.
type GeoShape struct {
	geometry orb.MultiPolygon // The actual polygon geometry
	bboxSet  []*rtreego.Rect  // Bounding boxes for each quadrant (NW, NE, SW, SE)
}

// NewGeoShape creates a new GeoShape with the given geometry and bounding boxes.
func NewGeoShape(
	geometry orb.MultiPolygon,
	bboxSet []*rtreego.Rect,
) *GeoShape {
	return &GeoShape{
		geometry: geometry,
		bboxSet:  bboxSet,
	}
}

// Geometry returns the multi-polygon geometry of this shape.
func (gs GeoShape) Geometry() orb.MultiPolygon {
	return gs.geometry
}

// BBoxSet returns the bounding boxes for each world quadrant.
func (gs GeoShape) BBoxSet() []*rtreego.Rect {
	return gs.bboxSet
}
