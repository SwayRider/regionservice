// Package index provides spatial indexing for region geometries and border crossings.
//
// The package uses R-tree spatial indices for efficient geospatial queries.
// It supports searching by point, bounding box, and radius, as well as
// finding border crossing locations between regions.
//
// # Quadrant Handling
//
// To handle geometries that cross the antimeridian (180° longitude), the package
// divides the world into four quadrants (NW, NE, SW, SE) and maintains separate
// bounding boxes for each. This lets the R-tree find candidates for regions
// that span the date line. The quadrant boxes are the candidate half of the
// story; the exact containment/intersection checks additionally unwrap
// longitudes around the query (see antimeridian.go) so planar tests are
// consistent across the date line.
//
// Degenerate (zero-area) quadrant boxes from axis-aligned regions are widened
// to the shape's extent before insertion (see bboxSetFromBounds) so such
// regions stay queryable.
package index

import (
	"github.com/dhconnelly/rtreego"
	"github.com/paulmach/orb"
)

// bboxPad is the minimum width and height of an R-tree bounding box, in
// degrees. Zero-area quadrant boxes (regions with axis-aligned vertices on
// the 0/±180 meridians or the 0 parallel) are padded to this size so the
// R-tree can still return them as candidates; the exact containment checks
// filter them later, so the padding can never produce false matches.
const bboxPad = 1e-9

// BoxLocation represents one of the four world quadrants.
// Used to handle geometries that cross the antimeridian.
type BoxLocation int

// World quadrant constants.
const (
	NW BoxLocation = 0 // Northwest quadrant (lon <= 0, lat >= 0)
	NE BoxLocation = 1 // Northeast quadrant (lon >= 0, lat >= 0)
	SW BoxLocation = 2 // Southwest quadrant (lon <= 0, lat <= 0)
	SE BoxLocation = 3 // Southeast quadrant (lon >= 0, lat <= 0)
)

// HasPoint returns true if the point falls within this quadrant.
func (bl BoxLocation) HasPoint(pt orb.Point) bool {
	switch bl {
	case NW:
		return pt.X() <= 0 && pt.Y() >= 0
	case NE:
		return pt.X() >= 0 && pt.Y() >= 0
	case SW:
		return pt.X() <= 0 && pt.Y() <= 0
	case SE:
		return pt.X() >= 0 && pt.Y() <= 0
	default:
		return false
	}
}

// BottomLeft returns the bottom-left corner of this quadrant.
func (bl BoxLocation) BottomLeft() orb.Point {
	switch bl {
	case NW:
		return orb.Point{-180, 0}
	case NE:
		return orb.Point{0, 0}
	case SW:
		return orb.Point{-180, -90}
	case SE:
		return orb.Point{0, -90}
	default:
		return orb.Point{0, 0}
	}
}

// TopRight returns the top-right corner of this quadrant.
func (bl BoxLocation) TopRight() orb.Point {
	switch bl {
	case NW:
		return orb.Point{0, 90}
	case NE:
		return orb.Point{180, 90}
	case SW:
		return orb.Point{0, 0}
	case SE:
		return orb.Point{180, 0}
	default:
		return orb.Point{0, 0}
	}
}

// TransformPoint adjusts a point's longitude to fit within this quadrant.
// Used for handling antimeridian crossings by shifting longitude by 360°.
func (bl BoxLocation) TransformPoint(pt orb.Point) orb.Point {
	if bl.HasPoint(pt) {
		return pt
	}

	npt := orb.Point{pt.X(), pt.Y()}
	switch bl {
	case NW, SW:
		if npt[0] > 0 {
			npt[0] -= 360
		}
	case NE, SE:
		if npt[0] < 0 {
			npt[0] += 360
		}
	}
	return npt
}

// Box represents an axis-aligned bounding box within a specific quadrant.
// Coordinates are in longitude/latitude format.
type Box struct {
	location BoxLocation // Which world quadrant this box belongs to
	min      orb.Point   // Minimum corner (bottom-left)
	max      orb.Point   // Maximum corner (top-right)
}

// NewBox creates a new empty bounding box for the given quadrant.
// The box is initialized with inverted bounds so the first Add() sets the actual bounds.
func NewBox(location BoxLocation) *Box {
	return &Box{
		location: location,
		min:      location.TopRight(),
		max:      location.BottomLeft(),
	}
}

// Add expands the bounding box to include the given point.
// Points outside this box's quadrant are ignored.
func (b *Box) Add(pt orb.Point) {
	if !b.location.HasPoint(pt) {
		return
	}
	// longitude
	if pt[0] < b.min[0] {
		b.min[0] = pt[0]
	}
	if pt[1] < b.min[1] {
		b.min[1] = pt[1]
	}
	if pt[0] > b.max[0] {
		b.max[0] = pt[0]
	}
	if pt[1] > b.max[1] {
		b.max[1] = pt[1]
	}
}

// IsEmpty returns true if no point has been added to this box, i.e. the
// bounds are still in their inverted initial state.
func (b Box) IsEmpty() bool {
	return b.max[0] < b.min[0] || b.max[1] < b.min[1]
}

// Size returns the area of the bounding box in square degrees.
// Returns 0 if the box is empty (inverted bounds) or degenerate (zero
// width or height); use IsEmpty to distinguish the two.
func (b Box) Size() float64 {
	if b.IsEmpty() {
		return 0
	}
	return (b.max[0] - b.min[0]) * (b.max[1] - b.min[1])
}

// Bounds returns the bounding box as an orb.Bound.
func (b Box) Bounds() orb.Bound {
	return orb.Bound{
		Min: b.min,
		Max: b.max,
	}
}

// Bounds holds bounding boxes for all four world quadrants.
// This structure enables handling of geometries that cross the antimeridian.
type Bounds struct {
	NW *Box // Northwest quadrant bounding box
	NE *Box // Northeast quadrant bounding box
	SW *Box // Southwest quadrant bounding box
	SE *Box // Southeast quadrant bounding box
}

// NewBounds creates a new Bounds with empty boxes for all quadrants.
func NewBounds() *Bounds {
	return &Bounds{
		NW: NewBox(NW),
		NE: NewBox(NE),
		SW: NewBox(SW),
		SE: NewBox(SE),
	}
}

// Add expands the bounds to include the given point.
// The point is added to the appropriate quadrant box.
func (b *Bounds) Add(pt orb.Point) {
	b.NW.Add(pt)
	b.NE.Add(pt)
	b.SW.Add(pt)
	b.SE.Add(pt)
}

// Extend expands these bounds to include all points from another Bounds.
// Degenerate boxes (zero area but with points) are propagated too, so
// axis-aligned geometries stay queryable.
func (b *Bounds) Extend(other *Bounds) {
	if !other.NW.IsEmpty() {
		b.NW.Add(other.NW.min)
		b.NW.Add(other.NW.max)
	}
	if !other.NE.IsEmpty() {
		b.NE.Add(other.NE.min)
		b.NE.Add(other.NE.max)
	}
	if !other.SW.IsEmpty() {
		b.SW.Add(other.SW.min)
		b.SW.Add(other.SW.max)
	}
	if !other.SE.IsEmpty() {
		b.SE.Add(other.SE.min)
		b.SE.Add(other.SE.max)
	}
}

// Boxes returns all four quadrant boxes as a slice.
func (b Bounds) Boxes() []*Box {
	return []*Box{
		b.NW,
		b.NE,
		b.SW,
		b.SE,
	}
}

// bboxSetFromBounds converts the quadrant boxes into the R-tree bounding
// boxes used for candidate filtering, one per quadrant (nil for empty
// quadrants, matching BoxLocation indexing).
//
// Boxes with no points are dropped. Degenerate (zero-area) boxes — regions
// with axis-aligned vertices on the 0/±180 meridians or the 0 parallel —
// are widened to the shape's overall extent in the degenerate dimension(s):
// the quadrant split alone leaves such boxes as bare lines or points that
// never cover the region's interior, and rtreego's exclusive intersection
// test never matches an exact-zero-width rect. A small pad is added so
// queries touching the extent boundary still match. Shapes whose longitude
// extent exceeds 180° are treated as date-line straddlers and left
// unexpanded: their degenerate side is a bare line or point on the ±180
// meridian with no queryable area (the area-bearing side always has a
// non-degenerate box), and widening it would admit every query as a
// candidate against the planar-inconsistent raw ring, causing false
// positives at the opposite meridian.
func bboxSetFromBounds(bounds *Bounds) ([]*rtreego.Rect, error) {
	boxes := bounds.Boxes()

	// Overall extent across all quadrants, used to widen degenerate boxes.
	var gMinLon, gMaxLon, gMinLat, gMaxLat float64
	have := false
	for _, box := range boxes {
		if box.IsEmpty() {
			continue
		}
		b := box.Bounds()
		if !have {
			gMinLon, gMaxLon, gMinLat, gMaxLat = b.Min[0], b.Max[0], b.Min[1], b.Max[1]
			have = true
		} else {
			gMinLon = min(gMinLon, b.Min[0])
			gMaxLon = max(gMaxLon, b.Max[0])
			gMinLat = min(gMinLat, b.Min[1])
			gMaxLat = max(gMaxLat, b.Max[1])
		}
	}

	expandLon := have && gMaxLon-gMinLon <= 180

	bboxSet := make([]*rtreego.Rect, len(boxes))
	for i, box := range boxes {
		if box.IsEmpty() {
			continue
		}
		b := box.Bounds()
		minP, maxP := b.Min, b.Max
		if expandLon && maxP[0]-minP[0] < bboxPad {
			minP[0] = gMinLon - bboxPad/2
			maxP[0] = gMaxLon + bboxPad/2
		}
		if maxP[1]-minP[1] < bboxPad {
			minP[1] = gMinLat - bboxPad/2
			maxP[1] = gMaxLat + bboxPad/2
		}
		bbox, err := rtreego.NewRectFromPoints(
			rtreego.Point{minP[0], minP[1]},
			rtreego.Point{maxP[0], maxP[1]},
		)
		if err != nil {
			return nil, err
		}
		bboxSet[i] = &bbox
	}
	return bboxSet, nil
}

// LineSegment represents a line between two points.
type LineSegment struct {
	p1 orb.Point // Start point
	p2 orb.Point // End point
}

// NewLineSegment creates a new line segment between two points.
func NewLineSegment(p1, p2 orb.Point) *LineSegment {
	return &LineSegment{
		p1: p1,
		p2: p2,
	}
}

// Rect represents an axis-aligned rectangle for intersection tests.
type Rect struct {
	min orb.Point // Minimum corner (bottom-left)
	max orb.Point // Maximum corner (top-right)
}

// NewRect creates a new rectangle from corner points.
func NewRect(min, max orb.Point) *Rect {
	return &Rect{
		min: min,
		max: max,
	}
}

// Contains returns true if this rectangle fully contains another rectangle.
func (r Rect) Contains(r2 *Rect) bool {
	return r.min[0] <= r2.min[0] && r.max[0] >= r2.max[0] &&
		r.min[1] <= r2.min[1] && r.max[1] >= r2.max[1]
}

// Within returns true if this rectangle is fully contained by another rectangle.
func (r Rect) Within(r2 *Rect) bool {
	return r2.Contains(&r)
}

// Intersects returns true if this rectangle overlaps with another rectangle.
func (r Rect) Intersects(r2 *Rect) bool {
	return r.min[0] < r2.max[0] && r.max[0] > r2.min[0] &&
		r.min[1] < r2.max[1] && r.max[1] > r2.min[1]
}

// ContainsLineSegment returns true if this rectangle fully contains a line segment.
func (r Rect) ContainsLineSegment(l *LineSegment) bool {
	return r.min[0] <= l.p1[0] && r.max[0] >= l.p2[0] &&
		r.min[1] <= l.p1[1] && r.max[1] >= l.p2[1]
}

// IntersectsLineSegment returns true if this rectangle intersects a line segment.
func (r Rect) IntersectsLineSegment(l *LineSegment) bool {
	return r.min[0] < l.p2[0] && r.max[0] > l.p1[0] &&
		r.min[1] < l.p2[1] && r.max[1] > l.p1[1]
}
