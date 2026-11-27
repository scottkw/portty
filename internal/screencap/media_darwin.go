package screencap

// media_darwin.go: the cgo side of the encoder (and capture) in media_darwin.m.
// Every C prototype lives here, the ObjC in media_darwin.m. Frames stay inside
// ObjC from capture to encoder and cross into Go once per access unit.
//
// No minimum-version CFLAG here: Phase 18.1's build.sh sets the macOS 14
// deployment target for the whole build.

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework ScreenCaptureKit -framework VideoToolbox -framework CoreMedia -framework CoreVideo -framework CoreGraphics -framework ImageIO -framework Foundation

#include <stdint.h>
#include <stdlib.h>

int pc_encode_synthetic(uintptr_t handle, int frames, int width, int height, int force, int rebuildAt, int rebuildW, int rebuildH, int *hardware);
int pc_encode_synthetic_rate(uintptr_t handle, int frames, int width, int height, int fps, int changeAt, int startBps, int newBps, int rebuildAt, int paceUs, int *hardware);

// Capture. Each returns 0 or an error code (SCK codes pass through, else OSStatus);
// the char buffer receives a message. One stream per process: nativeMu serialises.
int pc_stream_start(uintptr_t handle, int kind, uint32_t id, int fps, int maxLong, int bitrate, double keyframeSecs, char *errbuf, int errlen);
int pc_stream_select(int kind, uint32_t id, int maxLong, char *errbuf, int errlen);
int pc_list_surfaces(char **json, int *n, char *errbuf, int errlen);
void pc_stream_stop(void);
int pc_encoder_force_keyframe(void);
int pc_encoder_rebuild(int w, int h);
int pc_encoder_set_bitrate(int bps);
int pc_stream_set_fps(int fps);
int pc_encoder_hardware(void);
int pc_still_capture(int maxLong, uint8_t **out, int *n, char *errbuf, int errlen);
uint32_t pc_main_display_id(void);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// errNoEncoder is VideoToolbox's kVTCouldNotFindVideoEncoderErr: this machine
// has no H.264 encoder at all (a bare CI runner). Tests skip on it only.
var errNoEncoder = errors.New("screencap: no H.264 encoder available")

// ScreenCaptureKit and VideoToolbox codes the bridge reports.
const (
	sckUserDeclined   = -3801 // SCStreamErrorUserDeclined: no Screen Recording grant
	sckNoDisplay      = -3815 // SCStreamErrorNoDisplayList: no display (asleep) or main display unlisted
	osStatusNoEncoder = -12908
)

// sink receives what the ObjC side produces. One per Start (or per synthetic
// run), reached from the callbacks through a cgo.Handle.
type sink struct {
	au   func(AccessUnit)
	err  func(error)
	last atomic.Int64 // unix nanos of the previous unit, for Duration
	// audio and audioErr receive the Opus packets and audio failures of
	// audio_darwin.go; nil for a sink that has no audio.
	audio    func(AudioPacket)
	audioErr func(error)
}

// lookup returns the sink behind a callback's handle, or nil if it is gone.
func lookup(h uintptr) (s *sink) {
	defer func() {
		if recover() != nil {
			s = nil
		}
	}()
	s, _ = cgo.Handle(h).Value().(*sink)
	return s
}

// mapSCKError turns a code from the ObjC side into a typed error: -3801 is
// ErrPermissionDenied, -3815 ErrNoDisplay, no encoder is errNoEncoder, and any
// other code is an error that names it.
func mapSCKError(code int, msg string) error {
	switch code {
	case sckUserDeclined:
		return fmt.Errorf("%w: %s", ErrPermissionDenied, msg)
	case sckNoDisplay:
		return fmt.Errorf("%w: %s", ErrNoDisplay, msg)
	case osStatusNoEncoder:
		return errNoEncoder
	}
	return fmt.Errorf("screencap: %s (code %d)", msg, code)
}

//export goScreencapAccessUnit
func goScreencapAccessUnit(h C.uintptr_t, avcc unsafe.Pointer, avccLen C.int, sps unsafe.Pointer, spsLen C.int, pps unsafe.Pointer, ppsLen C.int, keyframe C.int, width, height C.int) {
	// A panic on a framework thread would end the daemon.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: access unit callback: %v", r)
		}
	}()
	s := lookup(uintptr(h))
	if s == nil {
		return
	}
	key := keyframe != 0
	var params [][]byte
	if key {
		params = [][]byte{C.GoBytes(sps, spsLen), C.GoBytes(pps, ppsLen)}
	}
	data, err := avccToAnnexB(C.GoBytes(avcc, avccLen), params, key)
	if err != nil {
		s.err(err)
		return
	}
	// Variable-rate capture (frames only on change): the RTP clock follows real time.
	now := time.Now().UnixNano()
	dur := time.Second / DefaultFPS
	if prev := s.last.Swap(now); prev != 0 {
		dur = max(time.Duration(now-prev), time.Millisecond)
	}
	s.au(AccessUnit{Data: data, Keyframe: key, Width: int(width), Height: int(height), Duration: dur})
}

//export goScreencapStreamError
func goScreencapStreamError(h C.uintptr_t, code C.int, msg *C.char) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: error callback: %v", r)
		}
	}()
	if s := lookup(uintptr(h)); s != nil {
		s.err(mapSCKError(int(code), C.GoString(msg)))
	}
}

// syntheticHardware is the encoder readback of the last encodeSyntheticForTest:
// 1 hardware, 0 software, -1 unknown. Test-only diagnostic.
var syntheticHardware atomic.Int32

// encodeSyntheticForTest runs only the encoder on synthetic NV12 frames: no
// capture, no TCC. A negative force or rebuildAt disables that step.
func encodeSyntheticForTest(frames, width, height int, force, rebuildAt int, rebuildW, rebuildH int) ([]AccessUnit, error) {
	var units []AccessUnit
	var errs []error
	h := cgo.NewHandle(&sink{
		au:  func(au AccessUnit) { units = append(units, au) },
		err: func(err error) { errs = append(errs, err) },
	})
	defer h.Delete()
	var hw C.int = -1
	rc := C.pc_encode_synthetic(C.uintptr_t(h), C.int(frames), C.int(width), C.int(height), C.int(force), C.int(rebuildAt), C.int(rebuildW), C.int(rebuildH), &hw)
	syntheticHardware.Store(int32(hw))
	if rc != 0 {
		return nil, mapSCKError(int(rc), "synthetic encode failed")
	}
	// pc_encode_synthetic completes the session before it returns, so every
	// callback has run and the slices are quiescent.
	return units, errors.Join(errs...)
}

// encodeSyntheticRateForTest feeds noisy NV12 frames at fps through a real
// encoder started at startBps and changes the running session to newBps at frame
// changeAt and rebuilds it at the same size at rebuildAt (a negative index
// disables either), sleeping pace after each frame (zero feeds as fast as the
// encoder takes them). Test-only, like encodeSyntheticForTest: no capture, no TCC.
func encodeSyntheticRateForTest(frames, width, height, fps, changeAt, startBps, newBps, rebuildAt int, pace time.Duration) ([]AccessUnit, error) {
	var units []AccessUnit
	var errs []error
	h := cgo.NewHandle(&sink{
		au:  func(au AccessUnit) { units = append(units, au) },
		err: func(err error) { errs = append(errs, err) },
	})
	defer h.Delete()
	var hw C.int = -1
	rc := C.pc_encode_synthetic_rate(C.uintptr_t(h), C.int(frames), C.int(width), C.int(height), C.int(fps), C.int(changeAt), C.int(startBps), C.int(newBps), C.int(rebuildAt), C.int(pace.Microseconds()), &hw)
	syntheticHardware.Store(int32(hw))
	if rc != 0 {
		return nil, mapSCKError(int(rc), "synthetic rate encode failed")
	}
	return units, errors.Join(errs...)
}

// errBufLen is the size of the message buffer the C side fills on failure.
const errBufLen = 256

// stillCeiling is how long CaptureStill waits (UI-SPEC Test Capture give-up).
const stillCeiling = 15 * time.Second

// nativeMu serialises every call into the ObjC stream state, which is one per
// process (gStream, gEnc). d.mu alone is not enough: the Manager builds a fresh
// DisplaySource per run and an old run's start may still be in flight.
// Lock order: d.mu, then nativeMu.
var nativeMu sync.Mutex

// DisplaySource is the Source for a display or a window: ScreenCaptureKit
// capture at Retina size into a VideoToolbox H.264 encoder. Start, Stop, Rebuild and
// ForceKeyframe are serialised by mu per source and by nativeMu across
// sources, because the ObjC side holds one stream per process.
type DisplaySource struct {
	mu      sync.Mutex
	running bool
	handle  cgo.Handle
	surface Surface      // what is captured (running) or will be at the next Start
	maxLong int          // the size cap of the running capture, for a switch
	hw      atomic.Int32 // encoder readback: 1 hardware, 0 software, -1 unknown

	emitMu    sync.Mutex // guards closed and channel sends
	closed    bool
	frames    chan AccessUnit
	errs      chan error
	audio     chan AudioPacket
	audioErrs chan error
}

var (
	_ Source        = (*DisplaySource)(nil)
	_ AudioSource   = (*DisplaySource)(nil)
	_ QualitySource = (*DisplaySource)(nil)
)

// NewDisplaySource returns a Source that captures the main display until told otherwise.
func NewDisplaySource() Source {
	d := &DisplaySource{}
	d.hw.Store(-1)
	return d
}

// checkSurface accepts a display (ID 0 is the main display) or a window.
func checkSurface(s Surface) error {
	if s.Kind != SurfaceDisplay && s.Kind != SurfaceWindow {
		return ErrSurfaceUnsupported
	}
	return nil
}

// SelectSurface switches a running capture to the display or window s (the
// encoder is rebuilt at the new size and the next unit is a keyframe), and on a
// stopped source records s for the next Start. A failed switch leaves the old
// surface streaming.
func (d *DisplaySource) SelectSurface(s Surface) error {
	if err := checkSurface(s); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		d.surface = s
		return nil
	}
	if testing.Testing() {
		return ErrRefusedInTest
	}
	var msg [errBufLen]C.char
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if rc := C.pc_stream_select(C.int(s.Kind), C.uint32_t(s.ID), C.int(d.maxLong), &msg[0], errBufLen); rc != 0 {
		return mapSCKError(int(rc), C.GoString(&msg[0]))
	}
	d.surface = s
	d.noteEncoder()
	return nil
}

// Start captures cfg.Surface (the surface recorded by SelectSurface when cfg
// names none, else the main display) with cfg's fps, size cap, bitrate and
// keyframe ceiling (zero values take the defaults). It blocks until the stream
// is running or has failed; -3801 comes back as ErrPermissionDenied and -3815
// as ErrNoDisplay. Stop ends the capture; ctx is only checked before starting.
func (d *DisplaySource) Start(ctx context.Context, cfg Config) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkSurface(cfg.Surface); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return errors.New("screencap: display source already started")
	}
	if cfg.Surface != (Surface{}) {
		d.surface = cfg.Surface
	}
	def := DefaultConfig()
	if cfg.FPS <= 0 {
		cfg.FPS = def.FPS
	}
	if cfg.MaxLongEdge <= 0 {
		cfg.MaxLongEdge = def.MaxLongEdge
	}
	if cfg.BitrateBps <= 0 {
		cfg.BitrateBps = def.BitrateBps
	}
	if cfg.KeyframeInterval <= 0 {
		cfg.KeyframeInterval = def.KeyframeInterval
	}

	d.emitMu.Lock()
	d.closed = false
	d.frames = make(chan AccessUnit, 8)
	d.errs = make(chan error, 4)
	d.audio = make(chan AudioPacket, 16)
	d.audioErrs = make(chan error, 4)
	d.emitMu.Unlock()
	d.handle = cgo.NewHandle(&sink{au: d.emit, err: d.emitErr, audio: d.emitAudio, audioErr: d.emitAudioErr})

	var msg [errBufLen]C.char
	nativeMu.Lock()
	defer nativeMu.Unlock()
	rc := C.pc_stream_start(C.uintptr_t(d.handle), C.int(d.surface.Kind), C.uint32_t(d.surface.ID), C.int(cfg.FPS), C.int(cfg.MaxLongEdge), C.int(cfg.BitrateBps), C.double(cfg.KeyframeInterval.Seconds()), &msg[0], errBufLen)
	if rc != 0 {
		d.handle.Delete()
		d.closeChannels()
		d.emitMu.Lock()
		d.frames, d.errs, d.audio, d.audioErrs = nil, nil, nil, nil
		d.emitMu.Unlock()
		return mapSCKError(int(rc), C.GoString(&msg[0]))
	}
	d.running = true
	d.maxLong = cfg.MaxLongEdge
	d.noteEncoder()
	return nil
}

// Stop ends the capture, flushes the encoder and closes Frames and Errors.
func (d *DisplaySource) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return ErrNotRunning
	}
	d.running = false
	nativeMu.Lock()
	C.pc_stream_stop() // returns only once no callback can run any more
	nativeMu.Unlock()
	d.handle.Delete()
	d.closeChannels()
	return nil
}

// Frames is nil until a Start succeeds and is closed by Stop.
func (d *DisplaySource) Frames() <-chan AccessUnit {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	return d.frames
}

// Errors delivers mapped capture errors (-3801 as ErrPermissionDenied, -3815
// as ErrNoDisplay) and is closed by Stop. Errors are dropped when nobody reads.
func (d *DisplaySource) Errors() <-chan error {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	return d.errs
}

// AudioPackets delivers the system audio as Opus, one 20 ms packet each; it is
// nil until a Start succeeds and is closed by Stop. A packet is dropped when
// nobody keeps up (audio is real time).
func (d *DisplaySource) AudioPackets() <-chan AudioPacket {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	return d.audio
}

// AudioErrors delivers audio failures (ErrAudioUnavailable); the picture keeps
// streaming. Closed by Stop.
func (d *DisplaySource) AudioErrors() <-chan error {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	return d.audioErrs
}

// ForceKeyframe re-encodes the cached frame as an IDR (a still screen yields
// no frames, so the next real frame is no use to a joining viewer).
func (d *DisplaySource) ForceKeyframe() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if rc := C.pc_encoder_force_keyframe(); rc != 0 {
		d.emitErr(mapSCKError(int(rc), "forcing a keyframe failed"))
	}
}

// Rebuild re-creates the encoder at width x height and resizes the capture to
// match; the next access unit is a keyframe carrying SPS and PPS.
func (d *DisplaySource) Rebuild(width, height int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return ErrNotRunning
	}
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if rc := C.pc_encoder_rebuild(C.int(width), C.int(height)); rc != 0 {
		return mapSCKError(int(rc), "rebuilding the encoder failed")
	}
	d.noteEncoder()
	return nil
}

// SetBitrate changes the running encoder's average bitrate (and its burst bound)
// without creating a new session: no keyframe, no break in the picture.
func (d *DisplaySource) SetBitrate(bps int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return ErrNotRunning
	}
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if rc := C.pc_encoder_set_bitrate(C.int(bps)); rc != 0 {
		return mapSCKError(int(rc), "changing the bitrate failed")
	}
	return nil
}

// SetFPS changes the running capture's frame rate (the stream's minimum frame
// interval, then the encoder's expected rate). It touches the live stream, so a
// test binary never reaches it; the VM rows prove it on a real capture.
func (d *DisplaySource) SetFPS(fps int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return ErrNotRunning
	}
	if testing.Testing() {
		return ErrRefusedInTest
	}
	nativeMu.Lock()
	defer nativeMu.Unlock()
	if rc := C.pc_stream_set_fps(C.int(fps)); rc != 0 {
		return mapSCKError(int(rc), "changing the frame rate failed")
	}
	return nil
}

// MainDisplayID is the main display's CGDirectDisplayID (0 if it cannot be read).
func MainDisplayID() uint32 { return uint32(C.pc_main_display_id()) }

// HardwareEncoder reports the readback taken after the encoder session was
// created or last rebuilt. known is false when the encoder did not say. A
// software encoder is logged at that moment; the Manager shows it as a status
// reason, because a picture beats none on an unattended Mac.
func (d *DisplaySource) HardwareEncoder() (hardware, known bool) {
	switch d.hw.Load() {
	case 1:
		return true, true
	case 0:
		return false, true
	}
	return false, false
}

func (d *DisplaySource) noteEncoder() {
	hw := int32(C.pc_encoder_hardware())
	d.hw.Store(hw)
	if hw == 0 {
		log.Print("screencap: encoder using software")
	}
}

// emit queues a unit for Frames without ever blocking the encoder thread. When
// the channel is full a delta unit is dropped and a keyframe never is: a new
// keyframe evicts the oldest queued unit (order of the rest is kept).
// shortcut: a channel cannot drop "the oldest delta" while keeping order, so a
// full queue drops the newest delta instead; either way the picture is broken
// until the next keyframe (at most 4 s) and the viewer's PLI through the
// Manager covers it. Upgrade: a slice queue, or a keyframe request on drop.
func (d *DisplaySource) emit(au AccessUnit) {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if d.closed {
		return
	}
	for {
		select {
		case d.frames <- au:
			return
		default:
		}
		if !au.Keyframe {
			return
		}
		select {
		case <-d.frames: // emitMu makes this goroutine the only sender, so the slot stays free
		default:
		}
	}
}

func (d *DisplaySource) emitErr(err error) {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if d.closed {
		return
	}
	select {
	case d.errs <- err:
	default:
	}
}

// emitAudio queues a packet without ever blocking the audio queue; a full
// channel drops it.
func (d *DisplaySource) emitAudio(p AudioPacket) {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if d.closed {
		return
	}
	select {
	case d.audio <- p:
	default:
	}
}

func (d *DisplaySource) emitAudioErr(err error) {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if d.closed {
		return
	}
	select {
	case d.audioErrs <- err:
	default:
	}
}

func (d *DisplaySource) closeChannels() {
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.frames)
		close(d.errs)
		close(d.audio)
		close(d.audioErrs)
	}
}

// CaptureStill returns a JPEG (quality 0.8) of the main display, at most
// maxLongEdge px on the long edge, for the onboarding Test Capture. It gives
// up after 15 s. Like every real bridge it refuses under testing.Testing().
func CaptureStill(ctx context.Context, maxLongEdge int) ([]byte, error) {
	if testing.Testing() {
		return nil, ErrRefusedInTest
	}
	ctx, cancel := context.WithTimeout(ctx, stillCeiling)
	defer cancel()
	type result struct {
		jpeg []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var out *C.uint8_t
		var n C.int
		var msg [errBufLen]C.char
		rc := C.pc_still_capture(C.int(maxLongEdge), &out, &n, &msg[0], errBufLen)
		if rc != 0 {
			done <- result{err: mapSCKError(int(rc), C.GoString(&msg[0]))}
			return
		}
		defer C.free(unsafe.Pointer(out))
		done <- result{jpeg: C.GoBytes(unsafe.Pointer(out), n)}
	}()
	select {
	case r := <-done:
		return r.jpeg, r.err
	case <-ctx.Done():
		// The goroutine finishes on its own, frees its buffer and fills the buffered channel.
		return nil, ctx.Err()
	}
}

// surfaceListCeiling is how long ListSurfaces waits for ScreenCaptureKit.
const surfaceListCeiling = 15 * time.Second

// ListSurfaces lists every display and the normal windows that can be shared,
// in the order BuildSurfaces gives (main display first, windows by app and
// title). It makes one ScreenCaptureKit request (15 s ceiling) and, like every
// real bridge, refuses under testing.Testing(). Nothing it logs names a window.
func ListSurfaces(ctx context.Context) ([]SurfaceInfo, error) {
	if testing.Testing() {
		return nil, ErrRefusedInTest
	}
	ctx, cancel := context.WithTimeout(ctx, surfaceListCeiling)
	defer cancel()
	type result struct {
		list []SurfaceInfo
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var out *C.char
		var n C.int
		var msg [errBufLen]C.char
		rc := C.pc_list_surfaces(&out, &n, &msg[0], errBufLen)
		if rc != 0 {
			done <- result{err: mapSCKError(int(rc), C.GoString(&msg[0]))}
			return
		}
		defer C.free(unsafe.Pointer(out))
		list, err := decodeSurfaces(C.GoBytes(unsafe.Pointer(out), n))
		done <- result{list: list, err: err}
	}()
	select {
	case r := <-done:
		return r.list, r.err
	case <-ctx.Done():
		// The goroutine finishes on its own, frees its buffer and fills the buffered channel.
		return nil, ctx.Err()
	}
}
