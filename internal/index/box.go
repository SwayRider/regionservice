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
// bounding boxes for each. This allows correct spatial queries for regions
// that span the date line.
package index

import "github.com/paulmach/orb"

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

// Size returns the area of the bounding box in square degrees.
// Returns 0 if the box is empty (inverted bounds).
func (b Box) Size() float64 {
	if b.max[0] < b.min[0] || b.max[1] < b.min[1] {
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
func (b *Bounds) Extend(other *Bounds) {
	if other.NW.Size() > 0 {
		b.NW.Add(other.NW.min)
		b.NW.Add(other.NW.max)
	}
	if other.NE.Size() > 0 {
		b.NE.Add(other.NE.min)
		b.NE.Add(other.NE.max)
	}
	if other.SW.Size() > 0 {
		b.SW.Add(other.SW.min)
		b.SW.Add(other.SW.max)
	}
	if other.SE.Size() > 0 {
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
