package index

import (
	"github.com/dhconnelly/rtreego"
)

// Region represents a geographic region with core and extended boundaries.
// The core shape is the official boundary; the extended shape includes a buffer
// zone for routing purposes (to handle routes that briefly exit and re-enter).
type Region struct {
	name          string    // Unique region identifier
	coreShape     *GeoShape // Official region boundary
	extendedShape *GeoShape // Extended boundary with buffer zone
}

// NewRegion creates a new region with the given name and shapes.
func NewRegion(
	name string,
	coreShape *GeoShape,
	extendedShape *GeoShape,
) *Region {
	return &Region{
		name: name,
		coreShape: coreShape,
		extendedShape: extendedShape,
	}
}

// Name returns the region's unique identifier.
func (r Region) Name() string {
	return r.name
}

// CoreShape returns the region's core (official) boundary.
func (r Region) CoreShape() *GeoShape {
	return r.coreShape
}

// ExtendedShape returns the region's extended boundary with buffer zone.
func (r Region) ExtendedShape() *GeoShape {
	return r.extendedShape
}

// AddToSpatialIndex adds this region's bounding boxes to the R-tree indices.
// Both core and extended shapes are indexed for efficient spatial queries.
func (c Region) AddToSpatialIndex(
	coreRtree *rtreego.Rtree,
	extendedRtree *rtreego.Rtree,
) {
	for i, rect := range c.coreShape.BBoxSet() {
		if rect == nil {
			continue
		}
		coreRtree.Insert(NewSpatialRegion(&c, BoxLocation(i), rect))
	}

	for i, rect := range c.extendedShape.BBoxSet() {
		if rect == nil {
			continue
		}
		extendedRtree.Insert(NewSpatialRegion(&c, BoxLocation(i), rect))
	}
}

// SpatialRegion wraps a Region for R-tree indexing.
// It stores the region reference along with its bounding box for a specific quadrant.
type SpatialRegion struct {
	Region      *Region       // Reference to the region
	boxLocation BoxLocation   // Which world quadrant this entry covers
	bbox        *rtreego.Rect // Bounding box for the R-tree
}

// NewSpatialRegion creates a new spatial index entry for a region.
func NewSpatialRegion(
	region *Region,
	boxLocation BoxLocation,
	bbox *rtreego.Rect,
) *SpatialRegion {
	return &SpatialRegion{
		Region:      region,
		boxLocation: boxLocation,
		bbox:        bbox,
	}
}

// Bounds returns the bounding box for R-tree queries.
// Implements the rtreego.Spatial interface.
func (s SpatialRegion) Bounds() rtreego.Rect {
	return *s.bbox
}

// BoxLocation returns which world quadrant this spatial entry covers.
func (s SpatialRegion) BoxLocation() BoxLocation {
	return s.boxLocation
}
