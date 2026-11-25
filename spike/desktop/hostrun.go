package main

// hostrun.go: `spike host-run`, the short run the person starts on the real Mac
// (and that VM office rehearses first). It is `serve --source sck` plus a
// full-screen test chart, a timeline of what the run observed, and a schedule
// of display sleep, wake and (only when allowed) typing. The pipeline is
// serve's own; host-run only adds hooks around it.
//
// Test content only: with --pattern the chart window covers the display, so
// every captured frame is the chart. Key input is off unless --allow-input and
// then only for keycodes listed in --allow-keycodes; keycodes are logged as
// numbers, never as characters.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
)

func init() {
	// AppKit must run on the process's first thread: pin the main goroutine to it.
	if len(os.Args) > 1 && os.Args[1] == "host-run" {
		runtime.LockOSThread()
	}
}

// The timeline's pinned event values (see the plan's "Pinned JSON keys").
const (
	evStart                = "start"
	evSRPreflight          = "sr_preflight"
	evViewerConnected      = "viewer_connected"
	evViewerDisconnected   = "viewer_disconnected"
	evMotion               = "motion"
	evPerf                 = "perf"
	evLock                 = "lock"
	evFrameStatus          = "frame_status"
	evDisplay              = "display"
	evSleep                = "sleep"
	evWake                 = "wake"
	evFirstFrameAfterWake  = "first_frame_after_wake"
	evKey                  = "key"
	timelineFile           = "host-run-timeline.jsonl"
	chartSettle            = 1500 * time.Millisecond // the chart must be on screen before the first captured frame
	staticTapLength        = 3 * time.Second
	pollSlow               = 250 * time.Millisecond
	pollFast               = 50 * time.Millisecond // while a wake waits for its first frame
	sessionPollEvery       = time.Second
	lockedKey              = "CGSSessionScreenIsLocked"
	controlWake            = "wake"
	controlMotionOn        = "motion:on"
	controlMotionOff       = "motion:off"
	controlKeyPrefix       = "key:"
	pmsetPath              = "/usr/bin/pmset"
	displaySleepCmdTimeout = 10 * time.Second
)

type hostRunOpts struct {
	serve      serveOpts
	flags      map[string]any // the start event's flags, without the token or the evidence path
	pattern    bool
	request    bool
	allowInput bool
	allowKeys  map[int]bool
	keepAwake  bool
	motionAt   time.Duration
	motionFor  time.Duration
	dumpAt     []time.Duration
	sleepAt    time.Duration
	wakeAfter  time.Duration
	typeAt     time.Duration
	typeKeys   []int
	statsEvery time.Duration
	duration   time.Duration
}

func parseOffsets(s string) ([]time.Duration, error) {
	if s == "" {
		return nil, nil
	}
	var out []time.Duration
	for _, part := range strings.Split(s, ",") {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("%q is not a positive duration", part)
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func parseKeySet(s string) (map[int]bool, error) {
	set := map[int]bool{}
	if s == "" {
		return set, nil
	}
	codes, err := parseKeycodes(s)
	if err != nil {
		return nil, err
	}
	for _, c := range codes {
		set[c] = true
	}
	return set, nil
}

// keyAllowed is the one gate for input: --allow-input and a listed keycode.
func keyAllowed(allowInput bool, allow map[int]bool, code int) bool {
	return allowInput && allow[code]
}

// parseKeyMsg reads a data-channel "key:<code>" message.
func parseKeyMsg(msg string) (int, bool) {
	n, ok := strings.CutPrefix(msg, controlKeyPrefix)
	if !ok {
		return 0, false
	}
	code, err := strconv.Atoi(n)
	if err != nil || code < 0 || code > 0xFFFF {
		return 0, false
	}
	return code, true
}

func parseHostRun(args []string) (hostRunOpts, error) {
	fs := newFlagSet("host-run")
	var o hostRunOpts
	var allowKeys, dumpAt, typeKeys string
	fs.StringVar(&o.serve.bind, "bind", "", "IPv4 address to bind (loopback, 100.64.0.0/10, or any with --allow-lan)")
	fs.IntVar(&o.serve.httpPort, "http-port", 47800, "HTTP port")
	fs.IntVar(&o.serve.udpPort, "udp-port", 47801, "UDP port for the ICE mux")
	fs.StringVar(&o.serve.token, "token", "", "32 hex characters (default: random, printed once)")
	fs.BoolVar(&o.serve.allowLAN, "allow-lan", false, "allow a bind address outside loopback and 100.64.0.0/10")
	fs.StringVar(&o.serve.evidenceDir, "evidence-dir", "", "directory for the timeline, raw frames and viewer reports (required)")
	fs.BoolVar(&o.pattern, "pattern", false, "cover the display with the test chart so only test content is captured")
	fs.BoolVar(&o.request, "request", false, "call the Screen Recording request once at start (and the key-post request when keys are allowed)")
	fs.BoolVar(&o.allowInput, "allow-input", false, "accept key input (data channel and --type-at); still limited to --allow-keycodes")
	fs.StringVar(&allowKeys, "allow-keycodes", "", "comma-separated keycodes that input may post (default none)")
	fs.DurationVar(&o.motionAt, "motion-at", 0, "turn the chart's motion region on this long after the start")
	fs.DurationVar(&o.motionFor, "motion-for", 0, "how long the motion stays on")
	fs.StringVar(&dumpAt, "dump-at", "", "comma-separated offsets; each writes raw-<n>.png, the first also static.h264 (3 s)")
	fs.DurationVar(&o.sleepAt, "sleep-at", 0, "run pmset displaysleepnow this long after the start")
	fs.DurationVar(&o.wakeAfter, "wake-after", 0, "declare user activity this long after the sleep")
	fs.DurationVar(&o.typeAt, "type-at", 0, "post --type-keycodes once, this long after the start")
	fs.StringVar(&typeKeys, "type-keycodes", "", "comma-separated keycodes for --type-at (each must be in --allow-keycodes)")
	fs.BoolVar(&o.keepAwake, "keep-awake", false, "hold a display-awake assertion while a viewer is connected")
	fs.DurationVar(&o.statsEvery, "stats-every", 5*time.Second, "perf sample interval")
	fs.DurationVar(&o.duration, "duration", 0, "stop this long after the capture starts (0 = until signalled)")
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("%w: %v", errUsage, err)
	}
	bad := func(format string, a ...any) (hostRunOpts, error) {
		return o, fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, a...))
	}
	if o.serve.bind == "" || o.serve.evidenceDir == "" || fs.NArg() > 0 {
		return bad("--bind and --evidence-dir are required")
	}
	if o.serve.token != "" && !validToken(o.serve.token) {
		return bad("--token must be 32 hex characters")
	}
	if o.statsEvery <= 0 || o.duration < 0 || o.motionAt < 0 || o.motionFor < 0 || o.sleepAt < 0 || o.wakeAfter < 0 || o.typeAt < 0 {
		return bad("--stats-every must be positive; durations must not be negative")
	}
	if (o.motionAt > 0) != (o.motionFor > 0) {
		return bad("--motion-at and --motion-for go together")
	}
	if o.wakeAfter > 0 && o.sleepAt == 0 {
		return bad("--wake-after needs --sleep-at")
	}
	var err error
	if o.allowKeys, err = parseKeySet(allowKeys); err != nil {
		return bad("--allow-keycodes: %v", err)
	}
	if o.dumpAt, err = parseOffsets(dumpAt); err != nil {
		return bad("--dump-at: %v", err)
	}
	if (o.typeAt > 0) != (typeKeys != "") {
		return bad("--type-at and --type-keycodes go together")
	}
	if typeKeys != "" {
		if o.typeKeys, err = parseKeycodes(typeKeys); err != nil {
			return bad("--type-keycodes: %v", err)
		}
		for _, c := range o.typeKeys {
			if !keyAllowed(o.allowInput, o.allowKeys, c) {
				return bad("typing refused: keycode %d needs --allow-input and a place in --allow-keycodes", c)
			}
		}
	}
	if o.duration > 0 {
		last := max(o.motionAt+o.motionFor, o.sleepAt+o.wakeAfter, o.typeAt)
		if len(o.dumpAt) > 0 {
			last = max(last, o.dumpAt[len(o.dumpAt)-1])
		}
		if last >= o.duration {
			return bad("a scheduled action at %s falls at or after --duration %s", last, o.duration)
		}
	}
	o.serve.source = "sck"
	o.serve.widthCap, o.serve.fps, o.serve.bitrate = sckWidthCap, 30, 8_000_000
	o.flags = map[string]any{}
	fs.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		case "token", "evidence-dir":
		case "bind": // a tailnet or LAN address stays out of committed evidence
			o.flags[f.Name] = string(redact([]byte(f.Value.String())))
		default:
			o.flags[f.Name] = f.Value.String()
		}
	})
	return o, nil
}

// timeline writes one JSON object per line: wall, ms (monotonic since the file
// was created) and event, plus the event's own fields.
type timeline struct {
	mu    sync.Mutex
	f     *os.File
	start time.Time
	err   error
}

func newTimeline(path string) (*timeline, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	return &timeline{f: f, start: time.Now()}, nil
}

func (t *timeline) emit(event string, kv map[string]any) {
	now := time.Now()
	rec := map[string]any{}
	for k, v := range kv {
		rec[k] = v
	}
	rec["wall"] = now.UTC().Format(time.RFC3339Nano)
	rec["ms"] = float64(now.Sub(t.start).Microseconds()) / 1000
	rec["event"] = event
	b, err := json.Marshal(rec)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err == nil {
		_, err = t.f.Write(append(b, '\n'))
	}
	if err != nil && t.err == nil {
		t.err = fmt.Errorf("timeline write (%s): %w", event, err)
		slog.Error("timeline write failed", "event", event, "err", err)
	}
}

func (t *timeline) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return errors.Join(t.err, t.f.Close())
}

// h264Tap copies access units to a file for a fixed window (the SSIM segment).
type h264Tap struct {
	mu sync.Mutex
	f  *os.File
}

func (t *h264Tap) begin(path string, d time.Duration) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.f = f
	t.mu.Unlock()
	time.AfterFunc(d, func() { _ = t.close() })
	return nil
}

func (t *h264Tap) write(au accessUnit) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f != nil {
		if _, err := t.f.Write(au.data); err != nil {
			slog.Error("static.h264 write", "err", err)
		}
	}
}

func (t *h264Tap) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return nil
	}
	err := t.f.Close()
	t.f = nil
	return err
}

// sckSnap is the part of spk_sck_stats the sampler reads.
type sckSnap struct {
	Status  statusCounts    `json:"status"`
	Running int             `json:"running"` // 0 or 1
	Error   json.RawMessage `json:"error"`
}

type latWindow struct {
	N    int64   `json:"n"`
	Mean float64 `json:"mean"`
	P95  float64 `json:"p95"`
	Max  float64 `json:"max"`
	Next int64   `json:"next"`
}

// frameClass names the frame-status run a poll interval belongs to.
func frameClass(d statusCounts, running bool) string {
	switch {
	case !running:
		return "stopped"
	case d.Blank > 0:
		return "blank"
	case d.Suspended > 0:
		return "suspended"
	case d.Complete > 0:
		return "complete"
	case d.Idle > 0:
		return "idle"
	}
	return "none"
}

type hostRun struct {
	o  hostRunOpts
	tl *timeline

	statsMu sync.Mutex // spk_sck_stats and friends return a shared buffer: one caller at a time
	latNext int64
	tb      [2]uint32

	wakeMu sync.Mutex
	wakeAt time.Time // set by a wake, cleared by the first Complete frame after it

	peersMu sync.Mutex
	peers   map[string]bool

	tap      h264Tap
	motionOn atomic.Bool
	asleep   atomic.Bool
	locked   atomic.Bool

	errMu sync.Mutex
	errs  []error
}

func (h *hostRun) fail(err error) {
	slog.Error("host-run", "err", err)
	h.errMu.Lock()
	h.errs = append(h.errs, err)
	h.errMu.Unlock()
}

func (h *hostRun) readStats() (sckSnap, error) {
	h.statsMu.Lock()
	raw := sckStats()
	h.statsMu.Unlock()
	var s sckSnap
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return s, fmt.Errorf("unreadable capture stats: %w", err)
	}
	return s, nil
}

func (h *hostRun) readLatency() (latWindow, error) {
	h.statsMu.Lock()
	raw := latSince(h.latNext)
	h.statsMu.Unlock()
	var w latWindow
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return w, fmt.Errorf("unreadable latency window: %w", err)
	}
	h.latNext = w.Next
	return w, nil
}

// cpuSeconds converts proc_pid_rusage's mach ticks with mach_timebase_info.
func (h *hostRun) cpuSeconds() (secs float64, rss uint64, err error) {
	user, sys, rss, err := rusageTicks()
	if err != nil {
		return 0, 0, err
	}
	return float64(user+sys) * float64(h.tb[0]) / float64(h.tb[1]) / 1e9, rss, nil
}

// sampler writes one perf line at once (with the encoder) and then every --stats-every.
func (h *hostRun) sampler(ctx context.Context) {
	prev, err := h.readStats()
	if err != nil {
		h.fail(err)
		return
	}
	prevCPU, _, err := h.cpuSeconds()
	if err != nil {
		h.fail(err)
		return
	}
	prevAt := time.Now()
	first := true
	tick := time.NewTicker(h.o.statsEvery)
	defer tick.Stop()
	for {
		extra := map[string]any{}
		if first {
			var enc any
			if json.Unmarshal([]byte(encoderInfo()), &enc) == nil {
				extra["encoder"] = enc
			}
			first = false
		} else {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
		now := time.Now()
		cur, err := h.readStats()
		if err != nil {
			h.fail(err)
			return
		}
		lat, err := h.readLatency()
		if err != nil {
			h.fail(err)
			return
		}
		cpu, rss, err := h.cpuSeconds()
		if err != nil {
			h.fail(err)
			return
		}
		window := now.Sub(prevAt).Seconds()
		d := cur.Status.sub(prev.Status)
		extra["window_s"] = window
		extra["fps"] = float64(d.Complete) / max(window, 0.001)
		extra["complete"], extra["idle"] = d.Complete, d.Idle
		extra["lat_n"] = lat.N
		if lat.N > 0 {
			extra["lat_mean_ms"], extra["lat_p95_ms"], extra["lat_max_ms"] = lat.Mean, lat.P95, lat.Max
		}
		extra["cpu_pct"] = (cpu - prevCPU) / max(window, 0.001) * 100
		extra["rss_mb"] = float64(rss) / (1 << 20)
		extra["running"] = cur.Running != 0
		extra["motion"], extra["asleep"], extra["locked"] = h.motionOn.Load(), h.asleep.Load(), h.locked.Load()
		h.tl.emit(evPerf, extra)
		prev, prevCPU, prevAt = cur, cpu, now
	}
}

// framePoller reports frame-status runs and the first Complete frame after a wake.
func (h *hostRun) framePoller(ctx context.Context) {
	prev, err := h.readStats()
	if err != nil {
		h.fail(err)
		return
	}
	cur, runStart := "", time.Now()
	for {
		wait := pollSlow
		h.wakeMu.Lock()
		armed := !h.wakeAt.IsZero()
		h.wakeMu.Unlock()
		if armed {
			wait = pollFast
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		s, err := h.readStats()
		if err != nil {
			h.fail(err)
			return
		}
		d := s.Status.sub(prev.Status)
		prev = s
		now := time.Now()
		if c := frameClass(d, s.Running != 0); c != cur {
			fields := map[string]any{"status": c}
			if cur != "" {
				fields["prev"], fields["prev_ms"] = cur, float64(now.Sub(runStart).Microseconds())/1000
			}
			if len(s.Error) > 0 {
				fields["error"] = s.Error
			}
			h.tl.emit(evFrameStatus, fields)
			cur, runStart = c, now
		}
		if d.Complete > 0 {
			h.wakeMu.Lock()
			at := h.wakeAt
			h.wakeAt = time.Time{}
			h.wakeMu.Unlock()
			if !at.IsZero() {
				h.tl.emit(evFirstFrameAfterWake, map[string]any{"delay_ms": float64(now.Sub(at).Microseconds()) / 1000})
			}
		}
	}
}

// sessionPoller reports lock-state and display-sleep changes, polled every second.
func (h *hostRun) sessionPoller(ctx context.Context) {
	var lockSeen, dispSeen bool
	tick := time.NewTicker(sessionPollEvery)
	defer tick.Stop()
	for {
		var dict map[string]any
		if err := json.Unmarshal([]byte(sessionDict()), &dict); err != nil {
			h.fail(fmt.Errorf("unreadable CGSession dictionary: %w", err))
			return
		}
		if e, bad := dict["error"]; bad {
			h.fail(fmt.Errorf("CGSession dictionary: %v", e))
			return
		}
		_, locked := dict[lockedKey]
		if !lockSeen || locked != h.locked.Load() {
			h.locked.Store(locked)
			h.tl.emit(evLock, map[string]any{"locked": locked, "initial": !lockSeen})
			lockSeen = true
		}
		asleep := displayAsleep()
		if !dispSeen || asleep != h.asleep.Load() {
			h.asleep.Store(asleep)
			h.tl.emit(evDisplay, map[string]any{"asleep": asleep, "initial": !dispSeen})
			dispSeen = true
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (h *hostRun) setMotion(on bool, source string) {
	h.motionOn.Store(on)
	chartMotion(on)
	h.tl.emit(evMotion, map[string]any{"on": on, "source": source})
}

func (h *hostRun) sleepDisplay() {
	ctx, cancel := context.WithTimeout(context.Background(), displaySleepCmdTimeout)
	defer cancel()
	fields := map[string]any{"cmd": "pmset displaysleepnow"}
	if out, err := exec.CommandContext(ctx, pmsetPath, "displaysleepnow").CombinedOutput(); err != nil {
		fields["error"] = fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))
		h.fail(fmt.Errorf("pmset displaysleepnow: %v", err))
	}
	h.tl.emit(evSleep, fields)
}

// wake declares user activity and starts timing the first Complete frame.
func (h *hostRun) wake(source string) {
	h.wakeMu.Lock()
	defer h.wakeMu.Unlock()
	at := time.Now()
	rc := userActivity()
	h.wakeAt = at
	h.tl.emit(evWake, map[string]any{"source": source, "ioreturn": rc})
	if rc != 0 {
		h.fail(fmt.Errorf("IOPMAssertionDeclareUserActivity: IOReturn %d", rc))
	}
}

// postKeys posts keycodes the caller already checked and logs them as numbers.
func (h *hostRun) postKeys(codes []int, source string) {
	rc := postKeycodes(codes)
	h.tl.emit(evKey, map[string]any{"codes": codes, "source": source, "rc": rc})
	if rc != 0 {
		h.fail(fmt.Errorf("posting keycodes failed (code %d)", rc))
	}
}

func (h *hostRun) dump(n int) {
	path := filepath.Join(h.o.serve.evidenceDir, fmt.Sprintf("raw-%d.png", n))
	luma, err := dumpPNG(path)
	if err != nil {
		h.fail(fmt.Errorf("frame dump %d: %w", n, err))
		return
	}
	if n == 1 {
		if err := h.tap.begin(filepath.Join(h.o.serve.evidenceDir, "static.h264"), staticTapLength); err != nil {
			h.fail(err)
			return
		}
		forceKeyframe() // an IDR of the same cached picture opens the segment
	}
	slog.Info("frame dumped", "file", filepath.Base(path), "meanLuma", luma)
}

// control handles the data-channel messages host-run adds to serve's ping.
func (h *hostRun) control(_ *webrtc.DataChannel, msg string) bool {
	switch msg {
	case controlWake:
		h.wake("datachannel")
	case controlMotionOn:
		h.setMotion(true, "datachannel")
	case controlMotionOff:
		h.setMotion(false, "datachannel")
	default:
		code, ok := parseKeyMsg(msg)
		if !ok {
			return false
		}
		// The refusal log carries the number only, never the message text.
		if !keyAllowed(h.o.allowInput, h.o.allowKeys, code) {
			slog.Warn("key refused", "keycode", code, "allowInput", h.o.allowInput)
			return true
		}
		h.postKeys([]int{code}, "datachannel")
	}
	return true
}

func (h *hostRun) peerState(peer, state string) {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()
	switch state {
	case "connected":
		if h.peers[peer] {
			return
		}
		h.peers[peer] = true
		h.tl.emit(evViewerConnected, map[string]any{"peer": peer, "viewers": len(h.peers)})
		if h.o.keepAwake && len(h.peers) == 1 {
			if rc := displayAwake(true); rc != 0 {
				h.fail(fmt.Errorf("display-awake assertion: IOReturn %d", rc))
			}
		}
	case "disconnected", "failed", "closed":
		if !h.peers[peer] {
			return
		}
		delete(h.peers, peer)
		h.tl.emit(evViewerDisconnected, map[string]any{"peer": peer, "state": state, "viewers": len(h.peers)})
		if h.o.keepAwake && len(h.peers) == 0 {
			displayAwake(false)
		}
	}
}

// schedule arms the timers for the planned actions; the returned func stops them.
func (h *hostRun) schedule() (stop func()) {
	var timers []*time.Timer
	at := func(d time.Duration, f func()) {
		if d > 0 {
			timers = append(timers, time.AfterFunc(d, f))
		}
	}
	o := h.o
	at(o.motionAt, func() { h.setMotion(true, "schedule") })
	at(o.motionAt+o.motionFor, func() { h.setMotion(false, "schedule") })
	for i, d := range o.dumpAt {
		at(d, func() { h.dump(i + 1) })
	}
	at(o.sleepAt, h.sleepDisplay)
	if o.wakeAfter > 0 {
		at(o.sleepAt+o.wakeAfter, func() { h.wake("schedule") })
	}
	at(o.typeAt, func() { h.postKeys(o.typeKeys, "schedule") })
	return func() {
		for _, t := range timers {
			t.Stop()
		}
	}
}

// hostRunCmd parses the flags, runs the work on a goroutine and gives the main
// thread to NSApplication until the work ends.
func hostRunCmd(ctx context.Context, args []string) error {
	o, err := parseHostRun(args)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- runHostRun(ctx, o)
		appStop()
	}()
	appRun()
	return <-done
}

func runHostRun(ctx context.Context, o hostRunOpts) (retErr error) {
	if err := os.MkdirAll(o.serve.evidenceDir, 0o755); err != nil {
		return fmt.Errorf("evidence dir: %w", err)
	}
	tl, err := newTimeline(filepath.Join(o.serve.evidenceDir, timelineFile))
	if err != nil {
		return fmt.Errorf("timeline: %w", err)
	}
	h := &hostRun{o: o, tl: tl, peers: map[string]bool{}}
	defer func() { retErr = errors.Join(retErr, tl.close()) }()
	h.tb[0], h.tb[1] = machTimebase()
	if h.tb[1] == 0 {
		return errors.New("mach_timebase_info returned a zero denominator")
	}

	start := map[string]any{"flags": o.flags, "macos": macosStep(), "pid": os.Getpid()}
	tl.emit(evStart, start)

	pre := srPreflight()
	pf := map[string]any{"granted": pre, "requested": o.request}
	if o.request {
		pf["granted_before"] = pre
		srRequest()
		if o.allowInput {
			postEventRequest()
		}
		pf["granted"] = srPreflight()
	}
	tl.emit(evSRPreflight, pf)
	if pf["granted"] != true {
		return errors.New("this app has no Screen Recording grant: grant it in System Settings, then start again")
	}

	if o.pattern {
		chartShow(true)
		defer chartShow(false)
		select {
		case <-time.After(chartSettle):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	defer displayAwake(false)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	so := o.serve
	so.control, so.peerState, so.tap = h.control, h.peerState, h.tap.write
	ready := make(chan listenInfo, 1)
	served := make(chan error, 1)
	go func() { served <- runServe(runCtx, so, ready) }()
	select {
	case <-ready:
	case err := <-served:
		return err
	}

	// The schedule's zero is the moment the capture is running.
	var wg sync.WaitGroup
	for _, f := range []func(context.Context){h.sampler, h.framePoller, h.sessionPoller} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f(runCtx)
		}()
	}
	stopTimers := h.schedule()
	if o.duration > 0 {
		time.AfterFunc(o.duration, cancel)
	}
	runErr := <-served
	cancel()
	stopTimers()
	wg.Wait()
	_ = h.tap.close()
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return errors.Join(append(h.errs, runErr)...)
}
