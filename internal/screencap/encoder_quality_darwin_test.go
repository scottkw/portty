package screencap

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

// The rate run: noisy frames (so the encoder can use whatever rate it is given)
// at 30 fps for 4 s, 8 Mbps for the first 2 s and 1 Mbps after a live change.
const (
	rateFPS      = 30
	rateFrames   = 4 * rateFPS
	rateChangeAt = 2 * rateFPS
	rateHigh     = 8_000_000
	rateLow      = 1_000_000
)

var (
	rateOnce  sync.Once
	rateUnits []AccessUnit
	rateErr   error
)

// rateRun runs the synthetic rate encode once and shares the result.
func rateRun(t *testing.T) []AccessUnit {
	t.Helper()
	rateOnce.Do(func() {
		rateUnits, rateErr = encodeSyntheticRateForTest(rateFrames, 1280, 720, rateFPS, rateChangeAt, rateHigh, rateLow, -1, time.Second/rateFPS)
	})
	if errors.Is(rateErr, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", rateErr)
	}
	if rateErr != nil {
		t.Fatal(rateErr)
	}
	return rateUnits
}

func sumBytes(units []AccessUnit) (n int) {
	for _, u := range units {
		n += len(u.Data)
	}
	return n
}

// A live bitrate change on the running session lowers the produced rate: the
// last second at 1 Mbps carries at most half the bytes per second of the first
// two seconds at 8 Mbps (A-apply; RESEARCH assumption A7).
func TestEncoderRate_LiveChangeLowersRate(t *testing.T) {
	units := rateRun(t)
	if len(units) != rateFrames {
		t.Fatalf("got %d units for %d frames; per-frame accounting needs one unit per frame", len(units), rateFrames)
	}
	before := float64(sumBytes(units[:rateChangeAt])) / 2  // bytes per second over the first 2 s
	after := float64(sumBytes(units[rateFrames-rateFPS:])) // bytes in the last second
	t.Logf("bytes_per_s before=%.0f (%.2f Mbps) after=%.0f (%.2f Mbps) ratio=%.3f", before, before*8/1e6, after, after*8/1e6, after/before)
	if after > before/2 {
		t.Fatalf("after the change to %d bps the rate is %.0f B/s, want at most half of %.0f B/s", rateLow, after, before)
	}
}

// The change is a property set on the live session, not a new session: no unit
// after the first is a keyframe (a new session would start with one).
func TestEncoderRate_SessionIsNotRecreated(t *testing.T) {
	units := rateRun(t)
	if !units[0].Keyframe {
		t.Fatal("the first unit is not a keyframe")
	}
	for i, u := range units[1:] {
		if u.Keyframe {
			t.Fatalf("unit %d is an unrequested keyframe: the session was recreated", i+1)
		}
	}
}

// A resolution step rebuilds the encoder; the rebuilt session keeps the live rate
// (its first unit is the keyframe a rebuild promises, and the rate stays low).
func TestEncoderRate_RebuildKeepsRate(t *testing.T) {
	const rebuildAt = rateChangeAt + rateFPS/2
	units, err := encodeSyntheticRateForTest(rateFrames, 1280, 720, rateFPS, rateChangeAt, rateHigh, rateLow, rebuildAt, time.Second/rateFPS)
	if errors.Is(err, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(units) < rateFrames {
		t.Fatalf("got %d units for %d frames", len(units), rateFrames)
	}
	after := float64(sumBytes(units[len(units)-rateFPS:]))
	before := float64(sumBytes(units[:rateChangeAt])) / 2
	t.Logf("bytes_per_s before=%.0f after_rebuild=%.0f ratio=%.3f", before, after, after/before)
	if after > before/2 {
		t.Fatalf("a rebuilt session runs at %.0f B/s, want at most half of %.0f B/s: it did not keep the live rate", after, before)
	}
}

// SetBitrate and SetFPS on a stopped source say so; SetFPS touches the live
// stream, so even a source that believes it runs refuses it inside a test.
func TestEncoderRate_StoppedAndTestRefusals(t *testing.T) {
	d := &DisplaySource{}
	if err := d.SetBitrate(rateLow); !errors.Is(err, ErrNotRunning) {
		t.Errorf("SetBitrate on a stopped source = %v, want ErrNotRunning", err)
	}
	if err := d.SetFPS(15); !errors.Is(err, ErrNotRunning) {
		t.Errorf("SetFPS on a stopped source = %v, want ErrNotRunning", err)
	}
	d.running = true
	if err := d.SetFPS(15); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("SetFPS under a test = %v, want ErrRefusedInTest", err)
	}
	d.running = false
}

// Still-screen refinement (DESK-07): a synthetic text chart scrolls for a while at
// 1 Mbps, then the screen goes still and the cached last frame is refined once
// and refreshed once, through the real encoder, and decoded again to be measured.
// Synthetic frames only: no capture, no TCC.
const (
	stillW, stillH = 1920, 1080
	stillFrames    = 12      // moving frames before the screen goes still
	stillMovingBps = 1000000 // the moving-state target
	stillRefineBps = 4000000 // refine's bound, 4 times the moving target
)

// Refine mechanisms the synthetic driver can run: the bitrate raise plus a forced
// keyframe alone, the one-frame Quality property, and the raise plus a keyframe
// followed by two P-frames of the same picture (the product's: the keyframe's
// quality does not follow the raised rate, the P-frames after it do).
const (
	refineBitrate  = 0
	refineQuality  = 1
	refineRepeated = 2
	refineDefault  = -1 // whatever pc_encoder_refine uses in the product
)

// textChartLuma is one video-range luma frame of small dark strokes on white
// (glyph-like cells of 8 x 12 px with word and line gaps), scrolled 3 px per frame
// so successive frames differ.
func textChartLuma(w, h, frame int) []byte {
	const cellW, cellH = 8, 12
	y := make([]byte, w*h)
	for i := range y {
		y[i] = 235
	}
	shift := frame * 3
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			sc := c + shift
			cx, cy := sc/cellW, r/cellH
			if cx%7 == 6 || cy%2 == 1 { // word and line gaps
				continue
			}
			x := uint32(cx*7919+cy*104729) | 1
			x ^= x << 13
			x ^= x >> 17
			x ^= x << 5
			lx, ly := sc%cellW, r%cellH
			// a stem 2 px wide, a bar, and an optional diagonal
			stem := int(x%5) + 1
			var v byte = 235
			switch {
			case ly > 1 && ly < 10 && (lx == stem || lx == stem+1):
				v = 16
			case ly == int((x>>8)%6)+2 && lx > 0 && lx < 7:
				v = 24
			case x&0x10000 != 0 && lx == ly/2 && ly > 1 && ly < 10:
				v = 60
			}
			y[r*w+c] = v
		}
	}
	return y
}

// lumaPSNR is the PSNR in dB of b against a over the 8-bit luma range.
func lumaPSNR(a, b []byte) float64 {
	var se float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		se += d * d
	}
	if se == 0 {
		return 99
	}
	return 10 * math.Log10(255*255/(se/float64(len(a))))
}

// stillResult is one run of the still scenario, decoded. Unit order: stillFrames
// fresh, one forced keyframe, one refine, one idle.
type stillResult struct {
	units    []AccessUnit
	src      []byte   // the still frame's luma (the last moving frame)
	decoded  [][]byte // luma of every unit, in order
	avgAfter int      // the session's average bitrate after the refine
}

// Mechanism 2 emits three refine units (a keyframe and two P-frames); the others one.
func (r stillResult) fresh() []AccessUnit { return r.units[:stillFrames] }
func (r stillResult) forced() AccessUnit  { return r.units[stillFrames] }
func (r stillResult) refine() AccessUnit  { return r.units[stillFrames+1] }
func (r stillResult) lastRefine() int     { return len(r.units) - 2 }
func (r stillResult) idle() AccessUnit    { return r.units[len(r.units)-1] }

func runStill(t *testing.T, mech int) stillResult {
	t.Helper()
	var all []byte
	for i := 0; i < stillFrames; i++ {
		all = append(all, textChartLuma(stillW, stillH, i)...)
	}
	units, avg, err := encodeSyntheticStillForTest(stillW, stillH, rateFPS, stillMovingBps, all, stillFrames, stillRefineBps, mech)
	if errors.Is(err, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(units) < stillFrames+3 {
		t.Fatalf("mechanism %d: got %d units, want at least %d (fresh, forced, refine, idle)", mech, len(units), stillFrames+3)
	}
	dec, err := decodeSyntheticForTest(units)
	if err != nil {
		t.Fatal(err)
	}
	return stillResult{units: units, src: all[(stillFrames-1)*stillW*stillH:], decoded: dec, avgAfter: avg}
}

// psnrs reports the last moving frame's PSNR and the refine unit's, against the
// still frame, on a line that starts "psnr".
func (r stillResult) psnrs(t *testing.T, mech int) (moving, refined float64) {
	t.Helper()
	moving = lumaPSNR(r.src, r.decoded[stillFrames-1])
	refined = lumaPSNR(r.src, r.decoded[r.lastRefine()])
	t.Logf("psnr mechanism=%d moving_1mbps=%.2f dB refine=%.2f dB gain=%.2f dB idle_after=%.2f dB; bytes moving=%d refine=%d idle=%d",
		mech, moving, refined, refined-moving, lumaPSNR(r.src, r.decoded[len(r.units)-1]), len(r.units[stillFrames-1].Data), len(r.refine().Data), len(r.idle().Data))
	return moving, refined
}

// The refined keyframe of a still text chart is at least 6 dB sharper than the
// frame the moving state produced for it, and at least 38 dB against the source
// (RESEARCH Pitfall 4, A7 and A8). Every mechanism is measured and logged; the one
// the product uses (refineDefault) must clear the bar.
func TestEncoderRefine_KeyframeIsSharperThanMoving(t *testing.T) {
	for _, mech := range []int{refineBitrate, refineQuality, refineRepeated, refineDefault} {
		moving, refined := runStill(t, mech).psnrs(t, mech)
		if mech != refineDefault {
			continue
		}
		if refined < moving+6 || refined < 38 {
			t.Errorf("mechanism %d: refine %.2f dB against moving %.2f dB; want at least 6 dB above it and at least 38 dB", mech, refined, moving)
		}
	}
}

// An idle refresh is a small non-key unit: under a tenth of the refine unit.
func TestEncoderRefine_IdleRefreshIsSmall(t *testing.T) {
	r := runStill(t, refineDefault)
	idle, ref := r.idle(), r.refine()
	t.Logf("idle_bytes=%d refine_bytes=%d", len(idle.Data), len(ref.Data))
	if idle.Keyframe {
		t.Error("the idle refresh is a keyframe")
	}
	if len(idle.Data)*10 >= len(ref.Data) {
		t.Errorf("idle unit %d bytes against refine %d bytes: not under a tenth", len(idle.Data), len(ref.Data))
	}
}

// The raise lasts one frame: the session's average bitrate is the moving target again.
func TestEncoderRefine_TargetIsRestored(t *testing.T) {
	r := runStill(t, refineDefault)
	if r.avgAfter != stillMovingBps {
		t.Errorf("average bitrate after the refine is %d, want %d", r.avgAfter, stillMovingBps)
	}
}

// Every unit says how it came to be: captured frames are fresh, a join keyframe
// is forced, then refine and idle.
func TestEncoderRefine_UnitKinds(t *testing.T) {
	r := runStill(t, refineDefault)
	for i, u := range r.fresh() {
		if u.Kind != UnitFresh {
			t.Errorf("captured frame %d has kind %d, want UnitFresh", i, u.Kind)
		}
	}
	if u := r.forced(); u.Kind != UnitForced || !u.Keyframe {
		t.Errorf("join keyframe: kind %d keyframe %v, want UnitForced keyframe", u.Kind, u.Keyframe)
	}
	if u := r.refine(); u.Kind != UnitRefine || !u.Keyframe {
		t.Errorf("refine unit: kind %d keyframe %v, want UnitRefine keyframe", u.Kind, u.Keyframe)
	}
	if u := r.idle(); u.Kind != UnitIdle {
		t.Errorf("idle unit: kind %d, want UnitIdle", u.Kind)
	}
}

// Refine and IdleRefresh on a stopped source say so.
func TestEncoderRefine_StoppedSource(t *testing.T) {
	d := &DisplaySource{}
	if err := d.Refine(stillRefineBps); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Refine on a stopped source = %v, want ErrNotRunning", err)
	}
	if err := d.IdleRefresh(); !errors.Is(err, ErrNotRunning) {
		t.Errorf("IdleRefresh on a stopped source = %v, want ErrNotRunning", err)
	}
}
