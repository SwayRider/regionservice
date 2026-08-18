package index

import (
	"math"
	"testing"

	"github.com/paulmach/orb"
)

// =============================================================================
// wrapLon Tests
// =============================================================================

func TestWrapLon(t *testing.T) {
	tests := []struct {
		name string
		lon  float64
		want float64
	}{
		{"in range", 45, 45},
		{"negative in range", -45, -45},
		{"boundary max", 180, -180}, // ±180 are the same meridian; returns in [-180, 180)
		{"boundary min", -180, -180},
		{"above max", 181.5, -178.5},
		{"below min", -181.5, 178.5},
		{"full wrap", 360, 0},
		{"full wrap negative", -360, 0},
		{"multiple wraps", 540, -180},
		{"near dateline east", 179.9, 179.9},
		{"near dateline west", -179.9, -179.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLon(tt.lon)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("wrapLon(%v) = %v, want %v", tt.lon, got, tt.want)
			}
		})
	}
}

// =============================================================================
// unwrapPoint Tests
// =============================================================================

func TestUnwrapPoint(t *testing.T) {
	tests := []struct {
		name    string
		p       orb.Point
		refLon  float64
		wantLon float64
	}{
		{"already near ref", orb.Point{45, 10}, 40, 45},
		{"east side of dateline stays", orb.Point{179.5, 10}, 179.8, 179.5},
		{"west side shifted east", orb.Point{-179.5, 10}, 179.8, 180.5},
		{"west side stays", orb.Point{-179.5, 10}, -179.8, -179.5},
		{"east side shifted west", orb.Point{179.5, 10}, -179.8, -180.5},
		{"far west shifted east", orb.Point{-100, 10}, 120, 260},
		{"far east shifted west", orb.Point{100, 10}, -120, -260},
		{"exactly 180 away", orb.Point{-179, 10}, 1, 181},
		{"negative ref far east", orb.Point{170, 10}, -170, -190},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unwrapPoint(tt.p, tt.refLon)
			if math.Abs(got[0]-tt.wantLon) > 1e-9 {
				t.Errorf("unwrapPoint(%v, %v)[0] = %v, want %v",
					tt.p, tt.refLon, got[0], tt.wantLon)
			}
			if got[1] != tt.p[1] {
				t.Errorf("unwrapPoint changed latitude: got %v, want %v", got[1], tt.p[1])
			}
		})
	}
}

// =============================================================================
// unwrapRing / unwrapMultiPolygon Tests
// =============================================================================

// maxLonJump returns the largest |Δlon| between consecutive ring vertices.
func maxLonJump(r orb.Ring) float64 {
	maxJump := 0.0
	for i := 0; i < len(r); i++ {
		jump := math.Abs(r[(i+1)%len(r)][0] - r[i][0])
		if jump > maxJump {
			maxJump = jump
		}
	}
	return maxJump
}

func TestUnwrapRing_DatelineContiguous(t *testing.T) {
	ring := datelineRing()
	// Raw ring has a ~359° jump across the dateline.
	if maxJump := maxLonJump(ring); maxJump < 300 {
		t.Fatalf("fixture ring should cross the dateline, max lon jump = %v", maxJump)
	}

	for _, refLon := range []float64{179.8, -179.8, 180, -180} {
		got := unwrapRing(ring, refLon)
		if maxJump := maxLonJump(got); maxJump > 180 {
			t.Errorf("unwrapRing(_, %v) not contiguous: max lon jump = %v, ring = %v",
				refLon, maxJump, got)
		}
		// Same number of vertices, latitudes preserved.
		if len(got) != len(ring) {
			t.Errorf("unwrapRing changed ring length: got %d, want %d", len(got), len(ring))
		}
		for i := range ring {
			if got[i][1] != ring[i][1] {
				t.Errorf("unwrapRing changed latitude at %d: got %v, want %v", i, got[i][1], ring[i][1])
			}
		}
	}
}

func TestUnwrapRing_NonCrossingUnchanged(t *testing.T) {
	ring := orb.Ring{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	got := unwrapRing(ring, 5)
	for i := range ring {
		if got[i] != ring[i] {
			t.Errorf("non-crossing ring changed at %d: got %v, want %v", i, got[i], ring[i])
		}
	}
}

func TestUnwrapMultiPolygon(t *testing.T) {
	mp := orb.MultiPolygon{
		orb.Polygon{datelineRing()},
		orb.Polygon{{{0, 0}, {10, 0}, {10, 10}, {0, 10}}},
	}
	got := unwrapMultiPolygon(mp, -179.8)
	if len(got) != 2 {
		t.Fatalf("expected 2 polygons, got %d", len(got))
	}
	if maxJump := maxLonJump(got[0][0]); maxJump > 180 {
		t.Errorf("dateline polygon not contiguous after unwrap: max jump = %v", maxJump)
	}
	if got[1][0][0] != mp[1][0][0] {
		t.Errorf("non-crossing polygon changed: got %v, want %v", got[1][0][0], mp[1][0][0])
	}
}

// =============================================================================
// unwrapIfNeeded Tests
// =============================================================================

func TestUnwrapIfNeeded_NoCopyWhenFarFromDateline(t *testing.T) {
	mp := orb.MultiPolygon{{{{0, 0}, {10, 0}, {10, 10}, {0, 10}}}}
	shape := testGeoShape(mp)

	got := shape.unwrapIfNeeded(5)
	if len(got) != 1 {
		t.Fatalf("expected 1 polygon, got %d", len(got))
	}
	// Same backing array → no copy was made.
	if &got[0][0][0] != &mp[0][0][0] {
		t.Error("unwrapIfNeeded copied geometry that needed no unwrap")
	}
}

func TestUnwrapIfNeeded_CopiesWhenDatelineInRange(t *testing.T) {
	mp := orb.MultiPolygon{orb.Polygon{datelineRing()}}
	shape := testGeoShape(mp)

	for _, refLon := range []float64{179.8, -179.8} {
		got := shape.unwrapIfNeeded(refLon)
		if &got[0][0][0] == &mp[0][0][0] {
			t.Errorf("unwrapIfNeeded(%v) reused geometry that needed unwrapping", refLon)
		}
		if maxJump := maxLonJump(got[0][0]); maxJump > 180 {
			t.Errorf("unwrapIfNeeded(%v) result not contiguous: max jump = %v", refLon, maxJump)
		}
	}
}

func TestUnwrapIfNeeded_NoCopyWhenRefLonFarAway(t *testing.T) {
	mp := orb.MultiPolygon{orb.Polygon{datelineRing()}}
	shape := testGeoShape(mp)

	// refLon 0 is outside the ring's quadrant boxes, so no candidate would
	// normally reach the exact check; the unwrap must still be a no-op.
	got := shape.unwrapIfNeeded(0)
	if &got[0][0][0] != &mp[0][0][0] {
		t.Error("unwrapIfNeeded copied geometry for a far-away reference longitude")
	}
}
