package index

import (
	"errors"

	"github.com/dhconnelly/rtreego"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/geojson"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/planar"
)

// pointSize defines the minimum bounding box size for point queries.
const pointSize = 0.0001

// RegionResult represents a region found by a spatial query.
type RegionResult struct {
	Region     *Region // The matched region
	IsExtended bool    // True if matched via extended boundary, false for core
}

// RegionIndex provides spatial indexing for region lookups.
// It maintains separate R-trees for core and extended region boundaries.
type RegionIndex struct {
	coreRtree     *rtreego.Rtree // R-tree for core region boundaries
	extendedRtree *rtreego.Rtree // R-tree for extended region boundaries
}

// NewRegionIndex creates a new empty region index with R-trees initialized.
func NewRegionIndex() *RegionIndex {
	return &RegionIndex{
		coreRtree: rtreego.NewTree(2, 10, 25),
		extendedRtree: rtreego.NewTree(2, 10, 25),
	}
}

// Add parses GeoJSON feature collections and adds a region to the index.
// Both core and extended contours are parsed and indexed.
func (i *RegionIndex) Add(
	regionName string,
	coreFC *geojson.FeatureCollection,
	extendedFC *geojson.FeatureCollection,
) error {
	coreShape, err := parseFeature(coreFC)
	if err != nil {
		return err
	}

	extShape, err := parseFeature(extendedFC)
	if err != nil {
		return err
	}

	region := NewRegion(
		regionName,
		coreShape,
		extShape,
	)
	region.AddToSpatialIndex(i.coreRtree, i.extendedRtree)
	return nil
}

// SearchByPoint finds all regions containing the given point.
// If extended is true, also searches extended boundaries.
// Core matches are returned first, followed by extended-only matches.
func (i RegionIndex) SearchByPoint(
	p orb.Point,
	extended bool,
) (res []*RegionResult) {
	box, _ := rtreego.NewRect(
		rtreego.Point{p[0], p[1]},
		[]float64{pointSize, pointSize})

	var coreCands, extCands []rtreego.Spatial
	coreCands = i.coreRtree.SearchIntersect(box)
	if extended {
		extCands = i.extendedRtree.SearchIntersect(box)
	}

	seen := make(map[string]struct{})
	for _, r := range coreCands {
		sr := r.(*SpatialRegion)
		if _, found := seen[sr.Region.Name()]; found {
			continue
		}

		if planar.MultiPolygonContains(
			sr.Region.CoreShape().Geometry(), p,
		) {
			seen[sr.Region.Name()] = struct{}{}
			res = append(res, &RegionResult{
				Region: sr.Region,
				IsExtended: false,
			})
			
		}
	}
	if extended {
		for _, r := range extCands {
			sr := r.(*SpatialRegion)
			if _, found := seen[sr.Region.Name()]; found {
				continue
			}

			if planar.MultiPolygonContains(
				sr.Region.ExtendedShape().Geometry(), p,
			) {
				seen[sr.Region.Name()] = struct{}{}
				res = append(res, &RegionResult{
					Region: sr.Region,
					IsExtended: true,
				})
			}
		}
	}

	return
}

// SearchByBox finds all regions that intersect the given bounding box.
// Handles boxes that cross the antimeridian by splitting into two queries.
// If extended is true, also searches extended boundaries.
func (i RegionIndex) SearchByBox(
	bottomLeft, topRight orb.Point,
	extended bool,
) (res []*RegionResult) {
	w, h := topRight[0]-bottomLeft[0], topRight[1]-bottomLeft[1]

	// Negative width --> crossing on 180th meridian
	// Split in 2 regions and search for both
	if w < 0 {
		topRight1 := orb.Point{180, topRight[1]}
		bottomLeft1 := orb.Point{-180, bottomLeft[1]}
		res1 := i.SearchByBox(bottomLeft, topRight1, extended)
		res2 := i.SearchByBox(bottomLeft1, topRight, extended)

		seen := make(map[string]struct{})

		for _, r := range res1 {
			if r.IsExtended {
				continue
			}
			if _, found := seen[r.Region.Name()]; found {
				continue
			}
			seen[r.Region.Name()] = struct{}{}
			res = append(res, r)
		}
		for _, r := range res2 {
			if r.IsExtended {
				continue
			}
			if _, found := seen[r.Region.Name()]; found {
				continue
			}
			seen[r.Region.Name()] = struct{}{}
			res = append(res, r)
		}

		if extended {
			for _, r := range res1 {
				if !r.IsExtended {
					continue
				}
				if _, found := seen[r.Region.Name()]; found {
					continue
				}
				seen[r.Region.Name()] = struct{}{}
				res = append(res, r)
			}
			for _, r := range res2 {
				if !r.IsExtended {
					continue
				}
				if _, found := seen[r.Region.Name()]; found {
					continue
				}
				seen[r.Region.Name()] = struct{}{}
				res = append(res, r)
			}
		}
		return res
	}

	box, _ := rtreego.NewRect(
		rtreego.Point{bottomLeft[0], bottomLeft[1]},
		[]float64{w, h},
	)

	var coreCands, extCands []rtreego.Spatial
	coreCands = i.extendedRtree.SearchIntersect(box)
	if extended {
		extCands = i.extendedRtree.SearchIntersect(box)
	}

	seen := make(map[string]struct{})
	for _, r := range coreCands {
		sr := r.(*SpatialRegion)
		if _, found := seen[sr.Region.Name()]; found {
			continue
		}
		
		if containsOrIntersectsBox(
			bottomLeft, topRight, sr.Region, false,
		) {
			seen[sr.Region.Name()] = struct{}{}
			res = append(res, &RegionResult{
				Region: sr.Region,
				IsExtended: false,
			})
		}
	}
	if extended {
		for _, r := range extCands {
			sr := r.(*SpatialRegion)
			if _, found := seen[sr.Region.Name()]; found {
				continue
			}
			
			if containsOrIntersectsBox(
				bottomLeft, topRight, sr.Region, true,
			) {
				seen[sr.Region.Name()] = struct{}{}
				res = append(res, &RegionResult{
					Region: sr.Region,
					IsExtended: true,
				})
			}
		}
	}
	return res
}

// SearchByRadius finds all regions within the given radius of a center point.
// The radius is specified in kilometers. Uses SearchByBox internally with
// a bounding box approximation, then filters by actual distance.
func (i RegionIndex) SearchByRadius(
	center orb.Point,
	radiusKm float64,
	extended bool,
) (res []*RegionResult) {
	radiusMeter := radiusKm * 1000

	bl := orb.Point{
		geo.PointAtBearingAndDistance(center, 270, radiusMeter)[0],
		geo.PointAtBearingAndDistance(center, 180, radiusMeter)[1]}
	tr := orb.Point{
		geo.PointAtBearingAndDistance(center, 90, radiusMeter)[0],
		geo.PointAtBearingAndDistance(center, 0, radiusMeter)[1]}

	
	cands := i.SearchByBox(bl, tr, extended)
	for _, rr := range cands {
		if containsOrIntersectsCircle(
			center, radiusMeter, rr.Region, rr.IsExtended,
		) {
			res = append(res, rr)
		}
	}
	return
}

// containsOrIntersectsCircle checks if a region intersects with a circle.
// Returns true if the center is inside the region or any vertex is within radius.
func containsOrIntersectsCircle(
	center orb.Point,
	radiusMeters float64,
	r *Region,
	extended bool,
) bool {
	var geom orb.MultiPolygon
	if extended {
		geom = r.ExtendedShape().Geometry()
	} else {
		geom = r.CoreShape().Geometry()
	}

	if planar.MultiPolygonContains(geom, center) {
		return true
	}

	for _, polygon := range geom {
		for _, lineString := range polygon {
			for _, point := range lineString {
				if geo.Distance(center, point) <= radiusMeters {
					return true
				}
			}
		}
	}
	return false
}

// containsOrIntersectsBox checks if a region intersects with a bounding box.
// Returns true if any box corner is inside the region or any region vertex is in the box.
func containsOrIntersectsBox(
	bottomLeft, topRight orb.Point,
	r *Region,
	extended bool,
) bool {
	var geom orb.MultiPolygon
	if extended {
		geom = r.ExtendedShape().Geometry()
	} else {
		geom = r.CoreShape().Geometry()
	}

	p0 := bottomLeft;
	p1 := orb.Point{bottomLeft.X(), topRight.Y()};
	p2 := topRight;
	p3 := orb.Point{topRight.X(), bottomLeft.Y()};
	if (
			planar.MultiPolygonContains(geom, p0) ||
			planar.MultiPolygonContains(geom, p1) ||
			planar.MultiPolygonContains(geom, p2) ||
			planar.MultiPolygonContains(geom, p3)) {
		return true
	}

	for _, polygon := range geom {
		for _, lineString := range polygon {
			for _, point := range lineString {
				if (
						point.X() >= bottomLeft.X() &&
						point.X() <= topRight.X() &&
						point.Y() >= bottomLeft.Y() &&
						point.Y() <= topRight.Y()) {
					return true
				}
			}
		}
	}
	return false
}

// parseFeature converts a GeoJSON feature collection to a GeoShape.
// Extracts polygon geometries and computes bounding boxes per quadrant.
func parseFeature(gj *geojson.FeatureCollection) (*GeoShape, error) {
	var multiPoly orb.MultiPolygon
	bounds := NewBounds()

	for _, feature := range gj.Features {
		geometry := feature.Geometry
		switch g := geometry.Geometry.(type) {
		case geom.Polygon:
			poly, polyBounds := geomPolygonToOrbPolygon(g)
			multiPoly = append(multiPoly, poly)
			bounds.Extend(polyBounds)
		case geom.MultiPolygon:
			for _, p := range g {
				poly, polyBounds := geomPolygonToOrbPolygon(p)
				multiPoly = append(multiPoly, poly)
				bounds.Extend(polyBounds)
			}
		}
	}
	if len(multiPoly) == 0 {
		return nil, errors.New("no polygons found")
	}

	bboxSet := make([]*rtreego.Rect, len(bounds.Boxes()))
	for i, box := range bounds.Boxes() {
		if box.Size() == 0 {
			continue
		}
		bounds := box.Bounds()
		bbox, _  := rtreego.NewRectFromPoints(
			rtreego.Point{bounds.Min.X(), bounds.Min.Y()},
			rtreego.Point{bounds.Max.X(), bounds.Max.Y()},
		)
		bboxSet[i] = &bbox
	}

	return NewGeoShape(multiPoly, bboxSet), nil
}

// geomPolygonToOrbPolygon converts a geom.Polygon to an orb.Polygon.
// Also computes the bounding boxes for each world quadrant.
func geomPolygonToOrbPolygon(
	geomPoly geom.Polygon,
) (
	orb.Polygon,
	*Bounds,
) {
	numRings := len(geomPoly.LinearRings())
	orbPoly := make(orb.Polygon, numRings)
	bounds := NewBounds()

	for i, ring := range geomPoly.LinearRings() {
		numCoords := len(ring)
		orbRing := make(orb.Ring, numCoords)

		for j, coord := range ring {
			p := orb.Point{coord[0], coord[1]}
			orbRing[j] = p
			bounds.Add(p)
		}
		orbPoly[i] = orbRing
	}
	return orbPoly, bounds
}
