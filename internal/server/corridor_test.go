// corridor_test.go tests computeCorridorBoxes, the corridor bounding-box
// computation for FindRouteRegionPaths.

package server

import (
	"testing"

	"github.com/paulmach/orb"
)

// =============================================================================
// computeCorridorBoxes Tests
// =============================================================================

func TestComputeCorridorBoxes_NoCrossing(t *testing.T) {
	boxes := computeCorridorBoxes(orb.Point{0, 0}, orb.Point{1, 1}, 100)
	if len(boxes) != 1 {
		t.Fatalf("expected 1 box, got %d: %v", len(boxes), boxes)
	}

	bl, tr := boxes[0].bottomLeft, boxes[0].topRight
	if bl[0] >= 0 || tr[0] <= 1 {
		t.Errorf("lon extent %v..%v should contain the segment [0, 1]", bl[0], tr[0])
	}
	if bl[1] >= 0 || tr[1] <= 1 {
		t.Errorf("lat extent %v..%v should contain the segment [0, 1]", bl[1], tr[1])
	}
	// 100 km width → ~0.5° on each side.
	if width := tr[0] - bl[0]; width > 2.5 {
		t.Errorf("box width %v° is too large for a 100 km corridor", width)
	}
}

func TestComputeCorridorBoxes_EastboundCrossing(t *testing.T) {
	// Segment from lon 179 to -179 crosses the 180th meridian.
	boxes := computeCorridorBoxes(orb.Point{179, 10}, orb.Point{-179, 10.5}, 100)
	if len(boxes) != 2 {
		t.Fatalf("expected 2 boxes for a dateline crossing, got %d: %v", len(boxes), boxes)
	}

	east, west := boxes[0], boxes[1]
	if east.topRight[0] != 180 {
		t.Errorf("east box should end at the 180th meridian, got %v", east)
	}
	if west.bottomLeft[0] != -180 {
		t.Errorf("west box should start at the -180th meridian, got %v", west)
	}

	// Neither box may span more than a few degrees (regression for the
	// ~358° single-box over-approximation).
	for _, box := range boxes {
		if w := box.topRight[0] - box.bottomLeft[0]; w > 3 {
			t.Errorf("box width %v° is too large: %v", w, box)
		}
		if box.bottomLeft[0] < -180 || box.topRight[0] > 180 {
			t.Errorf("box outside valid longitude range: %v", box)
		}
	}
	// The corridor covers both sides of the dateline.
	if east.topRight[0]-east.bottomLeft[0] < 1 {
		t.Errorf("east box too narrow to cover the corridor: %v", east)
	}
	if west.topRight[0]-west.bottomLeft[0] < 1 {
		t.Errorf("west box too narrow to cover the corridor: %v", west)
	}
}

func TestComputeCorridorBoxes_WestboundCrossing(t *testing.T) {
	// Segment from lon -179 to 179 crosses the 180th meridian westbound.
	boxes := computeCorridorBoxes(orb.Point{-179, 10}, orb.Point{179, 10.5}, 100)
	if len(boxes) != 2 {
		t.Fatalf("expected 2 boxes for a dateline crossing, got %d: %v", len(boxes), boxes)
	}

	for _, box := range boxes {
		if w := box.topRight[0] - box.bottomLeft[0]; w > 3 {
			t.Errorf("box width %v° is too large: %v", w, box)
		}
		if box.bottomLeft[0] < -180 || box.topRight[0] > 180 {
			t.Errorf("box outside valid longitude range: %v", box)
		}
	}
}

func TestComputeCorridorBoxes_NoCrossingNearDateline(t *testing.T) {
	// Segment that approaches but does not cross the dateline.
	boxes := computeCorridorBoxes(orb.Point{179, 10}, orb.Point{179.5, 10.5}, 100)
	if len(boxes) != 1 {
		t.Fatalf("expected 1 box, got %d: %v", len(boxes), boxes)
	}
	bl, tr := boxes[0].bottomLeft, boxes[0].topRight
	if tr[0] > 180.5 {
		t.Errorf("box should not extend far past the dateline, got %v..%v", bl[0], tr[0])
	}
	if bl[0] < 178 {
		t.Errorf("box should start near the segment, got %v", bl[0])
	}
}
