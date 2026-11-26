package main

// probe.go: the R00 probe (vmprobe) and, from Task 3 of the plan, the
// permission-probe kit every launch path runs. Every step records its OS error
// domain and code instead of hiding it; a step that did not run is omitted from
// the JSON, never null, so has() tells whether it ran (see the plan's
// "Pinned JSON keys").

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// errBody is the pinned error object a failed step stores under its own key.
type errBody struct {
	Domain  string `json:"domain"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type stepErr struct {
	Error errBody `json:"error"`
}

func newStepErr(domain string, code int, msg string) stepErr {
	return stepErr{Error: errBody{Domain: domain, Code: code, Message: msg}}
}

// osErr keeps the errno when there is one.
func osErr(err error) stepErr {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return newStepErr("errno", int(errno), err.Error())
	}
	return newStepErr("spike", -1, err.Error())
}

// rawIsErr reports whether a JSON value returned by the C side is an
// {"error": ...} object (arrays and ordinary objects are not).
func rawIsErr(raw string) bool {
	var v struct {
		Error json.RawMessage `json:"error"`
	}
	return json.Unmarshal([]byte(raw), &v) == nil && len(v.Error) > 0
}

type osVersion struct {
	Version string `json:"version"`
	Build   string `json:"build"`
}

// macosVersion reads kern.osproductversion and kern.osversion.
func macosVersion() (osVersion, error) {
	v, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return osVersion{}, fmt.Errorf("sysctl kern.osproductversion: %w", err)
	}
	b, err := unix.Sysctl("kern.osversion")
	if err != nil {
		return osVersion{}, fmt.Errorf("sysctl kern.osversion: %w", err)
	}
	return osVersion{Version: v, Build: b}, nil
}

// macosStep is the pinned `macos` value: the object, or the error object.
func macosStep() any {
	v, err := macosVersion()
	if err != nil {
		return osErr(err)
	}
	return v
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// h264Info summarises an Annex-B file written by a drainer.
type h264Info struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	AccessUnits int    `json:"accessUnits"`
	Keyframes   int    `json:"keyframes"`
}

// drainer writes access units from auCh to an Annex-B file. With a limit it
// stops writing after that many units and closes reached.
type drainer struct {
	f       *os.File
	limit   int
	quit    chan struct{}
	done    chan struct{}
	reached chan struct{}
	info    h264Info
	err     error
}

func startDrain(path string, limit int) (*drainer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	d := &drainer{f: f, limit: limit, quit: make(chan struct{}), done: make(chan struct{}),
		reached: make(chan struct{}), info: h264Info{Path: path}}
	go d.loop()
	return d, nil
}

func (d *drainer) write(au accessUnit) {
	if d.limit > 0 && d.info.AccessUnits >= d.limit {
		return
	}
	if _, err := d.f.Write(au.data); err != nil && d.err == nil {
		d.err = err
	}
	d.info.Bytes += int64(len(au.data))
	d.info.AccessUnits++
	if au.key {
		d.info.Keyframes++
	}
	if d.limit > 0 && d.info.AccessUnits == d.limit {
		close(d.reached)
	}
}

func (d *drainer) loop() {
	defer close(d.done)
	for {
		select {
		case au := <-auCh:
			d.write(au)
		case <-d.quit:
			for {
				select {
				case au := <-auCh:
					d.write(au)
				default:
					return
				}
			}
		}
	}
}

// stop ends the drain after the source has stopped (so every flushed unit is
// already queued) and closes the file.
func (d *drainer) stop() (h264Info, error) {
	close(d.quit)
	<-d.done
	if err := d.f.Close(); err != nil && d.err == nil {
		d.err = err
	}
	return d.info, d.err
}

// vmRecord is the vmprobe JSON in its pinned order.
type vmRecord struct {
	PID          int      `json:"pid"`
	HVVMMPresent any      `json:"hv_vmm_present,omitempty"`
	MacOS        any      `json:"macos"`
	SRPreflight  *bool    `json:"sr_preflight,omitempty"`
	SRRequest    *bool    `json:"sr_request,omitempty"`
	Displays     any      `json:"displays,omitempty"`
	Capture      any      `json:"capture,omitempty"`
	PNG          any      `json:"png,omitempty"`
	Luma         *float64 `json:"luma,omitempty"`
	Encoders     any      `json:"encoders,omitempty"`
	Encoder      any      `json:"encoder,omitempty"`
	H264         any      `json:"h264,omitempty"`
}

const (
	vmprobeEncW    = 1280
	vmprobeEncH    = 720
	vmprobeFPS     = 30
	vmprobeBitrate = 4_000_000
)

// vmprobe is R00: does ScreenCaptureKit return frames here, is the IOSurface
// real, which H.264 encoders exist and is one hardware. It runs every step even
// after a failure, writes <out>/r00-vmprobe.json regardless, and exits 1 when
// any step failed. --encoders-only touches no SCK or TCC API (host-safe).
func vmprobe(ctx context.Context, args []string) error {
	fs := newFlagSet("vmprobe")
	var out string
	var seconds int
	var request, encodersOnly bool
	fs.StringVar(&out, "out", "", "output directory (required)")
	fs.IntVar(&seconds, "seconds", 5, "capture length in seconds")
	fs.BoolVar(&request, "request", false, "also call CGRequestScreenCaptureAccess (raises the prompt)")
	fs.BoolVar(&encodersOnly, "encoders-only", false, "list encoders and encode synthetic frames; no SCK, no TCC")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if out == "" || seconds <= 0 {
		return fmt.Errorf("%w: --out is required and --seconds must be positive", errUsage)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	rec := vmRecord{PID: os.Getpid(), MacOS: macosStep()}
	var fails []string
	if encodersOnly {
		fails = vmEncodersOnly(ctx, out, &rec)
	} else {
		fails = vmFull(ctx, out, seconds, request, &rec)
	}
	if _, bad := rec.MacOS.(stepErr); bad {
		fails = append(fails, "macos")
	}

	path := filepath.Join(out, "r00-vmprobe.json")
	if err := writeJSON(path, rec); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	b, _ := json.Marshal(rec)
	fmt.Println(string(b))
	if len(fails) > 0 {
		return fmt.Errorf("steps failed: %v (see %s)", fails, path)
	}
	return nil
}

// encoderStep fills the `encoders` and `encoder` keys.
func encoderStep(rec *vmRecord) (fails []string) {
	list := encoderList()
	rec.Encoders = json.RawMessage(list)
	if rawIsErr(list) {
		fails = append(fails, "encoders")
	}
	rec.Encoder = json.RawMessage(encoderInfo())
	return fails
}

func vmEncodersOnly(ctx context.Context, out string, rec *vmRecord) (fails []string) {
	if err := startEncoder(vmprobeEncW, vmprobeEncH, vmprobeFPS, vmprobeBitrate); err != nil {
		rec.Encoder = json.RawMessage(encoderInfo())
		rec.Encoders = json.RawMessage(encoderList())
		return append(fails, "encoder: "+err.Error())
	}
	defer stopSource()
	fails = append(fails, encoderStep(rec)...)

	d, err := startDrain(filepath.Join(out, "encoders-only.h264"), 60)
	if err != nil {
		return append(fails, "h264: "+err.Error())
	}
	if err := startSource("synthetic", vmprobeEncW, vmprobeEncH, vmprobeFPS); err != nil {
		_, _ = d.stop()
		return append(fails, "synthetic: "+err.Error())
	}
	select {
	case <-d.reached:
	case <-time.After(10 * time.Second):
		fails = append(fails, "h264: fewer than 60 access units in 10 s")
	case <-ctx.Done():
		fails = append(fails, "interrupted")
	}
	stopSource()
	info, err := d.stop()
	rec.H264 = info
	if err != nil {
		fails = append(fails, "h264: "+err.Error())
	}
	return fails
}

func vmFull(ctx context.Context, out string, seconds int, request bool, rec *vmRecord) (fails []string) {
	if v, err := unix.SysctlUint32("kern.hv_vmm_present"); err != nil {
		rec.HVVMMPresent = osErr(fmt.Errorf("sysctl kern.hv_vmm_present: %w", err))
		fails = append(fails, "hv_vmm_present")
	} else {
		rec.HVVMMPresent = int(v)
	}

	pre := srPreflight()
	rec.SRPreflight = &pre
	if request {
		req := srRequest()
		rec.SRRequest = &req
	}

	disp := displayList()
	rec.Displays = json.RawMessage(disp)
	if rawIsErr(disp) {
		fails = append(fails, "displays")
	}

	fails = append(fails, vmCapture(ctx, out, seconds, rec)...)
	return append(fails, encoderFallback(rec)...)
}

// vmCapture runs the --seconds stream, records its stats while it still runs,
// dumps the cached frame, then stops the stream and closes the .h264 file.
func vmCapture(ctx context.Context, out string, seconds int, rec *vmRecord) (fails []string) {
	d, err := startDrain(filepath.Join(out, "r00.h264"), 0)
	if err != nil {
		return append(fails, "h264: "+err.Error())
	}
	startErr := startSource("sck", 0, 0, vmprobeFPS)
	if startErr == nil {
		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
			fails = append(fails, "interrupted")
		}
	} else {
		fails = append(fails, "capture: "+startErr.Error())
	}
	stats := sckStats()
	rec.Capture = json.RawMessage(stats)
	if rawIsErr(stats) && startErr == nil {
		fails = append(fails, "capture: stream error")
	}
	if startErr == nil {
		pngPath := filepath.Join(out, "r00-frame.png")
		if luma, err := dumpPNG(pngPath); err != nil {
			rec.PNG = newStepErr("spike.png", -1, err.Error())
			fails = append(fails, "png")
		} else {
			rec.PNG = pngPath
			rec.Luma = &luma
		}
	}
	// Read the encoder before the session goes away; stopSource keeps the
	// readback, but this is the session the frames went through.
	rec.Encoder = json.RawMessage(encoderInfo())
	stopSource()
	info, err := d.stop()
	rec.H264 = info
	if err != nil {
		fails = append(fails, "h264: "+err.Error())
	}
	return fails
}

// encoderFallback lists the encoders and, when capture never created an
// encoder session, makes one at a fixed size so the hardware readback exists
// whatever the capture did.
func encoderFallback(rec *vmRecord) (fails []string) {
	if !hasEncoderID(rec.Encoder) {
		if err := startEncoder(vmprobeEncW, vmprobeEncH, vmprobeFPS, vmprobeBitrate); err != nil {
			fails = append(fails, "encoder: "+err.Error())
		}
		rec.Encoder = json.RawMessage(encoderInfo())
		stopSource()
	}
	return append(fails, encoderStep(rec)...)
}

// hasEncoderID reports whether an `encoder` value names a created session.
func hasEncoderID(v any) bool {
	raw, ok := v.(json.RawMessage)
	if !ok {
		return false
	}
	var e struct {
		EncoderID string `json:"encoderId"`
	}
	return json.Unmarshal(raw, &e) == nil && e.EncoderID != ""
}

// ---------------------------------------------------------------------------
// Permission-probe kit: one Probe function for every launch path.
// ---------------------------------------------------------------------------

// ProbeRecord is one probe's JSON in its pinned order. A step that did not run
// is omitted; a step that ran and failed holds its error object.
type ProbeRecord struct {
	Label              string   `json:"label"`
	Time               string   `json:"time"`
	PID                int      `json:"pid"`
	PPID               int      `json:"ppid"`
	PGID               int      `json:"pgid"`
	SID                int      `json:"sid"`
	Executable         string   `json:"executable"`
	MacOS              any      `json:"macos"`
	ManagerName        any      `json:"managername"`
	Session            any      `json:"session"`
	SRPreflight        any      `json:"sr_preflight,omitempty"`
	SRRequest          any      `json:"sr_request,omitempty"`
	Displays           any      `json:"displays,omitempty"`
	Capture            any      `json:"capture,omitempty"`
	PNG                any      `json:"png,omitempty"`
	Luma               *float64 `json:"luma,omitempty"`
	AXTrusted          any      `json:"ax_trusted,omitempty"`
	AXPrompt           any      `json:"ax_prompt,omitempty"`
	PostEventPreflight any      `json:"post_event_preflight,omitempty"`
	MouseNudge         any      `json:"mouse_nudge,omitempty"`
	Typed              any      `json:"typed,omitempty"`
	Delta              any      `json:"delta,omitempty"`
	StreamRunning      *bool    `json:"streamRunning,omitempty"`
	LatestError        any      `json:"latestError,omitempty"`

	fails []string
}

// Failed lists the steps that recorded an error.
func (r *ProbeRecord) Failed() []string { return r.fails }

func (r *ProbeRecord) fail(step string) { r.fails = append(r.fails, step) }

// statusCounts mirrors the `status` object of the pinned `capture` keys.
type statusCounts struct {
	Complete  int64 `json:"complete"`
	Idle      int64 `json:"idle"`
	Blank     int64 `json:"blank"`
	Suspended int64 `json:"suspended"`
	Started   int64 `json:"started"`
	Stopped   int64 `json:"stopped"`
	Unknown   int64 `json:"unknown"`
}

func (a statusCounts) sub(b statusCounts) statusCounts {
	return statusCounts{a.Complete - b.Complete, a.Idle - b.Idle, a.Blank - b.Blank, a.Suspended - b.Suspended,
		a.Started - b.Started, a.Stopped - b.Stopped, a.Unknown - b.Unknown}
}

// streamTracker keeps one continuous SCStream across the records of a loop.
type streamTracker struct {
	attempted bool
	prev      statusCounts
}

type probeOpts struct {
	label          string
	captureSeconds int
	request        bool
	axMove         bool
	noTCC          bool
	pngPath        string
	stream         *streamTracker // non-nil: share one stream across records
	typed          any            // keycodes this process posted since the last record
}

// tccAnswer turns a 1, 0 or -1 from a TCC call into the pinned boolean, or an
// error object when the call itself failed.
func tccAnswer(v int, name string) any {
	if v < 0 {
		return newStepErr("spike.tcc", v, name+" could not be called")
	}
	return v == 1
}

func managerName(ctx context.Context) any {
	out, err := exec.CommandContext(ctx, "launchctl", "managername").Output()
	if err != nil {
		return osErr(fmt.Errorf("launchctl managername: %w", err))
	}
	return strings.TrimSpace(string(out))
}

// Probe records where this process runs (ids, session, launchd manager) and,
// unless noTCC, what the permission and capture APIs say. It is the one probe
// function behind probe, probe-loop and agent, so every launch path produces
// comparable records.
func Probe(ctx context.Context, o probeOpts) ProbeRecord {
	rec := ProbeRecord{Label: o.label, Time: time.Now().UTC().Format(time.RFC3339Nano),
		PID: os.Getpid(), PPID: os.Getppid(), MacOS: macosStep(), ManagerName: managerName(ctx), Typed: o.typed}
	if pgid, err := unix.Getpgid(0); err != nil {
		rec.PGID = -1
		rec.fail("pgid: " + err.Error())
	} else {
		rec.PGID = pgid
	}
	if sid, err := unix.Getsid(0); err != nil {
		rec.SID = -1
		rec.fail("sid: " + err.Error())
	} else {
		rec.SID = sid
	}
	if exe, err := os.Executable(); err != nil {
		rec.fail("executable: " + err.Error())
	} else {
		rec.Executable = exe
	}
	if _, bad := rec.MacOS.(stepErr); bad {
		rec.fail("macos")
	}
	if _, bad := rec.ManagerName.(stepErr); bad {
		rec.fail("managername")
	}
	if _, bad := rec.Typed.(stepErr); bad {
		rec.fail("typed")
	}
	sess := sessionDict()
	rec.Session = json.RawMessage(sess)
	if rawIsErr(sess) {
		rec.fail("session")
	}
	if o.noTCC {
		return rec
	}

	rec.SRPreflight = srPreflight()
	if o.request {
		rec.SRRequest = srRequest()
	}
	disp := displayList()
	rec.Displays = json.RawMessage(disp)
	if rawIsErr(disp) {
		rec.fail("displays")
	}
	switch {
	case o.stream != nil:
		probeStream(ctx, o, &rec)
	case o.captureSeconds > 0:
		probeCapture(ctx, o, &rec)
	}

	rec.AXTrusted = tccAnswer(axTrusted(false), "ax_trusted")
	if o.request {
		rec.AXPrompt = tccAnswer(axTrusted(true), "ax_prompt")
		// The request's own answer is not a pinned key: the preflight that
		// follows reports the state it left behind.
		postEventRequest()
	}
	rec.PostEventPreflight = tccAnswer(postEventPreflight(), "post_event_preflight")
	if o.axMove {
		rec.MouseNudge = tccAnswer(mouseNudge(), "mouse_nudge")
	}
	for _, v := range []any{rec.AXTrusted, rec.PostEventPreflight, rec.MouseNudge} {
		if _, bad := v.(stepErr); bad {
			rec.fail("tcc call")
		}
	}
	return rec
}

// probeCapture runs the record's own capture, then stops it.
func probeCapture(ctx context.Context, o probeOpts, rec *ProbeRecord) {
	startErr := startSource("sck", 0, 0, vmprobeFPS)
	if startErr != nil {
		rec.fail("capture: " + startErr.Error())
	} else {
		select {
		case <-time.After(time.Duration(o.captureSeconds) * time.Second):
		case <-ctx.Done():
			rec.fail("interrupted")
		}
	}
	stats := sckStats()
	rec.Capture = json.RawMessage(stats)
	if startErr == nil && rawIsErr(stats) {
		rec.fail("capture: stream error")
	}
	if startErr == nil {
		dumpRecordPNG(o.pngPath, rec)
	}
	stopSource()
}

// probeStream reads the shared stream's stats and the status-count deltas since
// the previous record. The stream starts once, on the first record (after any
// request); a failed start is recorded, not retried, so a loop observes what
// the system does instead of prompting again every interval.
func probeStream(ctx context.Context, o probeOpts, rec *ProbeRecord) {
	st := o.stream
	if !st.attempted {
		st.attempted = true
		if err := startSource("sck", 0, 0, vmprobeFPS); err != nil {
			rec.fail("capture: " + err.Error())
		} else if o.captureSeconds > 0 {
			select {
			case <-time.After(time.Duration(o.captureSeconds) * time.Second):
			case <-ctx.Done():
			}
		}
	}
	stats := sckStats()
	rec.Capture = json.RawMessage(stats)
	var parsed struct {
		Status  statusCounts    `json:"status"`
		Running int             `json:"running"` // sckStats emits 0 or 1, not a JSON boolean
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(stats), &parsed); err != nil {
		rec.fail("capture: unreadable stats: " + err.Error())
		return
	}
	rec.Delta = parsed.Status.sub(st.prev)
	st.prev = parsed.Status
	running := parsed.Running != 0
	rec.StreamRunning = &running
	if len(parsed.Error) > 0 {
		rec.LatestError = parsed.Error
		rec.fail("capture: stream error")
	}
	if running {
		dumpRecordPNG(o.pngPath, rec)
	}
}

func dumpRecordPNG(path string, rec *ProbeRecord) {
	if path == "" {
		return
	}
	luma, err := dumpPNG(path)
	if err != nil {
		rec.PNG = newStepErr("spike.png", -1, err.Error())
		rec.fail("png")
		return
	}
	rec.PNG = path
	rec.Luma = &luma
}

// preflightRecord is the `probe --preflight-only` JSON: only the three calls
// that never prompt, so 17-11 and 17-12 can prove a grant is gone.
type preflightRecord struct {
	Label              string `json:"label"`
	Time               string `json:"time"`
	PID                int    `json:"pid"`
	SRPreflight        any    `json:"sr_preflight"`
	AXTrusted          any    `json:"ax_trusted"`
	PostEventPreflight any    `json:"post_event_preflight"`
}

func probeCmd(ctx context.Context, args []string) error {
	fs := newFlagSet("probe")
	var out, label, pngPath string
	var o probeOpts
	var preflightOnly bool
	fs.StringVar(&out, "out", "", "also write the record to this file")
	fs.StringVar(&label, "label", "probe", "label stored in the record")
	fs.StringVar(&pngPath, "png", "", "save the captured frame here (with a capture)")
	fs.IntVar(&o.captureSeconds, "capture-seconds", 5, "capture length; 0 skips capture")
	fs.BoolVar(&o.request, "request", false, "also call the request APIs (raises prompts)")
	fs.BoolVar(&o.axMove, "ax-move", false, "nudge the pointer one point (needs Accessibility or PostEvent)")
	fs.BoolVar(&o.noTCC, "no-tcc", false, "call no TCC API and capture nothing (host-safe)")
	fs.BoolVar(&preflightOnly, "preflight-only", false, "only the three non-prompting TCC preflight calls")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if o.captureSeconds < 0 || fs.NArg() > 0 {
		return fmt.Errorf("%w: --capture-seconds must not be negative and probe takes no arguments", errUsage)
	}
	if o.noTCC && (preflightOnly || o.request || o.axMove || pngPath != "") {
		return fmt.Errorf("%w: --no-tcc cannot be combined with --preflight-only, --request, --ax-move or --png", errUsage)
	}
	if preflightOnly && (o.request || o.axMove || pngPath != "") {
		return fmt.Errorf("%w: --preflight-only cannot be combined with --request, --ax-move or --png", errUsage)
	}
	o.label, o.pngPath = label, pngPath

	if preflightOnly {
		rec := preflightRecord{Label: label, Time: time.Now().UTC().Format(time.RFC3339Nano), PID: os.Getpid(),
			SRPreflight: srPreflight(), AXTrusted: tccAnswer(axTrusted(false), "ax_trusted"),
			PostEventPreflight: tccAnswer(postEventPreflight(), "post_event_preflight")}
		if err := emitJSON(out, rec, false); err != nil {
			return err
		}
		for _, v := range []any{rec.AXTrusted, rec.PostEventPreflight} {
			if _, bad := v.(stepErr); bad {
				return errors.New("a preflight call could not be made (see the record)")
			}
		}
		return nil
	}
	rec := Probe(ctx, o)
	if err := emitJSON(out, rec, false); err != nil {
		return err
	}
	if f := rec.Failed(); len(f) > 0 {
		return fmt.Errorf("steps failed: %v", f)
	}
	return nil
}

// emitJSON prints one compact JSON line to stdout and writes it to path
// (truncating, or appending when appendLine) when path is set.
func emitJSON(path string, v any, appendLine bool) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	if path == "" {
		return nil
	}
	if appendLine {
		return appendLineTo(path, b)
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func appendLineTo(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func parseKeycodes(s string) ([]int, error) {
	var codes []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 || n > 0xFFFF {
			return nil, fmt.Errorf("%q is not a keycode (0-65535)", part)
		}
		codes = append(codes, n)
	}
	return codes, nil
}

func probeLoop(ctx context.Context, args []string) error {
	fs := newFlagSet("probe-loop")
	var out, label, typeCodes string
	var interval, duration, typeAfter time.Duration
	var pngEvery int
	var stream bool
	var o probeOpts
	fs.StringVar(&out, "out", "", "JSONL file, one record appended per interval (required)")
	fs.StringVar(&label, "label", "loop", "label stored in each record")
	fs.DurationVar(&interval, "interval", 5*time.Second, "time from one record's start to the next")
	fs.DurationVar(&duration, "duration", 0, "stop after this long (0 = until signalled)")
	fs.BoolVar(&o.request, "request", false, "call the request APIs on the first record only")
	fs.BoolVar(&stream, "stream", false, "keep one SCStream for the whole loop; records carry status deltas")
	fs.IntVar(&pngEvery, "png-every", 0, "save a PNG every N records next to --out")
	fs.DurationVar(&typeAfter, "type-after", 0, "post --type-keycodes once, this long after the loop starts")
	fs.StringVar(&typeCodes, "type-keycodes", "", "comma-separated keycodes for --type-after")
	fs.IntVar(&o.captureSeconds, "capture-seconds", 5, "capture length per record (stream: wait after the first start)")
	fs.BoolVar(&o.axMove, "ax-move", false, "nudge the pointer one point each record")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if out == "" || interval <= 0 || duration < 0 || pngEvery < 0 || o.captureSeconds < 0 || fs.NArg() > 0 {
		return fmt.Errorf("%w: --out is required; --interval must be positive; --duration, --png-every and --capture-seconds must not be negative", errUsage)
	}
	if time.Duration(o.captureSeconds)*time.Second >= interval {
		return fmt.Errorf("%w: a record's capture (%ds) must be shorter than --interval (%s)", errUsage, o.captureSeconds, interval)
	}
	var codes []int
	if (typeAfter > 0) != (typeCodes != "") {
		return fmt.Errorf("%w: --type-after and --type-keycodes go together", errUsage)
	}
	if typeCodes != "" {
		var err error
		if codes, err = parseKeycodes(typeCodes); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	var pendingMu sync.Mutex
	var pending any
	if typeAfter > 0 {
		t := time.AfterFunc(typeAfter, func() {
			var v any = codes
			if rc := postKeycodes(codes); rc != 0 {
				v = newStepErr("spike.cgevent", rc, "posting keycodes failed")
			}
			pendingMu.Lock()
			pending = v
			pendingMu.Unlock()
		})
		defer t.Stop()
	}

	var tracker *streamTracker
	if stream {
		tracker = &streamTracker{}
		defer func() {
			if tracker.attempted {
				stopSource()
			}
		}()
	}
	failedRecords := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for n := 0; ; n++ {
		ro := o
		ro.label = label
		ro.request = o.request && n == 0
		ro.stream = tracker
		if pngEvery > 0 && n%pngEvery == 0 {
			ro.pngPath = filepath.Join(filepath.Dir(out), fmt.Sprintf("%s-%04d.png", label, n))
		}
		pendingMu.Lock()
		ro.typed, pending = pending, nil
		pendingMu.Unlock()
		rec := Probe(ctx, ro)
		if err := emitJSON(out, rec, true); err != nil {
			return err
		}
		if len(rec.Failed()) > 0 {
			failedRecords++
		}
		select {
		case <-ctx.Done():
			if failedRecords > 0 {
				return fmt.Errorf("%d of %d records recorded a failed step", failedRecords, n+1)
			}
			return nil
		case <-ticker.C:
		}
	}
}

// agent runs one subcommand named by a JSON file, so a launchd plist stays
// static and a later plan changes behaviour by writing a config in the guest.
// The config is data for spike subcommands only; there is no shell involved.
func agent(ctx context.Context, args []string) error {
	fs := newFlagSet("agent")
	var cfgPath string
	fs.StringVar(&cfgPath, "config", "", "JSON file {\"argv\": [...]} naming the subcommand to run (required)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if cfgPath == "" || fs.NArg() > 0 {
		return fmt.Errorf("%w: --config is required", errUsage)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var cfg struct {
		Argv []string `json:"argv"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return fmt.Errorf("%s: %w", cfgPath, err)
	}
	if len(cfg.Argv) == 0 || cfg.Argv[0] == "agent" {
		return fmt.Errorf("%w: %s needs an argv naming a subcommand other than agent", errUsage, cfgPath)
	}
	fmt.Fprintf(os.Stderr, "agent: pid=%d ppid=%d argv=%q\n", os.Getpid(), os.Getppid(), cfg.Argv)
	found, err := dispatch(ctx, cfg.Argv[0], cfg.Argv[1:])
	if !found {
		return fmt.Errorf("%w: unknown subcommand %q in %s", errUsage, cfg.Argv[0], cfgPath)
	}
	return err
}

const spawnLog = "/tmp/spike-spawn.log"

// spawn is launch path C: re-exec this executable detached in its own session,
// as the app starts the daemon today (internal/daemon/process_unix.go).
func spawn(ctx context.Context, args []string) error {
	fs := newFlagSet("spawn")
	var quitAfter bool
	fs.BoolVar(&quitAfter, "quit-after", false, "exit once the child has started, so no parent is left")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("%w: spawn [--quit-after] -- <argv>", errUsage)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(spawnLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("spawn log: %w", err)
	}
	cmd := exec.Command(exe, fs.Args()...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Start()
	_ = logFile.Close() // the child keeps its own descriptor
	if err != nil {
		return err
	}
	var waitErr error
	done := make(chan struct{})
	go func() { waitErr = cmd.Wait(); close(done) }()
	if err := emitJSON("", map[string]any{"verb": "spawn", "pid": cmd.Process.Pid, "log": spawnLog}, false); err != nil {
		return err
	}
	if quitAfter {
		return nil
	}
	select {
	case <-done:
		return waitErr
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGTERM)
		<-done
		return nil
	}
}

const helperPlist = "app.portty.spike.helper.plist"

// smappCmd builds register, unregister and status. status reads SMAppService
// state only and calls no TCC API.
func smappCmd(verb string) func(context.Context, []string) error {
	return func(_ context.Context, args []string) error {
		fs := newFlagSet(verb)
		var out string
		fs.StringVar(&out, "out", "", "also write the JSON to this file")
		if err := fs.Parse(args); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
		if fs.NArg() > 0 {
			return fmt.Errorf("%w: %s takes no arguments", errUsage, verb)
		}
		var res struct {
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(smapp(verb, helperPlist)), &res); err != nil {
			return fmt.Errorf("unreadable SMAppService result: %w", err)
		}
		rec := map[string]any{"verb": verb, "plist": helperPlist, "status": res.Status}
		if len(res.Error) > 0 {
			rec["error"] = res.Error
		}
		if err := emitJSON(out, rec, false); err != nil {
			return err
		}
		if len(res.Error) > 0 {
			return fmt.Errorf("%s failed: %s", verb, res.Error)
		}
		return nil
	}
}

// swap exchanges two paths atomically (RENAME_SWAP), the bundle swap of R10.
func swap(_ context.Context, args []string) error {
	fs := newFlagSet("swap")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("%w: swap <a> <b>", errUsage)
	}
	a, b := fs.Arg(0), fs.Arg(1)
	if err := unix.RenamexNp(a, b, unix.RENAME_SWAP); err != nil {
		_ = emitJSON("", map[string]any{"verb": "swap", "a": a, "b": b, "error": osErr(err).Error}, false)
		return fmt.Errorf("renamex_np RENAME_SWAP %s %s: %w", a, b, err)
	}
	return emitJSON("", map[string]any{"verb": "swap", "a": a, "b": b}, false)
}

// opusProbe is R21. --source sine encodes a generated tone and calls no TCC or
// SCK API (host-safe). --source sck captures system audio for --seconds and
// reports what ScreenCaptureKit delivered and what the Opus encoder made of it.
// The record is written before any failure is returned, and the exit is
// non-zero when encoding fails or no packet is produced.
func opusProbe(ctx context.Context, args []string) error {
	fs := newFlagSet("opusprobe")
	var source, out string
	var seconds int
	fs.StringVar(&source, "source", "sine", "sine or sck")
	fs.IntVar(&seconds, "seconds", 10, "length in seconds")
	fs.StringVar(&out, "out", "", "also write the record to this file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if (source != "sine" && source != "sck") || seconds <= 0 || fs.NArg() > 0 {
		return fmt.Errorf("%w: --source is sine or sck, --seconds must be positive", errUsage)
	}

	var rec map[string]any
	var packets float64
	if source == "sine" {
		if err := json.Unmarshal([]byte(opusSine(seconds)), &rec); err != nil {
			return fmt.Errorf("unreadable opus result: %w", err)
		}
		packets, _ = rec["packets"].(float64)
	} else {
		rec = map[string]any{}
		startErr := startSCKAudio()
		if startErr == nil {
			select {
			case <-time.After(time.Duration(seconds) * time.Second):
			case <-ctx.Done():
				rec["interrupted"] = true
			}
		} else {
			rec["startError"] = startErr.Error()
		}
		var audio, capture map[string]any
		if err := json.Unmarshal([]byte(audioStats()), &audio); err != nil {
			return fmt.Errorf("unreadable audio stats: %w", err)
		}
		if err := json.Unmarshal([]byte(sckStats()), &capture); err != nil {
			return fmt.Errorf("unreadable capture stats: %w", err)
		}
		stopSource()
		rec["audio"], rec["capture"] = audio, capture
		packets, _ = audio["packets"].(float64)
	}
	rec["source"], rec["seconds"] = source, seconds
	rec["time"] = time.Now().UTC().Format(time.RFC3339)
	rec["macos"] = macosStep()
	if err := emitJSON(out, rec, false); err != nil {
		return err
	}
	if packets == 0 {
		return errors.New("no Opus packet was produced (see the record)")
	}
	return nil
}
