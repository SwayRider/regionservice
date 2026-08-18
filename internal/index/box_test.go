package index

import (
	"testing"

	"github.com/paulmach/orb"
)

// =============================================================================
// BoxLocation.HasPoint Tests
// =============================================================================

func TestBoxLocation_HasPoint(t *testing.T) {
	tests := []struct {
		name     string
		location BoxLocation
		pt       orb.Point
		want     bool
	}{
		// NW: lon <= 0, lat >= 0
		{"NW inside", NW, orb.Point{-10, 10}, true},
		{"NW on lon boundary", NW, orb.Point{0, 45}, true},
		{"NW on lat boundary", NW, orb.Point{-90, 0}, true},
		{"NW positive lon", NW, orb.Point{10, 10}, false},
		{"NW negative lat", NW, orb.Point{-10, -10}, false},

		// NE: lon >= 0, lat >= 0
		{"NE inside", NE, orb.Point{10, 10}, true},
		{"NE on lon boundary", NE, orb.Point{0, 45}, true},
		{"NE negative lon", NE, orb.Point{-10, 10}, false},
		{"NE negative lat", NE, orb.Point{10, -10}, false},

		// SW: lon <= 0, lat <= 0
		{"SW inside", SW, orb.Point{-10, -10}, true},
		{"SW on boundaries", SW, orb.Point{0, 0}, true},
		{"SW positive lon", SW, orb.Point{10, -10}, false},
		{"SW positive lat", SW, orb.Point{-10, 10}, false},

		// SE: lon >= 0, lat <= 0
		{"SE inside", SE, orb.Point{10, -10}, true},
		{"SE on boundaries", SE, orb.Point{0, 0}, true},
		{"SE negative lon", SE, orb.Point{-10, -10}, false},
		{"SE positive lat", SE, orb.Point{10, 10}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.HasPoint(tt.pt)
			if got != tt.want {
				t.Errorf("HasPoint(%v) = %v, want %v", tt.pt, got, tt.want)
			}
		})
	}
}

// =============================================================================
// BoxLocation.TransformPoint Tests
// =============================================================================

func TestBoxLocation_TransformPoint(t *testing.T) {
	tests := []struct {
		name     string
		location BoxLocation
		pt       orb.Point
		wantLon  float64
	}{
		{"NE: point already in quadrant", NE, orb.Point{10, 10}, 10},
		{"NE: negative lon shifted +360", NE, orb.Point{-10, 10}, 350},
		{"NW: point already in quadrant", NW, orb.Point{-10, 10}, -10},
		{"NW: positive lon shifted -360", NW, orb.Point{10, 10}, -350},
		{"SE: point already in quadrant", SE, orb.Point{10, -10}, 10},
		{"SE: negative lon shifted +360", SE, orb.Point{-10, -10}, 350},
		{"SW: point already in quadrant", SW, orb.Point{-10, -10}, -10},
		{"SW: positive lon shifted -360", SW, orb.Point{10, -10}, -350},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.TransformPoint(tt.pt)
			if got[0] != tt.wantLon {
				t.Errorf("TransformPoint(%v).X = %v, want %v", tt.pt, got[0], tt.wantLon)
			}
			if got[1] != tt.pt[1] {
				t.Errorf("TransformPoint(%v).Y changed: got %v, want %v", tt.pt, got[1], tt.pt[1])
			}
		})
	}
}

// =============================================================================
// Box Tests
// =============================================================================

func TestBox_Size_Empty(t *testing.T) {
	b := NewBox(NE)
	if got := b.Size(); got != 0 {
		t.Errorf("empty box Size() = %v, want 0", got)
	}
}

func TestBox_IsEmpty(t *testing.T) {
	if !NewBox(NE).IsEmpty() {
		t.Error("new box must be empty")
	}
	b := NewBox(NE)
	b.Add(orb.Point{10, 20})
	if b.IsEmpty() {
		t.Error("box with a point must not be empty")
	}
}

func TestBox_IsEmpty_Degenerate(t *testing.T) {
	// Points sharing one coordinate give a zero-area box: not empty.
	b := NewBox(NE)
	b.Add(orb.Point{10, 20})
	b.Add(orb.Point{10, 30})
	if b.IsEmpty() {
		t.Error("degenerate (zero-area) box must not be empty")
	}
	if got := b.Size(); got != 0 {
		t.Errorf("degenerate box Size() = %v, want 0", got)
	}
}

func TestBox_Add_And_Size(t *testing.T) {
	b := NewBox(NE)
	b.Add(orb.Point{10, 20})
	b.Add(orb.Point{30, 50})

	if got := b.Size(); got <= 0 {
		t.Errorf("non-empty box Size() = %v, want > 0", got)
	}

	bounds := b.Bounds()
	if bounds.Min.X() != 10 || bounds.Min.Y() != 20 {
		t.Errorf("Min = %v, want [10 20]", bounds.Min)
	}
	if bounds.Max.X() != 30 || bounds.Max.Y() != 50 {
		t.Errorf("Max = %v, want [30 50]", bounds.Max)
	}
}

func TestBox_Add_IgnoresOutOfQuadrant(t *testing.T) {
	b := NewBox(NE)           // NE: lon >= 0, lat >= 0
	b.Add(orb.Point{-10, 10}) // negative lon → ignored
	if got := b.Size(); got != 0 {
		t.Errorf("box should be empty after out-of-quadrant point, Size() = %v", got)
	}
}

func TestBox_SizeCalculation(t *testing.T) {
	b := NewBox(NE)
	b.Add(orb.Point{0, 0})
	b.Add(orb.Point{2, 3})
	want := 2.0 * 3.0
	if got := b.Size(); got != want {
		t.Errorf("Size() = %v, want %v", got, want)
	}
}

// =============================================================================
// Bounds.Extend Tests
// =============================================================================

func TestBounds_Extend(t *testing.T) {
	b1 := NewBounds()
	b1.Add(orb.Point{10, 20}) // NE quadrant
	b1.Add(orb.Point{12, 22}) // gives NE a non-zero area

	b2 := NewBounds()
	b2.Add(orb.Point{30, 40}) // NE quadrant
	b2.Add(orb.Point{35, 45}) // gives NE a non-zero area

	b1.Extend(b2)

	if b1.NE.Size() == 0 {
		t.Error("NE box should be non-empty after Extend")
	}
	bounds := b1.NE.Bounds()
	if bounds.Min.X() != 10 {
		t.Errorf("after Extend, NE.Min.X = %v, want 10", bounds.Min.X())
	}
	if bounds.Max.X() != 35 {
		t.Errorf("after Extend, NE.Max.X = %v, want 35", bounds.Max.X())
	}
}

func TestBounds_Extend_EmptyOther(t *testing.T) {
	b1 := NewBounds()
	b1.Add(orb.Point{10, 20})
	origSize := b1.NE.Size()

	b1.Extend(NewBounds()) // extend with empty bounds — should not change
	if b1.NE.Size() != origSize {
		t.Errorf("Extend with empty bounds changed NE size: got %v, want %v", b1.NE.Size(), origSize)
	}
}

func TestBounds_Extend_Degenerate(t *testing.T) {
	// A single point in a quadrant yields a degenerate box; it must still
	// propagate on Extend so axis-aligned geometries stay queryable.
	b1 := NewBounds()
	b2 := NewBounds()
	b2.Add(orb.Point{10, 20})

	b1.Extend(b2)
	if b1.NE.IsEmpty() {
		t.Error("degenerate quadrant box must propagate on Extend")
	}
	if got := b1.NE.Size(); got != 0 {
		t.Errorf("single-point box Size() = %v, want 0", got)
	}

	// Degenerate line boxes propagate too.
	b3 := NewBounds()
	b3.Add(orb.Point{30, 40})
	b3.Add(orb.Point{30, 50})
	b1.Extend(b3)
	if b1.NE.IsEmpty() {
		t.Error("degenerate line box must propagate on Extend")
	}
	bounds := b1.NE.Bounds()
	if bounds.Min.X() != 10 || bounds.Max.X() != 30 {
		t.Errorf("NE lon extent = %v..%v, want [10, 30]", bounds.Min.X(), bounds.Max.X())
	}
	if bounds.Min.Y() != 20 || bounds.Max.Y() != 50 {
		t.Errorf("NE lat extent = %v..%v, want [20, 50]", bounds.Min.Y(), bounds.Max.Y())
	}
}

// =============================================================================
// Rect Tests
// =============================================================================

func TestRect_Contains(t *testing.T) {
	outer := NewRect(orb.Point{0, 0}, orb.Point{10, 10})
	inner := NewRect(orb.Point{2, 2}, orb.Point{8, 8})
	partial := NewRect(orb.Point{5, 5}, orb.Point{15, 15})

	if !outer.Contains(inner) {
		t.Error("outer should contain inner")
	}
	if outer.Contains(partial) {
		t.Error("outer should not contain partial overlap")
	}
	if inner.Contains(outer) {
		t.Error("inner should not contain outer")
	}
}

func TestRect_Within(t *testing.T) {
	outer := NewRect(orb.Point{0, 0}, orb.Point{10, 10})
	inner := NewRect(orb.Point{2, 2}, orb.Point{8, 8})

	if !inner.Within(outer) {
		t.Error("inner should be within outer")
	}
	if outer.Within(inner) {
		t.Error("outer should not be within inner")
	}
}

func TestRect_Intersects(t *testing.T) {
	r1 := NewRect(orb.Point{0, 0}, orb.Point{5, 5})
	r2 := NewRect(orb.Point{3, 3}, orb.Point{8, 8})   // overlaps
	r3 := NewRect(orb.Point{6, 6}, orb.Point{10, 10}) // no overlap

	if !r1.Intersects(r2) {
		t.Error("r1 and r2 should intersect")
	}
	if r1.Intersects(r3) {
		t.Error("r1 and r3 should not intersect")
	}
}
