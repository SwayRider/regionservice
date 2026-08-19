package index

import (
	"testing"

	"github.com/paulmach/orb"
)

// =============================================================================
// segmentIntersectsBox Tests
// =============================================================================

func TestSegmentIntersectsBox(t *testing.T) {
	bl := orb.Point{0, 0}
	tr := orb.Point{10, 10}

	tests := []struct {
		name string
		a, b orb.Point
		want bool
	}{
		{"crossing through middle", orb.Point{5, -1}, orb.Point{5, 11}, true},
		{"diagonal crossing", orb.Point{-1, -1}, orb.Point{11, 11}, true},
		{"fully inside", orb.Point{1, 1}, orb.Point{9, 9}, true},
		{"fully outside right", orb.Point{11, 1}, orb.Point{12, 2}, false},
		{"fully outside top", orb.Point{1, 11}, orb.Point{9, 12}, false},
		{"parallel outside", orb.Point{-1, 11}, orb.Point{11, 11}, false},
		{"parallel on boundary", orb.Point{-1, 10}, orb.Point{11, 10}, true},
		{"vertical on right edge", orb.Point{10, 1}, orb.Point{10, 9}, true},
		{"horizontal on bottom edge", orb.Point{1, 0}, orb.Point{9, 0}, true},
		{"touches corner", orb.Point{-1, -1}, orb.Point{0, 0}, true},
		{"corner outside", orb.Point{-1, -1}, orb.Point{-0.5, -0.5}, false},
		{"degenerate inside", orb.Point{5, 5}, orb.Point{5, 5}, true},
		{"degenerate outside", orb.Point{15, 15}, orb.Point{15, 15}, false},
		{"crosses left edge only", orb.Point{-5, 5}, orb.Point{0.5, 5}, true},
		{"stops before box", orb.Point{-5, 5}, orb.Point{-0.5, 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segmentIntersectsBox(tt.a, tt.b, bl, tr)
			if got != tt.want {
				t.Errorf("segmentIntersectsBox(%v, %v) = %v, want %v",
					tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// =============================================================================
// polygonIntersectsBox Tests
// =============================================================================

func TestPolygonIntersectsBox_EdgeCrossing(t *testing.T) {
	// Triangle with every vertex outside the box, but two edges crossing it.
	mp := orb.MultiPolygon{{{
		{5, -1}, {7, -1}, {6, 11},
	}}}
	if !polygonIntersectsBox(mp, orb.Point{0, 0}, orb.Point{10, 10}) {
		t.Error("edge crossing the box must intersect")
	}
}

func TestPolygonIntersectsBox_NoIntersection(t *testing.T) {
	mp := orb.MultiPolygon{{{
		{20, 20}, {21, 20}, {20.5, 21},
	}}}
	if polygonIntersectsBox(mp, orb.Point{0, 0}, orb.Point{10, 10}) {
		t.Error("disjoint polygon must not intersect")
	}
}

func TestPolygonIntersectsBox_EdgeTouchesBoundary(t *testing.T) {
	// Edge lies exactly on the box boundary.
	mp := orb.MultiPolygon{{{
		{10, 1}, {10, 9}, {11, 5},
	}}}
	if !polygonIntersectsBox(mp, orb.Point{0, 0}, orb.Point{10, 10}) {
		t.Error("edge touching the box boundary must intersect")
	}
}

func TestPolygonIntersectsBox_HoleEdgeCrossing(t *testing.T) {
	// A ring with a hole; the hole's edge crosses the box.
	mp := orb.MultiPolygon{{{
		{-5, -5}, {15, -5}, {15, 15}, {-5, 15},
	}, {
		{5, -1}, {7, -1}, {6, 11},
	}}}
	if !polygonIntersectsBox(mp, orb.Point{0, 0}, orb.Point{10, 10}) {
		t.Error("hole edge crossing the box must intersect")
	}
}

func TestPolygonIntersectsBox_Empty(t *testing.T) {
	if polygonIntersectsBox(nil, orb.Point{0, 0}, orb.Point{10, 10}) {
		t.Error("empty multipolygon must not intersect")
	}
}

// =============================================================================
// polygonIntersectsCircle Tests
// =============================================================================

func TestPolygonIntersectsCircle_EdgeWithinRadius(t *testing.T) {
	// Vertical edges at lon 0.006° (~667 m) and 0.007° (~778 m); vertices at
	// ±0.012° lat (~1336 m). Radius 800 m reaches the edges but no vertex.
	mp := orb.MultiPolygon{{{
		{0.006, 0.012}, {0.007, 0.012}, {0.007, -0.012}, {0.006, -0.012},
	}}}
	if !polygonIntersectsCircle(mp, orb.Point{0, 0}, 800) {
		t.Error("edge passing within radius must intersect")
	}
	if polygonIntersectsCircle(mp, orb.Point{0, 0}, 600) {
		t.Error("edge beyond radius must not intersect")
	}
}

func TestPolygonIntersectsCircle_VertexWithinRadius(t *testing.T) {
	mp := orb.MultiPolygon{{{
		{0.005, 0.001}, {0.006, 0.001}, {0.006, 0.002}, {0.005, 0.002},
	}}}
	if !polygonIntersectsCircle(mp, orb.Point{0, 0}, 800) {
		t.Error("vertex within radius must intersect")
	}
}

func TestPolygonIntersectsCircle_FarAway(t *testing.T) {
	mp := orb.MultiPolygon{{{
		{1, 1}, {1.1, 1}, {1.1, 1.1}, {1, 1.1},
	}}}
	if polygonIntersectsCircle(mp, orb.Point{0, 0}, 10000) {
		t.Error("far-away polygon must not intersect a 10 km radius")
	}
}

func TestPolygonIntersectsCircle_Touching(t *testing.T) {
	// Edge at ~111 m from the origin (0.001°), radius slightly larger.
	mp := orb.MultiPolygon{{{
		{0.001, 0.001}, {0.001, -0.001}, {0.002, 0},
	}}}
	if !polygonIntersectsCircle(mp, orb.Point{0, 0}, 115) {
		t.Error("edge touching the circle must intersect")
	}
	if polygonIntersectsCircle(mp, orb.Point{0, 0}, 100) {
		t.Error("edge beyond the circle must not intersect")
	}
}

func TestPolygonIntersectsCircle_Dateline(t *testing.T) {
	// Region crossing the antimeridian; edge within radius of a center on
	// the other side of the dateline.
	mp := orb.MultiPolygon{orb.Polygon{datelineRing()}}
	if !polygonIntersectsCircle(mp, orb.Point{179.6, 13.2}, 200000) {
		t.Error("edge within 200 km across the dateline must intersect")
	}
	if polygonIntersectsCircle(mp, orb.Point{179.6, 13.2}, 100000) {
		t.Error("edge beyond 100 km must not intersect")
	}
}
