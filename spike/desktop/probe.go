package main

// probe.go: the R00 probe (vmprobe) and, from Task 3 of the plan, the
// permission-probe kit every launch path runs. Every step records its OS error
// domain and code instead of hiding it; a step that did not run is omitted from
// the JSON, never null, so has() tells whether it ran (see the plan's
// "Pinned JSON keys").

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
