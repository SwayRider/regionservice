// antimeridian.go implements longitude unwrapping for geometries that cross
// the 180th meridian (the antimeridian / date line).
//
// Polygon geometry is stored in raw GeoJSON coordinates (lon in [-180, 180]),
// so a ring whose vertices span the date line (e.g. 179.5 and -179.5) is
// planar-inconsistent: ray casting treats it as spanning ~358° instead of the
// ~2° the ring actually covers. Before any planar containment or intersection
// test, vertices are unwrapped — shifted by the multiple of 360° that brings
// them closest to the query's reference longitude — so the ring becomes
// contiguous around the query. Regions that do not approach the date line are
// returned unchanged without copying.

package index

import (
	"math"

	"github.com/paulmach/orb"
)

// wrapLon normalizes a longitude into the range [-180, 180].
func wrapLon(lon float64) float64 {
	lon = math.Mod(lon+180, 360)
	if lon < 0 {
		lon += 360
	}
	return lon - 180
}

// unwrapPoint returns p with its longitude shifted by the multiple of 360°
// that brings it closest to refLon. Points already within 180° of refLon are
// returned unchanged. A vertex exactly 180° away may be shifted either way;
// both resulting longitudes are the same place on the globe.
func unwrapPoint(p orb.Point, refLon float64) orb.Point {
	p[0] -= 360 * math.Round((p[0]-refLon)/360)
	return p
}

// unwrapRing returns a copy of r with every vertex unwrapped around refLon.
func unwrapRing(r orb.Ring, refLon float64) orb.Ring {
	out := make(orb.Ring, len(r))
	for i, pt := range r {
		out[i] = unwrapPoint(pt, refLon)
	}
	return out
}

// unwrapPolygon returns a copy of p with every ring unwrapped around refLon.
func unwrapPolygon(p orb.Polygon, refLon float64) orb.Polygon {
	out := make(orb.Polygon, len(p))
	for i, ring := range p {
		out[i] = unwrapRing(ring, refLon)
	}
	return out
}

// unwrapMultiPolygon returns a copy of mp with every polygon unwrapped
// around refLon.
func unwrapMultiPolygon(mp orb.MultiPolygon, refLon float64) orb.MultiPolygon {
	out := make(orb.MultiPolygon, len(mp))
	for i, polygon := range mp {
		out[i] = unwrapPolygon(polygon, refLon)
	}
	return out
}

// unwrapIfNeeded returns the shape's geometry unwrapped around refLon so it
// is longitude-contiguous in planar tests. When every quadrant bounding box
// already lies within 180° of refLon — the case for regions far from the
// antimeridian — the shared geometry is returned unchanged, avoiding a copy.
func (gs *GeoShape) unwrapIfNeeded(refLon float64) orb.MultiPolygon {
	for _, box := range gs.BBoxSet() {
		if box == nil {
			continue
		}
		if box.PointCoord(0) < refLon-180 ||
			box.PointCoord(0)+box.LengthsCoord(0) > refLon+180 {
			return unwrapMultiPolygon(gs.Geometry(), refLon)
		}
	}
	return gs.Geometry()
}
