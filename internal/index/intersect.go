// intersect.go implements exact polygon intersection tests used by the
// region index: polygon edge vs. axis-aligned box, and polygon edge vs.
// circle.
//
// The previous corner/vertex-only checks missed the classic edge-edge
// crossing case — a polygon edge crossing a box edge, or passing within a
// circle's radius, with no vertex inside — producing silent false negatives.
// All tests operate in an unwrapped longitude space (see antimeridian.go) and
// treat touching the boundary as intersecting.

package index

import (
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/planar"
)

// metersPerDegree converts degrees to meters at the equator.
const metersPerDegree = orb.EarthRadius * math.Pi / 180

// segmentIntersectsBox reports whether the segment from a to b intersects the
// axis-aligned box defined by bottomLeft and topRight, using the
// Liang-Barsky clipping algorithm. Segments lying on the box boundary count
// as intersecting.
func segmentIntersectsBox(a, b, bottomLeft, topRight orb.Point) bool {
	dx := b[0] - a[0]
	dy := b[1] - a[1]

	t0, t1 := 0.0, 1.0
	for i := 0; i < 4; i++ {
		var p, q float64
		switch i {
		case 0: // left edge
			p, q = -dx, a[0]-bottomLeft[0]
		case 1: // right edge
			p, q = dx, topRight[0]-a[0]
		case 2: // bottom edge
			p, q = -dy, a[1]-bottomLeft[1]
		default: // top edge
			p, q = dy, topRight[1]-a[1]
		}

		if p == 0 {
			// Segment is parallel to this boundary; it can only be
			// outside when it lies entirely beyond it.
			if q < 0 {
				return false
			}
			continue
		}

		r := q / p
		if p < 0 {
			// Segment enters the box through this boundary.
			if r > t1 {
				return false
			}
			if r > t0 {
				t0 = r
			}
		} else {
			// Segment leaves the box through this boundary.
			if r < t0 {
				return false
			}
			if r < t1 {
				t1 = r
			}
		}
	}
	return true
}

// ringIntersectsBox reports whether any edge of ring (including the closing
// edge) intersects the box.
func ringIntersectsBox(ring orb.Ring, bottomLeft, topRight orb.Point) bool {
	if len(ring) < 2 {
		return false
	}
	for i := 1; i < len(ring); i++ {
		if segmentIntersectsBox(ring[i-1], ring[i], bottomLeft, topRight) {
			return true
		}
	}
	return segmentIntersectsBox(ring[len(ring)-1], ring[0], bottomLeft, topRight)
}

// polygonIntersectsBox reports whether any edge of the multi-polygon
// intersects the axis-aligned box.
func polygonIntersectsBox(mp orb.MultiPolygon, bottomLeft, topRight orb.Point) bool {
	for _, polygon := range mp {
		for _, ring := range polygon {
			if ringIntersectsBox(ring, bottomLeft, topRight) {
				return true
			}
		}
	}
	return false
}

// polygonIntersectsCircle reports whether any edge of the multi-polygon comes
// within radiusMeters of the center. Vertices are projected into an
// equirectangular plane centered on the circle center — longitude deltas are
// wrapped to [-180, 180], keeping the projection consistent across the
// antimeridian — and the planar distance from the origin to each edge
// approximates the geodesic distance. The projection error is below ~2% for
// any practical radius, which only matters at the decision boundary.
func polygonIntersectsCircle(mp orb.MultiPolygon, center orb.Point, radiusMeters float64) bool {
	cosLat := math.Cos(center[1] * math.Pi / 180)
	project := func(p orb.Point) orb.Point {
		return orb.Point{
			wrapLon(p[0]-center[0]) * metersPerDegree * cosLat,
			(p[1] - center[1]) * metersPerDegree,
		}
	}

	origin := orb.Point{0, 0}
	for _, polygon := range mp {
		for _, ring := range polygon {
			if ringIntersectsCircle(ring, project, origin, radiusMeters) {
				return true
			}
		}
	}
	return false
}

// ringIntersectsCircle reports whether any edge of ring comes within
// radiusMeters of the origin in the projected plane.
func ringIntersectsCircle(
	ring orb.Ring,
	project func(orb.Point) orb.Point,
	origin orb.Point,
	radiusMeters float64,
) bool {
	if len(ring) < 2 {
		return false
	}
	prev := project(ring[0])
	for i := 1; i < len(ring); i++ {
		cur := project(ring[i])
		if planar.DistanceFromSegment(prev, cur, origin) <= radiusMeters {
			return true
		}
		prev = cur
	}
	// Closing edge; a degenerate (already closed) ring makes this a
	// distance-to-vertex check, which the vertex test already covers.
	return planar.DistanceFromSegment(prev, project(ring[0]), origin) <= radiusMeters
}
