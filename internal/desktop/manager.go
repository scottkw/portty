// Package desktop serves the always-available desktop link: one shared H.264
// track fed by a screencap.Source, one pion PeerConnection per viewer, and the
// versioned signalling protocol the /desktop/ws route carries.
//
// Signalling protocol v1 (JSON text frames, each with a string "type", 32 KiB
// read limit):
//
//	viewer to server: hello {v, caps} first, then offer {sdp} (complete,
//	non-trickle), and ping {n}, surfaces and select {kind, id} at any time.
//	server to viewer: hello {v, viewer, caps, rights}, answer {sdp}, state
//	{state, viewers, locked}, size {w, h}, surface {e, kind, id, name, w, h,
//	reason}, surfaces {displays, windows}, select_failed {code}, pong {n},
//	error {code}.
//
// Unknown caps keys and unknown message types after the offer are ignored, so
// Phase 20 adds an "input" capability (and the rights map) without a version
// bump; testdata/signal-v1.jsonl and input-v1.jsonl freeze the v1 additions.
// Close codes:
// 4401 token rejected, 4410 desktop off, 1012 server restarting.
package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/ice/v4"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/wakewatch"
)

// ProtocolVersion is the signalling protocol version in hello.
const ProtocolVersion = 1

// Close codes beyond the standard ones (1012 is websocket.StatusServiceRestart).
const (
	CloseTokenRejected websocket.StatusCode = 4401
	CloseDesktopOff    websocket.StatusCode = 4410
	CloseFull          websocket.StatusCode = 4429 // the viewer cap is reached
)

// MaxViewers is the most viewers one Manager serves at once (T-19-11). A
// further viewer is refused, so it sees the viewer's normal retrying state.
const MaxViewers = 8

// State is what the desktop link is doing, pushed to viewers and the host GUI.
// state.go derives it from the flags the state loop and the watchdog keep.
type State string

// The state enum. capture_paused is the crash-loop breaker's state.
const (
	StateOff              State = "off"
	StateNeedsPermission  State = "needs_permission"
	StateAwaitingApproval State = "awaiting_approval"
	StateDisplayAsleep    State = "display_asleep"
	StateLocked           State = "locked"
	StateIdle             State = "idle"
	StateStreaming        State = "streaming"
	StateCapturePaused    State = "capture_paused"
)

// keyframeCoalesce is the shortest gap between forced keyframes. Joins, PLI
// and FIR all go through requestKeyframe, so a storm of requests costs the
// encoder at most one keyframe per window plus one deferred to the window end.
const keyframeCoalesce = time.Second

// outboundMTU is the RTP payload budget per packet (pion's own default).
const outboundMTU = 1200

// Handshake bounds (T-19-03): the hello must arrive within helloTimeout, the
// offer within offerTimeout after it, and no frame may exceed readLimit.
const (
	helloTimeout = 10 * time.Second
	offerTimeout = 30 * time.Second
	readLimit    = 32768
	writeTimeout = 5 * time.Second
)

// Error codes of the error message.
const (
	errCodeVersion     = "version"
	errCodeBadMessage  = "bad_message"
	errCodeOfferFailed = "offer_failed"
)

// Message types.
const (
	typeHello  = "hello"
	typeOffer  = "offer"
	typeAnswer = "answer"
	typeState  = "state"
	typeSize   = "size"
	typePing   = "ping"
	typePong   = "pong"
	typeError  = "error"
)

type msgHello struct {
	Type   string         `json:"type"`
	V      int            `json:"v"`
	Viewer string         `json:"viewer,omitempty"`
	Caps   map[string]int `json:"caps,omitempty"`
	// Rights is what this viewer may do (control, clipboard, surface).
	Rights map[string]bool `json:"rights,omitempty"`
}

type msgAnswer struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type msgState struct {
	Type    string `json:"type"`
	State   State  `json:"state"`
	Viewers int    `json:"viewers"`
	Locked  bool   `json:"locked"`
	// Audio is "failed" while audio capture or encoding has failed; omitted otherwise.
	Audio string `json:"audio,omitempty"`
}

type msgSize struct {
	Type string `json:"type"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

type msgPong struct {
	Type string `json:"type"`
	N    int    `json:"n"`
}

type msgError struct {
	Type string `json:"type"`
	Code string `json:"code"`
}

// inbound is every viewer-to-server message (hello, offer, ping, select) decoded
// loosely: the type decides which fields matter, and unknown fields are ignored.
type inbound struct {
	Type string `json:"type"`
	V    int    `json:"v"`
	SDP  string `json:"sdp"`
	N    int    `json:"n"`
	Kind string `json:"kind"` // select: "display" or "window"
	ID   uint32 `json:"id"`   // select: the display or window ID, 0 is the main display
	// The clipboard fields: clip_get and clip_put carry the transfer ID; a
	// clip_put chunk adds its place (Seq of Total), the whole paste's byte
	// length and its text.
	Xid   string `json:"xid"`
	Seq   int    `json:"seq"`
	Total int    `json:"total"`
	Len   int    `json:"len"`
	Text  string `json:"text"`
	// Caps is the viewer's hello capabilities; only the ones this server has are
	// ever acted on or echoed.
	Caps map[string]int `json:"caps"`
}

// Deps is what the Manager needs from its host.
type Deps struct {
	Bind         netip.Addr              // the one address media is served on
	MediaPort    int                     // the one UDP port, fixed because it travels in the SDP
	NewSource    func() screencap.Source // a fresh source per capture run
	SourceConfig screencap.Config
	ConfigDir    string
	Now          func() time.Time // also drives the keyframe coalescer
	// OnDataChannel receives a data channel a viewer created before its
	// offer. nil means CloseDataChannel: Phase 19 is view-only.
	OnDataChannel DataChannelHook
	// Input posts remote input; nil means screencap.NewInputBridge(). Every real
	// posting call refuses under tests, so tests inject a fake.
	Input InputBridge
	// DisplayBounds reads a display's global bounds in points (id 0 is the main
	// display); nil means screencap.DisplayBounds.
	DisplayBounds func(id uint32) (x, y, w, h float64, err error)
	// PointOnDisplay reports whether a global point lies on any active display;
	// nil means screencap.PointOnDisplay (which refuses under tests).
	PointOnDisplay func(x, y float64) (bool, error)
	// MainDisplayID is the main display's real ID, which a surface message
	// carries so a viewer can match it to the list; nil means screencap.MainDisplayID.
	MainDisplayID func() uint32
	// ListSurfaces lists every display and window for the picker; nil means
	// screencap.ListSurfaces, which refuses under tests, so tests inject a fake.
	ListSurfaces func(ctx context.Context) ([]screencap.SurfaceInfo, error)
	// Window reads and raises the shared window for the window gate; nil means
	// screencap.NewWindowBridge, which refuses under tests, so tests inject a fake.
	Window WindowBridge
	// Pasteboard reads and writes the Mac's clipboard for a viewer's explicit
	// Copy and Paste; nil means screencap.NewPasteboard, which refuses under
	// tests, so tests inject a fake.
	Pasteboard PasteboardBridge
	// Notify posts the approval-lapse notification when the daemon is its sender; nil means log-only.
	Notify func(id, title, body string)
	// OnViewerJoin and OnViewerLeave fire once per viewer, own-account or guest,
	// after the join succeeds and after the viewer is removed. Both run outside
	// every Manager lock; nil means nothing.
	OnViewerJoin, OnViewerLeave func(Viewer)
	// PeerIdentity resolves a media peer address to the user ID and stable node
	// ID that tailscaled reports for it. A guest's selected ICE pair is checked
	// against the admitted identity through it (media_binding.go); nil closes
	// every guest, an own-account viewer is never checked.
	PeerIdentity func(ctx context.Context, addr string) (userID int64, stableID string, err error)

	// Physical reports what the listen-only input tap knows about the person at
	// the Mac; nil means TapPhysicalSource. StartTap, StopTap and TapState drive
	// that tap (nil means the screencap functions; StartTap refuses under tests)
	// and SecureInputOn reads Secure Event Input. Tests replace every one.
	Physical      PhysicalSource
	StartTap      func() error
	StopTap       func()
	TapState      func() (running, enabled bool)
	SecureInputOn func() bool

	// The native reads and actions below default (when nil) to the screencap
	// bridges and the real prober, so a host that does not care wires nothing;
	// tests replace every one with a fake.
	Lock            func() (bool, error)                                     // is the session screen locked
	DisplayAsleep   func() (bool, error)                                     // is the main display asleep
	OnAC            func() (bool, error)                                     // is the Mac on AC power
	HoldDisplay     func(name string) (release func() error, err error)      // PreventUserIdleDisplaySleep
	HoldSystem      func(name string) (release func() error, err error)      // PreventUserIdleSystemSleep
	DeclareActivity func(name string) error                                  // remote user activity (lights a dark display)
	Nudge           func() (bool, error)                                     // one-point pointer move and return
	Probe           func(ctx context.Context) (screencap.Permissions, error) // fresh-child permission read
	Wake            <-chan wakewatch.Reason                                  // nil means wakewatch.Watch on the Manager's context
	ApprovalStore   func(ctx context.Context) (time.Time, error)             // replayd's LastAlerted date, if readable
}

// Package-level defaults for the Deps seams.
var (
	defaultLock          = screencap.SessionLocked
	defaultDisplayAsleep = screencap.DisplayAsleep
	defaultOnAC          = screencap.OnACPower
	defaultNudge         = screencap.NudgePointer
	defaultDeclare       = screencap.DeclareRemoteUserActivity
	defaultHoldDisplay   = holdFunc(screencap.HoldDisplayAwake)
	defaultHoldSystem    = holdFunc(screencap.HoldSystemAwake)
	defaultApprovalStore = readApprovalStore
)

func holdFunc(take func(string) (screencap.PowerAssertion, error)) func(string) (func() error, error) {
	return func(name string) (func() error, error) {
		a, err := take(name)
		if err != nil {
			return nil, err
		}
		return a.Release, nil
	}
}

// Status is the desktop link's state as the host sees it. Locked is an
// attribute that coexists with Viewers. LapseSeq counts approval lapses and
// LapseClaimedSeq is the last one whose notification has a sender.
type Status struct {
	State            State
	Viewers          int
	Locked           bool
	ApproachingLapse bool
	LapseSeq         int
	LapseClaimedSeq  int
	Reason           string
}

// viewer is one connected viewer: its public record, signalling socket,
// PeerConnection and outbound track. ready and armed are guarded by Manager.vmu.
type viewer struct {
	Viewer
	ws    *websocket.Conn
	pc    *webrtc.PeerConnection
	track *webrtc.TrackLocalStaticRTP
	// audio is the viewer's Opus track, nil when it did not offer audio (audio.go).
	audio    *webrtc.TrackLocalStaticRTP
	audioCap int  // the viewer's hello caps audio, 1 when it can play audio
	ready    bool // the media path is up (PeerConnection Connected)
	armed    bool // a keyframe has been written to this viewer, deltas may follow

	inputCap int                 // the viewer's hello caps input, 1 when it can send input
	in       *viewerInput        // its input state, guarded by Manager.in.mu
	dc       *webrtc.DataChannel // its accepted input channel, guarded by Manager.vmu

	gs       *GuestShare // the share a guest was admitted to; nil for an own-account viewer
	mediaCut atomic.Bool // a media identity check already cut this guest
	cut      atomic.Bool // CloseViewer or CloseGuests already cut this viewer
	// leaseReady is set once the answer is sent: lease messages start then.
	leaseReady atomic.Bool

	// rights is the live Rights, changed by SetRights and read by the press gate
	// and the clipboard handlers; Viewer.Rights is only the starting record.
	rights atomic.Uint32
}

// Manager owns the media socket, the shared track, the capture source and the
// connected viewers.
type Manager struct {
	d Deps

	// mu guards the lifecycle (socket, API, capture source) and is held across
	// a source stop, never across a source start (a native start can block for
	// about 35 s). vmu guards the viewer set and the last picture size; the
	// fan-out goroutine takes only vmu, so a stop that waits for it under mu
	// cannot deadlock. Lock order: mu, then vmu.
	mu       sync.Mutex
	ctx      context.Context
	conn     *net.UDPConn
	mux      ice.UDPMux
	api      *webrtc.API
	closed   atomic.Bool // atomic so Close never waits on mu to set it
	starting bool        // a source start is in flight outside mu
	// wrapConn, set only by tests, wraps the media socket before the mux.
	wrapConn func(net.PacketConn) net.PacketConn
	src      screencap.Source
	srcStop  context.CancelFunc
	fanDone  chan struct{}
	// audioDone is closed when audioOut has ended; nil when the source has no audio.
	audioDone chan struct{}
	// audioFail is set while audio capture or encoding has failed (audio.go).
	audioFail atomic.Bool

	// q is the adaptive quality controller and bwe the slot that hands each new
	// PeerConnection's estimator to answer (quality.go, peer.go).
	q   *qualityController
	bwe bweSlot

	// in is the input path's state (input.go).
	in inputState

	// surf is what the share shows (surface.go).
	surf surfaceState

	// win is the window gate's state (window.go).
	win windowState

	// clip is the clipboard's state (clipboard.go).
	clip clipState

	// arb decides who may type right now (arbiter.go); lease is what viewers are
	// told about it and the input tap's life cycle (lease.go).
	arb   *arbiter
	lease leaseCtl

	// rmu serialises SetRights, so the live bits and the admission's stored grants
	// stay in the same order; a leaf lock.
	rmu sync.Mutex

	vmu      sync.Mutex
	viewers  map[ViewerID]*viewer
	sizeW    int
	sizeH    int
	sizeKnow bool

	// kmu guards the keyframe coalescer; it is a leaf lock (never held while
	// taking another).
	kmu       sync.Mutex
	kfLast    time.Time
	kfPending bool
	kfTimer   *time.Timer

	// smu guards the derived-state flags below; it is a leaf lock (never held
	// while taking another, never across a native call). Every state transition
	// goes through a method that holds it, which serialises the stall watchdog,
	// the idle probe and the display read against each other.
	smu          sync.Mutex
	lastFrameAt  time.Time // wall clock of the last access unit
	permDenied   bool      // Screen Recording is off (needs_permission)
	asleep       bool      // the main display is asleep (display_asleep)
	locked       bool      // the session screen is locked (attribute)
	reasons      map[string]string
	pushed       msgState // the last state message broadcast
	pushedKnown  bool
	lastProbeAt  time.Time
	frames       uint64    // access units seen, for "a frame since" checks
	awaitSeq     uint64    // frames when the open lapse began (or its last retry ended)
	noDisplay    bool      // SCK said there is no display (-3815); sticky until a start succeeds
	nextStartAt  time.Time // no start retry before this
	rtOff        bool      // desktop-state.json is unreadable: nothing is written to it
	approvalSeen time.Time // Portty's own record of the last fresh approval
	approvalAt   time.Time // when the approval signals were last refreshed
	guiSeenAt    time.Time // the GUI's last NoteGUISeen
	lapseDue     time.Time // the daemon claims the open lapse at this time if the GUI has not
	paused       bool      // the crash-loop breaker is open (capture_paused)
	awaiting     bool      // the stall watchdog classified waiting for approval
	approaching  bool      // the approval record is 27 to 30 days old
	lapseSeq     int       // approval lapses so far
	lapseClaimed int       // the last lapse whose notification has a sender

	// pmu guards the power assertions (power.go); native calls run under it.
	pmu        sync.Mutex
	relDisplay func() error // the viewer display assertion, nil when not held
	relSystem  func() error // the viewer system-sleep assertion
	relAC      func() error // the always-available AC system-sleep assertion
	acAt       time.Time    // when the AC state was last read
	wakeAt     time.Time    // a wake awaiting its stream restart (guarded by smu)

	// lmu guards the Locked() cache; a leaf lock.
	lmu       sync.Mutex
	lockAt    time.Time
	lockVal   bool
	lockKnown bool

	tickMu                sync.Mutex // one state-loop pass at a time; also guards wd
	wd                    watchdog
	kick                  chan struct{} // a join or leave: re-pace the loop and tick now
	tickViewers, tickIdle time.Duration // the state loop's cadence (tests shorten it)
	loopCancel            context.CancelFunc
	loopDone              chan struct{}
}

// NewManager validates d.
func NewManager(d Deps) (*Manager, error) {
	if !d.Bind.IsValid() {
		return nil, ErrNoTailnetAddr
	}
	if d.MediaPort <= 0 || d.MediaPort > 65535 {
		return nil, errors.New("desktop: Deps.MediaPort out of range")
	}
	if d.NewSource == nil {
		return nil, errors.New("desktop: Deps.NewSource is nil")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.SourceConfig == (screencap.Config{}) {
		d.SourceConfig = screencap.DefaultConfig()
	}
	if d.Lock == nil {
		d.Lock = defaultLock
	}
	if d.DisplayAsleep == nil {
		d.DisplayAsleep = defaultDisplayAsleep
	}
	if d.OnAC == nil {
		d.OnAC = defaultOnAC
	}
	if d.HoldDisplay == nil {
		d.HoldDisplay = defaultHoldDisplay
	}
	if d.HoldSystem == nil {
		d.HoldSystem = defaultHoldSystem
	}
	if d.DeclareActivity == nil {
		d.DeclareActivity = defaultDeclare
	}
	if d.Nudge == nil {
		d.Nudge = defaultNudge
	}
	if d.Probe == nil {
		d.Probe = DefaultProber().Read
	}
	if d.ApprovalStore == nil {
		d.ApprovalStore = defaultApprovalStore
	}
	if d.Input == nil {
		d.Input = screencap.NewInputBridge()
	}
	if d.DisplayBounds == nil {
		d.DisplayBounds = screencap.DisplayBounds
	}
	if d.PointOnDisplay == nil {
		d.PointOnDisplay = screencap.PointOnDisplay
	}
	if d.MainDisplayID == nil {
		d.MainDisplayID = screencap.MainDisplayID
	}
	if d.ListSurfaces == nil {
		d.ListSurfaces = screencap.ListSurfaces
	}
	if d.Window == nil {
		d.Window = screencap.NewWindowBridge()
	}
	if d.Physical == nil {
		d.Physical = TapPhysicalSource()
	}
	if d.StartTap == nil {
		d.StartTap = screencap.StartInputTap
	}
	if d.StopTap == nil {
		d.StopTap = screencap.StopInputTap
	}
	if d.TapState == nil {
		d.TapState = screencap.InputTapState
	}
	if d.SecureInputOn == nil {
		d.SecureInputOn = screencap.SecureInputOn
	}
	if d.Pasteboard == nil {
		d.Pasteboard = screencap.NewPasteboard()
	}
	m := &Manager{
		d: d, ctx: context.Background(), viewers: make(map[ViewerID]*viewer),
		reasons: make(map[string]string), kick: make(chan struct{}, 1), tickViewers: stateTickViewers, tickIdle: stateTickIdle,
	}
	m.q = newQualityController(m)
	m.arb = newArbiter(d.Now, d.Physical)
	m.lease.kick = make(chan struct{}, 1)
	m.lease.sent = make(map[ViewerID]msgLease)
	m.in.epoch.Store(1)
	m.in.counts = make(map[string]uint64)
	m.in.wake = make(chan struct{}, 1)
	if m.d.OnDataChannel == nil {
		m.d.OnDataChannel = m.acceptDataChannel
	}
	m.loadRuntimeState()
	return m, nil
}

// Start binds the media socket (tailnet address only, a busy port retried) and
// builds the pion API.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		return errors.New("desktop: manager closed")
	}
	if m.api != nil {
		return errors.New("desktop: manager already started")
	}
	conn, err := listenMedia(ctx, m.d.Bind, m.d.MediaPort)
	if err != nil {
		return err
	}
	var pconn net.PacketConn = conn
	if m.wrapConn != nil {
		pconn = m.wrapConn(conn)
	}
	mux := ice.NewUDPMuxDefault(ice.UDPMuxParams{UDPConn: pconn})
	api, err := newAPI(mux, m.d.Bind, m.d.Now, &m.bwe)
	if err != nil {
		_ = mux.Close()
		_ = conn.Close()
		return err
	}
	m.ctx, m.conn, m.mux, m.api = ctx, conn, mux, api

	lctx, cancel := context.WithCancel(ctx)
	m.loopCancel, m.loopDone = cancel, make(chan struct{})
	wake := m.d.Wake
	if wake == nil {
		wake = wakewatch.Watch(lctx)
	}
	m.refreshAC(m.d.Now().Round(0), true)
	go m.stateLoop(lctx, wake, m.loopDone)
	m.surf.done = make(chan struct{})
	go m.surfaceLoop(lctx, m.surf.done)
	m.win.done = make(chan struct{})
	go m.windowLoop(lctx, m.win.done)
	m.lease.done = make(chan struct{})
	go m.leaseLoop(lctx, m.lease.done)

	// The input goroutine outlives the state loop's context, and the caller's:
	// the daemon cancels ctx before it calls Close, and Close drains the queued
	// releases through this goroutine before it cancels it. Only Close ends it.
	ictx, icancel := context.WithCancel(context.WithoutCancel(ctx))
	m.in.mu.Lock()
	m.in.cancel, m.in.done = icancel, make(chan struct{})
	m.in.mu.Unlock()
	go m.inputLoop(ictx, m.in.done)
	return nil
}

// recoverLog turns a panic into a log line (the panic type only), so a bug in
// one desktop goroutine ends that viewer, never the daemon.
func recoverLog() {
	if r := recover(); r != nil {
		slog.Error("desktop: recovered", "panic", fmt.Sprintf("%T", r))
	}
}

// send writes one JSON message with a bounded wait.
func send(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, b)
}

// read reads one frame; d bounds the wait when positive. A frame that is not a
// JSON object returns errBadMessage, any other error means the socket is gone.
var errBadMessage = errors.New("desktop: bad message")

func read(ctx context.Context, c *websocket.Conn, d time.Duration) (inbound, error) {
	if d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	_, b, err := c.Read(ctx)
	if err != nil {
		return inbound{}, err
	}
	var in inbound
	if err := json.Unmarshal(b, &in); err != nil {
		return inbound{}, errBadMessage
	}
	return in, nil
}

// reject tells the viewer why and closes the socket with a protocol error.
func reject(ctx context.Context, c *websocket.Conn, code string) {
	_ = send(ctx, c, msgError{Type: typeError, Code: code})
	_ = c.Close(websocket.StatusProtocolError, code)
}

func newViewerID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ServeDesktopViewer runs one viewer's signalling session on an accepted
// WebSocket until it closes. The route layer has already checked Origin,
// account and token; admit re-checks the token when the viewer joins (nil
// admits nobody). Logs carry counts, viewer ID prefixes and error codes
// only: never a token, SDP or address.
func (m *Manager) ServeDesktopViewer(ctx context.Context, c *websocket.Conn, admit func() bool) {
	m.ServeDesktopViewerAs(ctx, c, admit, Identity{})
}

// ServeDesktopViewerAs is ServeDesktopViewer for a caller that knows who the
// own-account viewer is; the identity is recorded on the Viewer. The wire is
// the same as ServeDesktopViewer's.
func (m *Manager) ServeDesktopViewerAs(ctx context.Context, c *websocket.Conn, admit func() bool, ident Identity) {
	defer recoverLog()
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(readLimit)

	// A full house is refused before any handshake work; join checks again for
	// viewers that were mid-handshake together.
	if m.Full() {
		_ = c.Close(CloseFull, "full")
		return
	}

	// hello
	in, err := read(ctx, c, helloTimeout)
	if err != nil {
		if errors.Is(err, errBadMessage) {
			reject(ctx, c, errCodeBadMessage)
		}
		return
	}
	if in.Type != typeHello {
		reject(ctx, c, errCodeBadMessage)
		return
	}
	if in.V != ProtocolVersion {
		reject(ctx, c, errCodeVersion)
		return
	}
	id, err := newViewerID()
	if err != nil {
		slog.Error("desktop: viewer id", "err", err)
		return
	}
	inputCap := in.Caps["input"]
	rights := RightsOwnAccount
	hello := msgHello{
		Type: typeHello, V: ProtocolVersion, Viewer: id,
		Caps: map[string]int{"video": 1, "input": 1, "audio": 1},
		Rights: map[string]bool{
			"control":   rights&RightsControl != 0,
			"clipboard": rights&RightsClipboard != 0,
			"surface":   rights&RightsSurface != 0,
		},
	}
	if err := send(ctx, c, hello); err != nil {
		return
	}

	v := &viewer{Viewer: Viewer{ID: ViewerID(id), Rights: rights, JoinedAt: m.d.Now(), Mode: ModeOwn, Identity: ident}, ws: c, inputCap: inputCap, audioCap: in.Caps["audio"], in: &viewerInput{}}
	v.setRightsBits(rights)
	m.serveAdmitted(ctx, c, v, admit, nil)
}

// readResult is one frame read off a viewer's socket on another goroutine.
type readResult struct {
	in  inbound
	err error
}

// serveAdmitted is everything after the hello: the offer, the answer, the join
// and the read loop. v is fully built (ID, rights, mode, identity). pend, when
// not nil, is a read already in flight on c (a guest's wait loop leaves one
// running); its result is taken before the socket is read again.
func (m *Manager) serveAdmitted(ctx context.Context, c *websocket.Conn, v *viewer, admit func() bool, pend <-chan readResult) {
	id := string(v.ID)
	// offer: a ping before it is answered, anything else is refused.
	offerDeadline := time.Now().Add(offerTimeout)
	var offer string
	for offer == "" {
		rem := time.Until(offerDeadline)
		if rem <= 0 {
			return // read treats a zero wait as unbounded: pings must not extend the handshake
		}
		var in inbound
		var err error
		if pend != nil {
			select {
			case r := <-pend:
				in, err = r.in, r.err
				pend = nil
			case <-time.After(rem):
				return
			}
		} else {
			in, err = read(ctx, c, rem)
		}
		if err != nil {
			if errors.Is(err, errBadMessage) {
				reject(ctx, c, errCodeBadMessage)
			}
			return
		}
		switch in.Type {
		case typePing:
			if send(ctx, c, msgPong{Type: typePong, N: in.N}) != nil {
				return
			}
		case typeOffer:
			offer = in.SDP
			if offer == "" {
				reject(ctx, c, errCodeBadMessage)
				return
			}
		default:
			reject(ctx, c, errCodeBadMessage)
			return
		}
	}

	pc, answerSDP, err := m.answer(ctx, v, offer)
	if err != nil {
		slog.Warn("desktop: offer failed", "viewer", id[:8])
		reject(ctx, c, errCodeOfferFailed)
		return
	}
	v.pc = pc
	defer func() { _ = pc.Close() }()
	// answer registered the viewer's estimator; whatever way this returns, it goes.
	defer m.q.unregister(v.ID)
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		switch st {
		case webrtc.PeerConnectionStateConnected:
			// The viewer gets nothing until a keyframe written after this point,
			// so ask for one now. (A request at join would be wasted: the media
			// path is not up yet, and it would use up the coalescing window.)
			m.vmu.Lock()
			v.ready = true
			m.vmu.Unlock()
			m.requestKeyframe("connected")
			if v.Mode == ModeGuest {
				m.checkSelectedPair(v)
			}
		case webrtc.PeerConnectionStateFailed:
			// Close the socket so the viewer rebuilds from scratch, not a zombie
			// that keeps capture running.
			go func() { _ = c.Close(websocket.StatusGoingAway, "media failed") }()
		}
	})

	if v.Mode == ModeGuest {
		m.bindGuestMedia(v, pc)
	}

	// Capture starts only after a successful answer, so a refused offer never
	// starts the source.
	if err := m.join(v, admit); err != nil {
		if errors.Is(err, errFull) {
			_ = c.Close(CloseFull, "full")
			return
		}
		if errors.Is(err, errRevoked) {
			reason := "token rejected"
			if v.Mode == ModeGuest {
				reason = guestReasonTokenInvalid // a fixed code the guest page reads
			}
			_ = c.Close(CloseTokenRejected, reason)
			return
		}
		slog.Warn("desktop: join failed", "viewer", id[:8], "err", err)
		_ = c.Close(websocket.StatusInternalError, "capture unavailable")
		return
	}
	joined := false
	defer func() {
		m.leave(v)
		if joined && m.d.OnViewerLeave != nil {
			m.d.OnViewerLeave(v.Viewer)
		}
	}()
	if v.gs != nil {
		// Registered above, so a Stop or revoke either finds v and cuts it or has
		// already happened and is seen here: neither can be missed.
		select {
		case <-v.gs.Ended():
			_ = c.Close(CloseShareEnded, guestReasonShareEnded)
			return
		default:
		}
		if v.gs.IsDenied(v.Identity.UserID) {
			_ = c.Close(CloseGuestRevoked, guestReasonRevoked)
			return
		}
		key := AdmissionKey{UserID: v.Identity.UserID, StableID: v.Identity.StableID}
		v.gs.attach(key, v.ID, func() { _ = c.Close(websocket.StatusNormalClosure, guestReasonReplaced) })
		defer v.gs.detach(key, v.ID)
	}
	joined = true
	m.syncTap() // the first controller starts the tap before its first press
	m.kickLease()
	if m.d.OnViewerJoin != nil {
		m.d.OnViewerJoin(v.Viewer)
	}

	if err := send(ctx, c, msgAnswer{Type: typeAnswer, SDP: answerSDP}); err != nil {
		return
	}
	v.leaseReady.Store(true) // lease messages follow the answer, never precede it
	m.kickLease()
	m.broadcastState(ctx)
	if w, h, ok := m.size(); ok {
		_ = send(ctx, c, msgSize{Type: typeSize, W: w, H: h})
	}
	_ = send(ctx, c, m.surfaceMsg(""))
	slog.Info("desktop: viewer joined", "viewer", id[:8], "viewers", m.Viewers())

	for {
		in, err := read(ctx, c, 0)
		if err != nil {
			if errors.Is(err, errBadMessage) {
				reject(ctx, c, errCodeBadMessage)
			}
			return
		}
		switch in.Type {
		case typePing:
			if send(ctx, c, msgPong{Type: typePong, N: in.N}) != nil {
				return
			}
		case typeSurfaces:
			m.handleSurfaces(ctx, v)
		case typeSelect:
			m.handleSelect(ctx, v, in.Kind, in.ID)
		case typeClipGet:
			m.handleClipGet(ctx, v, in.Xid)
		case typeClipPut:
			m.handleClipPut(ctx, v, in)
		}
	}
}

var (
	errFull    = errors.New("desktop: viewer cap reached")
	errRevoked = errors.New("desktop: token no longer valid")
)

// join registers v and starts capture for the first viewer. admit is the
// token re-check: a key regenerated during the handshake refuses v here.
func (m *Manager) join(v *viewer, admit func() bool) error {
	m.mu.Lock()
	if m.closed.Load() {
		m.mu.Unlock()
		return errors.New("desktop: manager closed")
	}
	if m.Full() {
		m.mu.Unlock()
		return errFull
	}
	// Checked in the same vmu hold as the registration: CloseAll takes vmu too,
	// so a revocation either finds v or this check sees the new key. Registered
	// before capture starts, so a start that finishes after every viewer left
	// knows nobody is waiting for it.
	m.vmu.Lock()
	if admit == nil || !admit() {
		m.vmu.Unlock()
		m.mu.Unlock()
		return errRevoked
	}
	m.viewers[v.ID] = v
	m.vmu.Unlock()
	// Remote activity lights a dark display before capture starts; viewers hold
	// the display and system assertions until the last leaves (a join retries
	// one that failed).
	m.declareActivity()
	m.takeViewerHolds()
	start := m.src == nil && !m.starting && !m.isPaused()
	if start {
		m.starting = true
	}
	m.mu.Unlock()
	m.kickLoop()
	if !start {
		return nil
	}
	// A viewer that cannot be served yet (no Screen Recording grant, no
	// display) stays joined: it is told why, and the state loop retries.
	if err := m.startSource(); err != nil && !errors.Is(err, screencap.ErrPermissionDenied) && !errors.Is(err, screencap.ErrNoDisplay) {
		m.leave(v)
		return err
	}
	return nil
}

// startSource starts a fresh source and its fan-out; the caller set m.starting
// under mu. The native start runs without mu: it can block for about 35 s (a
// pending macOS dialog), and joins, leaves, keyframes, the state loop and
// Close must not wait behind it (WR-01). A start error is classified (-3801
// needs permission, -3815 no display) before it is returned. Every viewer
// waits for the new source's first keyframe.
func (m *Manager) startSource() error {
	src := m.d.NewSource()
	sctx, cancel := context.WithCancel(m.ctx)
	now := m.d.Now().Round(0)
	cfg := m.d.SourceConfig
	// A restart keeps the surface; a first start after the last viewer left is the main display.
	var cfgInfo screencap.SurfaceInfo
	cfg.Surface, cfgInfo = m.currentSurface()
	err := src.Start(sctx, cfg)
	resync := false
	defer func() {
		// After mu is released: viewers learn what the stream really shows.
		if resync {
			m.releaseHeldAll("surface")
			m.broadcast(m.ctx, m.surfaceMsg(""))
		}
	}()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starting = false
	if err != nil {
		cancel()
		m.noteStartFailure(err, now)
		return err
	}
	if m.closed.Load() || m.Viewers() == 0 {
		// Stopped or abandoned while starting: nobody to serve, nothing recorded.
		cancel()
		_ = src.Stop()
		return nil
	}
	// A select that ran while this start was in flight only recorded its choice.
	if cur := m.surfaceSel(); cur != cfg.Surface {
		if err := src.SelectSurface(cur); err != nil {
			// The stream shows cfg.Surface: record it, so input never maps onto
			// a surface the viewers cannot see (M-01).
			slog.Warn("desktop: surface switch after start failed", "err", err)
			m.setSurface(cfg.Surface, cfgInfo)
			m.bumpEpoch()
			resync = true
		}
	}
	m.src, m.srcStop = src, cancel
	m.fanDone = make(chan struct{})
	m.vmu.Lock()
	for _, v := range m.viewers {
		v.armed = false
	}
	m.vmu.Unlock()
	m.noteStartSuccess(src, now)
	go m.fanOut(src, m.fanDone)
	m.q.start(sctx, src)
	if as, ok := src.(screencap.AudioSource); ok {
		// A new capture run starts with a clean audio state: a failure it has is
		// still queued on AudioErrors and sets the flag again.
		m.audioFail.Store(false)
		m.audioDone = make(chan struct{})
		go m.audioOut(sctx, as, m.audioDone)
	}
	return nil
}

// restartSource stops the source, if any, and starts a new one (stall recovery,
// wake, retry). It does nothing without a viewer or while a start is in flight.
func (m *Manager) restartSource() error {
	m.mu.Lock()
	if m.closed.Load() || m.Viewers() == 0 || m.starting {
		m.mu.Unlock()
		return nil
	}
	m.stopSourceLocked()
	m.starting = true
	m.mu.Unlock()
	return m.startSource()
}

// leave unregisters v and stops capture when it was the last viewer (capture
// only runs while someone is watching).
func (m *Manager) leave(v *viewer) {
	m.ReleaseViewer(v.ID, "left")
	m.arb.forget(v.ID)
	m.clip.drop(v.ID)
	m.mu.Lock()
	m.vmu.Lock()
	delete(m.viewers, v.ID)
	n := len(m.viewers)
	m.vmu.Unlock()
	if n == 0 {
		m.stopSourceLocked()
		m.resetSurface() // the surface is per viewing session, never persisted
		m.noteCaptureStopped()
		m.releaseViewerHolds()
	}
	m.mu.Unlock()
	m.syncTap()
	m.kickLease()
	m.kickLoop()
	slog.Info("desktop: viewer left", "viewer", string(v.ID)[:8], "viewers", n)
	m.broadcastState(context.Background())
}

// stopSourceLocked (m.mu held) stops the source and waits for the fan-out to
// drain. A start in flight is not stopped here: it finds no viewer (or a
// closed Manager) when it finishes and stops itself.
func (m *Manager) stopSourceLocked() {
	if m.src == nil {
		return
	}
	m.srcStop()
	m.q.stop() // before the source stops: it may be inside a setter on it
	_ = m.src.Stop()
	<-m.fanDone
	if m.audioDone != nil {
		<-m.audioDone
	}
	m.src, m.srcStop, m.fanDone, m.audioDone = nil, nil, nil, nil
	m.vmu.Lock()
	m.sizeKnow = false
	m.vmu.Unlock()
}

// requestKeyframe asks the source for a keyframe, coalesced: at most one per
// keyframeCoalesce by Deps.Now. A request inside the window schedules one
// deferred keyframe at the window's end and later requests ride on it.
func (m *Manager) requestKeyframe(reason string) {
	m.kmu.Lock()
	if m.kfPending {
		m.kmu.Unlock()
		return
	}
	now := m.d.Now()
	if since := now.Sub(m.kfLast); !m.kfLast.IsZero() && since < keyframeCoalesce {
		m.kfPending = true
		m.kfTimer = time.AfterFunc(keyframeCoalesce-since, m.fireKeyframe)
		m.kmu.Unlock()
		return
	}
	m.kfLast = now
	m.kmu.Unlock()
	slog.Debug("desktop: keyframe", "reason", reason)
	m.forceKeyframeNow()
}

// fireKeyframe is the deferred half of requestKeyframe.
func (m *Manager) fireKeyframe() {
	defer recoverLog()
	m.kmu.Lock()
	m.kfPending = false
	m.kfLast = m.d.Now()
	m.kmu.Unlock()
	m.forceKeyframeNow()
}

func (m *Manager) forceKeyframeNow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.src != nil {
		m.src.ForceKeyframe()
	}
}

// fanOut packetizes every access unit of src once and writes it to each
// viewer's track until Frames closes, announcing a changed picture size to the
// viewers. SSRC and payload type are set per viewer by its track.
func (m *Manager) fanOut(src screencap.Source, done chan<- struct{}) {
	defer close(done)
	defer recoverLog()
	pkt := rtp.NewPacketizer(outboundMTU, 0, 0, &codecs.H264Payloader{}, rtp.NewRandomSequencer(), h264Capability.ClockRate)
	frames, errs := src.Frames(), src.Errors()
	for {
		select {
		case au, ok := <-frames:
			if !ok {
				return
			}
			// noteUnit says whether this unit is a captured frame for the stall
			// watchdog (every unit is, until a refine unit exists).
			if m.noteUnit(au) {
				m.noteFrame()
			}
			if m.noteSize(au.Width, au.Height) {
				m.broadcast(context.Background(), msgSize{Type: typeSize, W: au.Width, H: au.Height})
			}
			if m.isAsleep() {
				// A cached frame re-encoded for a joiner is stale while the
				// display is dark: the viewer is shown the state instead.
				continue
			}
			m.writeUnit(au.Keyframe, packetize(pkt, au))
		case err, ok := <-errs:
			if !ok {
				errs = nil // a nil channel never fires; the frames case ends the loop
				continue
			}
			slog.Warn("desktop: capture error", "err", err)
			m.noteSourceError(err)
			m.pushIfChanged(context.Background())
		}
	}
}

// packetize stamps au at its own capture time. Duration is the gap since the
// previous unit, and pion stamps a unit before it adds its samples, so the
// clock is advanced first (WR-02). Integer ticks wrap mod 2^32 as RTP
// timestamps do, so an idle gap of any length stays well-defined.
// shortcut: each step truncates to whole 90 kHz ticks (under 11 us a unit).
func packetize(pkt rtp.Packetizer, au screencap.AccessUnit) []*rtp.Packet {
	rate, d := uint64(h264Capability.ClockRate), au.Duration
	pkt.SkipSamples(uint32(uint64(d/time.Second)*rate + uint64(d%time.Second)*rate/uint64(time.Second)))
	return pkt.Packetize(au.Data, 0)
}

// writeUnit sends one access unit's packets to every viewer whose media path
// is up. A viewer is armed by the first keyframe after that and receives no
// delta before it, so no decoder ever starts on a picture it cannot show.
func (m *Manager) writeUnit(keyframe bool, pkts []*rtp.Packet) {
	m.vmu.Lock()
	targets := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		if !v.ready || (!v.armed && !keyframe) {
			continue
		}
		v.armed = true
		targets = append(targets, v)
	}
	m.vmu.Unlock()
	for _, v := range targets {
		for _, p := range pkts {
			// A write error means that viewer's PeerConnection is closing; its
			// socket close removes it, and the others must still be served.
			_ = v.track.WriteRTP(p)
		}
	}
}

// noteSize records the latest picture size and reports whether it changed.
func (m *Manager) noteSize(w, h int) bool {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	if m.sizeKnow && m.sizeW == w && m.sizeH == h {
		return false
	}
	m.sizeW, m.sizeH, m.sizeKnow = w, h, true
	return true
}

func (m *Manager) size() (w, h int, ok bool) {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	return m.sizeW, m.sizeH, m.sizeKnow
}

// broadcast sends v to every viewer in turn. shortcut: writes are sequential,
// so a stalled viewer delays the others by up to writeTimeout once before its
// socket is dropped; per-viewer queues if that ever shows up in the VM rows.
func (m *Manager) broadcast(ctx context.Context, v any) {
	m.vmu.Lock()
	conns := make([]*websocket.Conn, 0, len(m.viewers))
	for _, vw := range m.viewers {
		conns = append(conns, vw.ws)
	}
	m.vmu.Unlock()
	for _, c := range conns {
		_ = send(ctx, c, v)
	}
}

func (m *Manager) broadcastState(ctx context.Context) {
	msg := m.stateMsg(m.Status())
	m.smu.Lock()
	m.pushed, m.pushedKnown = msg, true
	m.smu.Unlock()
	m.broadcast(ctx, msg)
}

// Full reports whether the viewer cap is reached. The route checks it before
// upgrading a socket so a viewer past the cap gets HTTP 429.
func (m *Manager) Full() bool { return m.Viewers() >= MaxViewers }

// Viewers reports how many viewers are connected.
func (m *Manager) Viewers() int {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	return len(m.viewers)
}

// ViewerList returns a copy of every connected viewer's record, oldest first.
// Rights is the live value.
func (m *Manager) ViewerList() []Viewer {
	m.vmu.Lock()
	out := make([]Viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vw := v.Viewer
		vw.Rights = Rights(v.rights.Load())
		out = append(out, vw)
	}
	m.vmu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out
}

// Status returns the link's derived state (state.go).
func (m *Manager) Status() Status { return m.deriveState() }

// CloseAll closes every viewer socket with code and reason.
func (m *Manager) CloseAll(code websocket.StatusCode, reason string) {
	m.vmu.Lock()
	conns := make([]*websocket.Conn, 0, len(m.viewers))
	for _, v := range m.viewers {
		conns = append(conns, v.ws)
	}
	m.vmu.Unlock()
	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Close(code, reason)
		}()
	}
	wg.Wait()
}

// Close ends every viewer, stops capture and releases the media socket.
func (m *Manager) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	// First, before taking mu or closing any viewer: a close handshake can take
	// seconds and the process may exit first (Quit All, an upgrade's 1 s drain).
	// With closed set no start records captureActive again (noteStartSuccess
	// checks it under the state file's lock), so this stays false and the next
	// process counts no abnormal exit. A real crash never reaches this line.
	m.noteCaptureStopped()
	// Everything a viewer holds is let go before anything else closes, so a
	// handoff or quit never leaves input held on this process's behalf.
	m.releaseAllViewers("closing")
	m.in.mu.Lock()
	icancel, idone := m.in.cancel, m.in.done
	m.in.mu.Unlock()
	if icancel != nil {
		icancel()
		<-idone
		// Nothing runs now: a later flushInput must return at once, not wait flushWait.
		m.in.mu.Lock()
		m.in.done = nil
		m.in.mu.Unlock()
	}
	m.mu.Lock()
	cancel, done, surfDone, winDone, leaseDone := m.loopCancel, m.loopDone, m.surf.done, m.win.done, m.lease.done
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
		<-surfDone
		<-winDone
		<-leaseDone
	}
	m.syncTap() // closed is set: no controller counts, so the tap stops

	m.CloseAll(websocket.StatusServiceRestart, "server stopping")
	m.kmu.Lock()
	if m.kfTimer != nil {
		m.kfTimer.Stop()
	}
	m.kmu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopSourceLocked()
	m.releaseAllHolds()
	m.vmu.Lock()
	for _, v := range m.viewers {
		_ = v.pc.Close()
	}
	m.vmu.Unlock()
	var err error
	if m.mux != nil {
		err = m.mux.Close()
	}
	if m.conn != nil {
		_ = m.conn.Close()
	}
	return err
}
