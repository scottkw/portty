package desktop

// input_map.go: the pure functions and limits behind input handling, each
// table-tested without a bridge.

import (
	"math"
	"time"
)

const (
	doubleClickInterval = 500 // ms between viewer timestamps that still count as a repeat click
	clickRadius         = 4   // points between two downs that still count as a repeat click
	maxWheel            = 2400
	inputRate           = 400 // messages per second per viewer (presses, moves, wheel, text)
	inputBurst          = 200

	deadManAfter      = 3 * time.Second // a viewer holding something and silent this long is released
	inputTick         = 250 * time.Millisecond
	optimisticLock    = 3 * time.Second // presses refused after the Lock Screen chord, until Locked() catches up
	cursorReportEvery = 33 * time.Millisecond
)

// mapPoint maps normalized (nx, ny) to the nearest global point, at most one
// point inside the far edge.
func mapPoint(t inputTarget, nx, ny float64) (x, y float64) {
	return t.x + min(math.Round(nx*t.w), t.w-1), t.y + min(math.Round(ny*t.h), t.h-1)
}

// integrateRelative moves from (fromX, fromY) by the normalized delta (ndx, ndy)
// of the surface and clamps the result to the surface.
func integrateRelative(t inputTarget, fromX, fromY, ndx, ndy float64) (x, y float64) {
	x = math.Max(t.x, math.Min(t.x+t.w-1, math.Round(fromX+ndx*t.w)))
	y = math.Max(t.y, math.Min(t.y+t.h-1, math.Round(fromY+ndy*t.h)))
	return x, y
}

// nextClickCount is the click state of a button-down from viewer timestamps (ms)
// and mapped points, never from arrival time: a second down of the same button
// within doubleClickInterval and clickRadius of the previous one is the next
// click, up to 3, anything else starts again at 1. prev is the previous down's
// count (0 when there was none); a missing or backwards timestamp starts again.
func nextClickCount(prevTS, ts, prevX, prevY, x, y float64, prevButton, button, prev int) int {
	if prev == 0 || button != prevButton || ts <= 0 || prevTS <= 0 {
		return 1
	}
	if dt := ts - prevTS; dt < 0 || dt > doubleClickInterval {
		return 1
	}
	if math.Hypot(x-prevX, y-prevY) > clickRadius {
		return 1
	}
	return min(prev+1, 3)
}

// scrollDelta turns browser wheel pixels into CoreGraphics ones: the sign is
// inverted (a browser's positive dy scrolls content up) and each axis is
// clamped to maxWheel.
func scrollDelta(dx, dy float64) (cgDX, cgDY int) {
	f := func(v float64) int {
		return int(math.Max(-maxWheel, math.Min(maxWheel, -math.Round(v))))
	}
	return f(dx), f(dy)
}
