package desktop

import "testing"

var testSurface = inputTarget{x: -1470, y: 0, w: 1470, h: 956, ok: true}

func TestInputMap_Point(t *testing.T) {
	for _, c := range []struct{ nx, ny, x, y float64 }{
		{0, 0, -1470, 0}, {1, 1, -1, 955}, {0.5, 0.25, -735, 239},
	} {
		if x, y := mapPoint(testSurface, c.nx, c.ny); x != c.x || y != c.y {
			t.Errorf("mapPoint(%v, %v) = (%v, %v), want (%v, %v)", c.nx, c.ny, x, y, c.x, c.y)
		}
	}
}

func TestInputMap_ClickCount(t *testing.T) {
	for _, c := range []struct {
		name                  string
		prevTS, ts            float64
		prevX, prevY, x, y    float64
		prevButton, btn, prev int
		want                  int
	}{
		{"first down", 0, 1000, 0, 0, 0, 0, 0, 0, 0, 1},
		{"second at 499 ms", 1000, 1499, 10, 10, 10, 10, 0, 0, 1, 2},
		{"second at 501 ms", 1000, 1501, 10, 10, 10, 10, 0, 0, 1, 1},
		{"second 4 points away", 1000, 1100, 10, 10, 14, 10, 0, 0, 1, 2},
		{"second 5 points away", 1000, 1100, 10, 10, 15, 10, 0, 0, 1, 1},
		{"other button", 1000, 1100, 10, 10, 10, 10, 0, 2, 1, 1},
		{"third", 1000, 1100, 10, 10, 10, 10, 0, 0, 2, 3},
		{"fourth stays three", 1000, 1100, 10, 10, 10, 10, 0, 0, 3, 3},
		{"no timestamp", 1000, 0, 10, 10, 10, 10, 0, 0, 1, 1},
		{"time went backwards", 1000, 900, 10, 10, 10, 10, 0, 0, 1, 1},
	} {
		got := nextClickCount(c.prevTS, c.ts, c.prevX, c.prevY, c.x, c.y, c.prevButton, c.btn, c.prev)
		if got != c.want {
			t.Errorf("%s: nextClickCount = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestInputMap_Scroll(t *testing.T) {
	for _, c := range []struct {
		dx, dy float64
		wx, wy int
	}{
		{0, 120, 0, -120}, {-40, 0, 40, 0}, {0, 5000, 0, -2400}, {0, -5000, 0, 2400}, {0.4, 0.4, 0, 0},
	} {
		if x, y := scrollDelta(c.dx, c.dy); x != c.wx || y != c.wy {
			t.Errorf("scrollDelta(%v, %v) = (%d, %d), want (%d, %d)", c.dx, c.dy, x, y, c.wx, c.wy)
		}
	}
}

func TestInputMap_Relative(t *testing.T) {
	s := inputTarget{w: 1000, h: 800, ok: true}
	for _, c := range []struct{ fx, fy, dx, dy, x, y float64 }{
		{100, 100, 0.1, 0, 200, 100},
		{950, 100, 0.2, 0, 999, 100},
		{100, 100, -1, -1, 0, 0},
		{100, 700, 0, 0.5, 100, 799},
	} {
		if x, y := integrateRelative(s, c.fx, c.fy, c.dx, c.dy); x != c.x || y != c.y {
			t.Errorf("integrateRelative from (%v, %v) by (%v, %v) = (%v, %v), want (%v, %v)", c.fx, c.fy, c.dx, c.dy, x, y, c.x, c.y)
		}
	}
	// A surface left of the main display clamps inside its own bounds.
	if x, _ := integrateRelative(testSurface, -10, 5, 1, 0); x != -1 {
		t.Errorf("x past the right edge of a negative-origin surface = %v, want -1", x)
	}
}
