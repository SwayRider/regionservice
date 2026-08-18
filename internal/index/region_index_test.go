package index

import (
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/geojson"
	"github.com/paulmach/orb"
)

// =============================================================================
// Test helpers
// =============================================================================

// testGeoShape builds a GeoShape from raw orb geometry, computing the
// quadrant bounding boxes via the same production code path as parseFeature.
func testGeoShape(mp orb.MultiPolygon) *GeoShape {
	bounds := NewBounds()
	for _, polygon := range mp {
		for _, ring := range polygon {
			for _, p := range ring {
				bounds.Add(p)
			}
		}
	}

	bboxSet, err := bboxSetFromBounds(bounds)
	if err != nil {
		panic(err)
	}
	return NewGeoShape(mp, bboxSet)
}

// newTestRegion builds a Region with the given core and extended shapes.
func newTestRegion(name string, core, extended orb.MultiPolygon) *Region {
	return NewRegion(name, testGeoShape(core), testGeoShape(extended))
}

// addTestRegion inserts a region into the index's R-trees.
func addTestRegion(idx *RegionIndex, name string, core, extended orb.MultiPolygon) {
	region := newTestRegion(name, core, extended)
	region.AddToSpatialIndex(idx.coreRtree, idx.extendedRtree)
}

// northeastRect is a simple 6°×6° rectangle in the northeast quadrant at
// lon/lat [2, 8] (kept off the 0/±180 meridians so its quadrant bounding
// box is non-degenerate).
func northeastRect() orb.MultiPolygon {
	return orb.MultiPolygon{{{
		{2, 2}, {8, 2}, {8, 8}, {2, 8},
	}}}
}

// datelineRing is a rectangle crossing the 180th meridian:
// lon [179.5, 180] ∪ [-180, -179.5], lat [10, 12]. Vertices follow the
// GeoJSON convention of jumping through ±180° at the dateline.
func datelineRing() orb.Ring {
	return orb.Ring{
		{179.5, 10}, {179.5, 12}, {180, 12},
		{-180, 12}, {-179.5, 12}, {-179.5, 10},
	}
}

// datelineRegion returns a region straddling the antimeridian.
func datelineRegion() orb.MultiPolygon {
	return orb.MultiPolygon{{datelineRing()}}
}

func newTestIndex() *RegionIndex {
	return NewRegionIndex()
}

// hasResult reports whether res contains a result for the given region name.
func hasResult(res []*RegionResult, name string) bool {
	for _, r := range res {
		if r.Region.Name() == name {
			return true
		}
	}
	return false
}

// =============================================================================
// Axis-aligned region tests (degenerate quadrant boxes)
// =============================================================================

// axisAlignedSquare is a rectangle with edges exactly on the 0/10 meridians
// and parallels — every quadrant bounding box is zero-area.
func axisAlignedSquare() orb.MultiPolygon {
	return orb.MultiPolygon{{{
		{0, 0}, {10, 0}, {10, 10}, {0, 10},
	}}}
}

func TestSearchByPoint_AxisAlignedRegion(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "axis", axisAlignedSquare(), axisAlignedSquare())

	for _, p := range []orb.Point{{5, 5}, {1, 1}, {9, 9}} {
		res := idx.SearchByPoint(p, false)
		if len(res) != 1 || !hasResult(res, "axis") {
			t.Errorf("point %v inside axis-aligned region: got %v, want [axis]", p, res)
		}
	}
	// Point on the boundary corner.
	res := idx.SearchByPoint(orb.Point{0, 0}, false)
	if len(res) != 1 || !hasResult(res, "axis") {
		t.Errorf("corner point: got %v, want [axis]", res)
	}
	// Point outside.
	res = idx.SearchByPoint(orb.Point{15, 15}, false)
	if len(res) != 0 {
		t.Errorf("point outside axis-aligned region: got %v, want none", res)
	}
}

func TestSearchByBox_AxisAlignedRegion(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "axis", axisAlignedSquare(), axisAlignedSquare())

	res := idx.SearchByBox(orb.Point{2, 2}, orb.Point{8, 8}, false)
	if len(res) != 1 || !hasResult(res, "axis") {
		t.Errorf("box over axis-aligned region: got %v, want [axis]", res)
	}

	res = idx.SearchByBox(orb.Point{-15, -15}, orb.Point{-5, -5}, false)
	if len(res) != 0 {
		t.Errorf("disjoint box: got %v, want none", res)
	}
}

func TestSearchByRadius_AxisAlignedRegion(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "axis", axisAlignedSquare(), axisAlignedSquare())

	res := idx.SearchByRadius(orb.Point{5, 5}, 100, false)
	if len(res) != 1 || !hasResult(res, "axis") {
		t.Errorf("radius over axis-aligned region: got %v, want [axis]", res)
	}
}

func TestSearchByPoint_StraddlingZeroMeridian(t *testing.T) {
	// Rectangle with vertices on lon 0 — its NE and NW quadrant boxes are
	// zero-width, so interior queries must rely on the widened boxes.
	straddle := orb.MultiPolygon{{{
		{-5, 2}, {5, 2}, {5, 8}, {-5, 8},
	}}}
	idx := newTestIndex()
	addTestRegion(idx, "straddle", straddle, straddle)

	for _, p := range []orb.Point{{0, 5}, {3, 5}, {-3, 5}, {-4.9, 7.9}} {
		res := idx.SearchByPoint(p, false)
		if len(res) != 1 || !hasResult(res, "straddle") {
			t.Errorf("point %v: got %v, want [straddle]", p, res)
		}
	}
	res := idx.SearchByPoint(orb.Point{10, 5}, false)
	if len(res) != 0 {
		t.Errorf("point outside: got %v, want none", res)
	}
}

func TestSearchByPoint_SingleVertexQuadrant(t *testing.T) {
	// Triangle with a single vertex in the NW quadrant; the NW box is a
	// degenerate point and must be widened to cover the triangle.
	tri := orb.MultiPolygon{{{
		{0, 0}, {10, 0}, {5, 10},
	}}}
	idx := newTestIndex()
	addTestRegion(idx, "tri", tri, tri)

	for _, p := range []orb.Point{{0.1, 0.1}, {5, 9}, {5, 5}} {
		res := idx.SearchByPoint(p, false)
		if len(res) != 1 || !hasResult(res, "tri") {
			t.Errorf("point %v: got %v, want [tri]", p, res)
		}
	}
	res := idx.SearchByPoint(orb.Point{-1, 5}, false)
	if len(res) != 0 {
		t.Errorf("point outside: got %v, want none", res)
	}
}

// =============================================================================
// SearchByPoint Tests
// =============================================================================

func TestSearchByPoint_Basic(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	res := idx.SearchByPoint(orb.Point{5, 5}, false)
	if len(res) != 1 || !hasResult(res, "rect") {
		t.Errorf("point inside rect: got %v, want [rect]", res)
	}

	res = idx.SearchByPoint(orb.Point{15, 15}, false)
	if len(res) != 0 {
		t.Errorf("point outside rect: got %v, want none", res)
	}
}

func TestSearchByPoint_Extended(t *testing.T) {
	idx := newTestIndex()
	ext := orb.MultiPolygon{{{
		{0.5, 0.5}, {12, 0.5}, {12, 12}, {0.5, 12},
	}}}
	addTestRegion(idx, "rect", northeastRect(), ext)

	// Point in the extended buffer only.
	res := idx.SearchByPoint(orb.Point{10, 10}, false)
	if len(res) != 0 {
		t.Errorf("extended point without flag: got %v, want none", res)
	}
	res = idx.SearchByPoint(orb.Point{10, 10}, true)
	if len(res) != 1 || !hasResult(res, "rect") {
		t.Errorf("extended point with flag: got %v, want [rect]", res)
	}
}

func TestSearchByPoint_DatelineRegion(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "dateline", datelineRegion(), datelineRegion())

	// Query from the east side of the dateline.
	res := idx.SearchByPoint(orb.Point{179.8, 11}, false)
	if len(res) != 1 || !hasResult(res, "dateline") {
		t.Errorf("east-side point: got %v, want [dateline]", res)
	}

	// Query from the west side of the dateline.
	res = idx.SearchByPoint(orb.Point{-179.8, 11}, false)
	if len(res) != 1 || !hasResult(res, "dateline") {
		t.Errorf("west-side point: got %v, want [dateline]", res)
	}

	// Query far from the dateline.
	res = idx.SearchByPoint(orb.Point{0, 11}, false)
	if len(res) != 0 {
		t.Errorf("far point: got %v, want none", res)
	}
}

// =============================================================================
// SearchByBox Tests
// =============================================================================

func TestSearchByBox_Basic(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	res := idx.SearchByBox(orb.Point{2.5, 2.5}, orb.Point{7.5, 7.5}, false)
	if len(res) != 1 || !hasResult(res, "rect") {
		t.Errorf("overlapping box: got %v, want [rect]", res)
	}

	res = idx.SearchByBox(orb.Point{20, 20}, orb.Point{30, 30}, false)
	if len(res) != 0 {
		t.Errorf("disjoint box: got %v, want none", res)
	}
}

func TestSearchByBox_ContainsRegion(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	// Box fully containing the region.
	res := idx.SearchByBox(orb.Point{-5, -5}, orb.Point{15, 15}, false)
	if len(res) != 1 || !hasResult(res, "rect") {
		t.Errorf("containing box: got %v, want [rect]", res)
	}
}

func TestSearchByBox_DatelineCrossingBox(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "dateline", datelineRegion(), datelineRegion())
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	// Box crossing the 180th meridian; the dateline region is inside it.
	res := idx.SearchByBox(orb.Point{179.5, 10}, orb.Point{-179.5, 12}, false)
	if len(res) != 1 || !hasResult(res, "dateline") {
		t.Errorf("crossing box: got %v, want [dateline]", res)
	}
	if hasResult(res, "rect") {
		t.Errorf("crossing box must not match far-away rect, got %v", res)
	}
}

func TestSearchByBox_DatelineRegionFromEitherSide(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "dateline", datelineRegion(), datelineRegion())

	res := idx.SearchByBox(orb.Point{179.7, 10}, orb.Point{179.9, 12}, false)
	if len(res) != 1 || !hasResult(res, "dateline") {
		t.Errorf("east-side box: got %v, want [dateline]", res)
	}

	res = idx.SearchByBox(orb.Point{-179.9, 10}, orb.Point{-179.7, 12}, false)
	if len(res) != 1 || !hasResult(res, "dateline") {
		t.Errorf("west-side box: got %v, want [dateline]", res)
	}
}

// =============================================================================
// SearchByRadius Tests
// =============================================================================

func TestSearchByRadius_Basic(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	// Center inside the region → hit.
	res := idx.SearchByRadius(orb.Point{5, 5}, 100, false)
	if len(res) != 1 || !hasResult(res, "rect") {
		t.Errorf("radius over region: got %v, want [rect]", res)
	}

	// Center far away → miss.
	res = idx.SearchByRadius(orb.Point{50, 50}, 100, false)
	if len(res) != 0 {
		t.Errorf("far radius: got %v, want none", res)
	}
}

func TestSearchByRadius_Dateline(t *testing.T) {
	idx := newTestIndex()
	addTestRegion(idx, "dateline", datelineRegion(), datelineRegion())
	// West-only region across the dateline from the center.
	westOnly := orb.MultiPolygon{{{
		{-179.9, 10}, {-179.6, 10}, {-179.6, 12}, {-179.9, 12},
	}}}
	addTestRegion(idx, "west-only", westOnly, westOnly)
	// Far-away control regions.
	addTestRegion(idx, "far-west", orb.MultiPolygon{{{
		{-170, 10}, {-160, 10}, {-160, 12}, {-170, 12},
	}}}, nil)
	addTestRegion(idx, "rect", northeastRect(), northeastRect())

	// Center just east of the dateline; radius spans both sides.
	res := idx.SearchByRadius(orb.Point{179.9, 11}, 150, false)
	if len(res) != 2 {
		t.Fatalf("dateline radius: got %v, want 2 results", res)
	}
	if !hasResult(res, "dateline") {
		t.Errorf("dateline radius must match dateline region, got %v", res)
	}
	if !hasResult(res, "west-only") {
		t.Errorf("dateline radius must match west-only region, got %v", res)
	}
	if hasResult(res, "far-west") || hasResult(res, "rect") {
		t.Errorf("dateline radius matched far-away regions: %v", res)
	}
}

// =============================================================================
// containsOrIntersectsBox Tests (exact intersection checks)
// =============================================================================

func TestContainsOrIntersectsBox_EdgeCrossing(t *testing.T) {
	// Triangle with all vertices outside the box and no box corner inside
	// the triangle — the classic edge-edge crossing false negative.
	tri := orb.MultiPolygon{{{
		{5, -1}, {7, -1}, {6, 11},
	}}}
	r := newTestRegion("tri", tri, tri)

	if !containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("edge crossing the box must intersect")
	}
}

func TestContainsOrIntersectsBox_BoxInsidePolygon(t *testing.T) {
	big := orb.MultiPolygon{{{
		{-5, -5}, {15, -5}, {15, 15}, {-5, 15},
	}}}
	r := newTestRegion("big", big, big)

	if !containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("box fully inside polygon must intersect")
	}
}

func TestContainsOrIntersectsBox_PolygonInsideBox(t *testing.T) {
	small := orb.MultiPolygon{{{
		{1, 1}, {9, 1}, {5, 9},
	}}}
	r := newTestRegion("small", small, small)

	if !containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("polygon fully inside box must intersect")
	}
}

func TestContainsOrIntersectsBox_Disjoint(t *testing.T) {
	far := orb.MultiPolygon{{{
		{20, 20}, {21, 20}, {20.5, 21},
	}}}
	r := newTestRegion("far", far, far)

	if containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("disjoint polygon must not intersect")
	}
}

func TestContainsOrIntersectsBox_Grazing(t *testing.T) {
	// Edge lying exactly on the box boundary; all vertices outside the box.
	grazing := orb.MultiPolygon{{{
		{10, -1}, {10, 11}, {11, 5},
	}}}
	r := newTestRegion("grazing", grazing, grazing)

	if !containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("edge grazing the box boundary must intersect")
	}
}

func TestContainsOrIntersectsBox_Dateline(t *testing.T) {
	r := newTestRegion("dateline", datelineRegion(), datelineRegion())

	// Box on the east side of the dateline.
	if !containsOrIntersectsBox(orb.Point{179.7, 10}, orb.Point{179.9, 12}, r, false) {
		t.Error("east-side box must intersect dateline region")
	}
	// Box on the west side of the dateline.
	if !containsOrIntersectsBox(orb.Point{-179.9, 10}, orb.Point{-179.7, 12}, r, false) {
		t.Error("west-side box must intersect dateline region")
	}
	// Box far away.
	if containsOrIntersectsBox(orb.Point{0, 0}, orb.Point{10, 10}, r, false) {
		t.Error("far box must not intersect dateline region")
	}
}

// =============================================================================
// containsOrIntersectsCircle Tests (exact intersection checks)
// =============================================================================

func TestContainsOrIntersectsCircle_EdgeWithinRadius(t *testing.T) {
	// Vertical edges at lon 0.006° (~667 m) and 0.007° (~778 m); vertices at
	// ±0.012° lat (~1336 m). Radius 800 m reaches the edges but no vertex.
	ring := orb.MultiPolygon{{{
		{0.006, 0.012}, {0.007, 0.012}, {0.007, -0.012}, {0.006, -0.012},
	}}}
	r := newTestRegion("edge", ring, ring)

	if !containsOrIntersectsCircle(orb.Point{0, 0}, 800, r, false) {
		t.Error("edge within radius must intersect")
	}
	if containsOrIntersectsCircle(orb.Point{0, 0}, 600, r, false) {
		t.Error("edge beyond radius must not intersect")
	}
}

func TestContainsOrIntersectsCircle_CenterInside(t *testing.T) {
	ring := orb.MultiPolygon{{{
		{-0.01, -0.01}, {0.01, -0.01}, {0.01, 0.01}, {-0.01, 0.01},
	}}}
	r := newTestRegion("center", ring, ring)

	if !containsOrIntersectsCircle(orb.Point{0, 0}, 1, r, false) {
		t.Error("center inside polygon must intersect")
	}
}

func TestContainsOrIntersectsCircle_VertexWithinRadius(t *testing.T) {
	ring := orb.MultiPolygon{{{
		{0.005, 0.001}, {0.006, 0.001}, {0.006, 0.002}, {0.005, 0.002},
	}}}
	r := newTestRegion("vertex", ring, ring)

	if !containsOrIntersectsCircle(orb.Point{0, 0}, 800, r, false) {
		t.Error("vertex within radius must intersect")
	}
}

func TestContainsOrIntersectsCircle_FarAway(t *testing.T) {
	ring := orb.MultiPolygon{{{
		{1, 1}, {1.1, 1}, {1.1, 1.1}, {1, 1.1},
	}}}
	r := newTestRegion("far", ring, ring)

	if containsOrIntersectsCircle(orb.Point{0, 0}, 10000, r, false) {
		t.Error("far polygon must not intersect a 10 km radius")
	}
}

func TestContainsOrIntersectsCircle_Dateline(t *testing.T) {
	r := newTestRegion("dateline", datelineRegion(), datelineRegion())

	// Center just east of the dateline, above the region: the top edge is
	// ~133 km away.
	if !containsOrIntersectsCircle(orb.Point{179.6, 13.2}, 200000, r, false) {
		t.Error("edge within 200 km across the dateline must intersect")
	}
	if containsOrIntersectsCircle(orb.Point{179.6, 13.2}, 100000, r, false) {
		t.Error("edge beyond 100 km must not intersect")
	}
}

// =============================================================================
// Add Tests (full GeoJSON parse → index path)
// =============================================================================

func TestAdd_GeoJSONFeatureCollection(t *testing.T) {
	idx := newTestIndex()

	fc := &geojson.FeatureCollection{
		Features: []geojson.Feature{
			{
				Geometry: geojson.Geometry{
					Geometry: geom.Polygon{
						{{2, 2}, {8, 2}, {8, 8}, {2, 8}},
					},
				},
			},
		},
	}

	if err := idx.Add("from-geojson", fc, fc); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	res := idx.SearchByPoint(orb.Point{5, 5}, false)
	if len(res) != 1 || !hasResult(res, "from-geojson") {
		t.Errorf("point inside added region: got %v, want [from-geojson]", res)
	}
}

func TestAdd_GeoJSONMultiPolygon(t *testing.T) {
	idx := newTestIndex()

	fc := &geojson.FeatureCollection{
		Features: []geojson.Feature{
			{
				Geometry: geojson.Geometry{
					Geometry: geom.MultiPolygon{
						{
							{{2, 2}, {4, 2}, {4, 4}, {2, 4}},
						},
						{
							{{6, 6}, {8, 6}, {8, 8}, {6, 8}},
						},
					},
				},
			},
		},
	}

	if err := idx.Add("from-multipolygon", fc, fc); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	for _, p := range []orb.Point{{3, 3}, {7, 7}} {
		res := idx.SearchByPoint(p, false)
		if len(res) != 1 || !hasResult(res, "from-multipolygon") {
			t.Errorf("point %v inside multi-polygon: got %v, want [from-multipolygon]", p, res)
		}
	}

	// Point in the gap between the two polygons: inside the union bounding
	// box but inside neither polygon, so the containment filter must reject it.
	res := idx.SearchByPoint(orb.Point{5, 5}, false)
	if len(res) != 0 {
		t.Errorf("point between polygons: got %v, want none", res)
	}
}

func TestAdd_NoPolygons(t *testing.T) {
	idx := newTestIndex()

	// A feature collection whose only geometry is not a (multi)polygon must
	// be rejected rather than producing an empty region shape.
	fc := &geojson.FeatureCollection{
		Features: []geojson.Feature{
			{
				Geometry: geojson.Geometry{
					Geometry: geom.Point{2, 2},
				},
			},
		},
	}

	if err := idx.Add("point", fc, fc); err == nil {
		t.Fatal("expected error for feature collection without polygons")
	}
}
