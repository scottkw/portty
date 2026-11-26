// Package desktop serves the always-available desktop link: one shared H.264
// track fed by a screencap.Source, one pion PeerConnection per viewer, and the
// versioned signalling protocol the /desktop/ws route carries.
//
// Signalling protocol v1 (JSON text frames, each with a string "type", 32 KiB
// read limit):
//
//	viewer to server: hello {v, caps} first, then offer {sdp} (complete,
//	non-trickle), and ping {n} at any time.
//	server to viewer: hello {v, viewer, caps}, answer {sdp}, state {state,
//	viewers, locked}, size {w, h}, pong {n}, error {code}.
//
// Unknown caps keys and unknown message types after the offer are ignored, so
// Phase 20 adds an "input" capability without a version bump. Close codes:
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
	"time"

	"github.com/coder/websocket"
	"github.com/pion/ice/v4"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/screencap"
)

// ProtocolVersion is the signalling protocol version in hello.
const ProtocolVersion = 1

// Close codes beyond the standard ones (1012 is websocket.StatusServiceRestart).
const (
	CloseTokenRejected websocket.StatusCode = 4401
	CloseDesktopOff    websocket.StatusCode = 4410
)

// State is what the desktop link is doing, pushed to viewers and the host GUI.
// 19-01 only produces idle and streaming; 19-09 derives the rest.
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

// inbound is every viewer-to-server message (hello, offer, ping) decoded
// loosely: the type decides which fields matter, and unknown fields are ignored.
type inbound struct {
	Type string `json:"type"`
	V    int    `json:"v"`
	SDP  string `json:"sdp"`
	N    int    `json:"n"`
}

// Deps is what the Manager needs from its host.
type Deps struct {
	Bind         netip.Addr              // the one address media is served on
	MediaPort    int                     // the one UDP port, fixed because it travels in the SDP
	NewSource    func() screencap.Source // a fresh source per capture run
	SourceConfig screencap.Config
	ConfigDir    string
	Now          func() time.Time
	// Notify posts a native notification. Used from 19-09 on; nil means log-only.
	Notify func(id, title, body string)
}

// Status is the desktop link's state as the host sees it. 19-01 fills State
// (idle or streaming) and Viewers; 19-09 derives the rest.
type Status struct {
	State            State
	Viewers          int
	Locked           bool
	ApproachingLapse bool
	LapseSeq         int
	Reason           string
}

// viewer is one connected viewer: its public record, signalling socket,
// PeerConnection and outbound track. ready and armed are guarded by Manager.vmu.
type viewer struct {
	Viewer
	ws    *websocket.Conn
	pc    *webrtc.PeerConnection
	track *webrtc.TrackLocalStaticRTP
	ready bool // the media path is up (PeerConnection Connected)
	armed bool // a keyframe has been written to this viewer, deltas may follow
}

// Manager owns the media socket, the shared track, the capture source and the
// connected viewers.
type Manager struct {
	d Deps

	// mu guards the lifecycle (socket, API, capture source) and is held across
	// a source stop. vmu guards the viewer set and the last picture size; the
	// fan-out goroutine takes only vmu, so a stop that waits for it under mu
	// cannot deadlock. Lock order: mu, then vmu.
	mu      sync.Mutex
	ctx     context.Context
	conn    *net.UDPConn
	mux     ice.UDPMux
	api     *webrtc.API
	closed  bool
	src     screencap.Source
	srcStop context.CancelFunc
	fanDone chan struct{}

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
}

// NewManager validates d.
func NewManager(d Deps) (*Manager, error) {
	if !d.Bind.IsValid() {
		return nil, errors.New("desktop: Deps.Bind is not a valid address")
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
	return &Manager{d: d, ctx: context.Background(), viewers: make(map[ViewerID]*viewer)}, nil
}

// Start binds the media socket and builds the pion API. 19-03 wraps the bind
// with the tailnet-only check and a retry.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("desktop: manager closed")
	}
	if m.api != nil {
		return errors.New("desktop: manager already started")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: m.d.Bind.AsSlice(), Port: m.d.MediaPort})
	if err != nil {
		return fmt.Errorf("desktop: listen media socket: %w", err)
	}
	mux := ice.NewUDPMuxDefault(ice.UDPMuxParams{UDPConn: conn})
	api, err := newAPI(mux, m.d.Bind)
	if err != nil {
		_ = mux.Close()
		_ = conn.Close()
		return err
	}
	m.ctx, m.conn, m.mux, m.api = ctx, conn, mux, api
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
// account and token. Logs carry counts, viewer ID prefixes and error codes
// only: never a token, SDP or address.
func (m *Manager) ServeDesktopViewer(ctx context.Context, c *websocket.Conn) {
	defer recoverLog()
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(readLimit)

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
	if err := send(ctx, c, msgHello{Type: typeHello, V: ProtocolVersion, Viewer: id, Caps: map[string]int{"video": 1}}); err != nil {
		return
	}

	// offer: a ping before it is answered, anything else is refused.
	offerDeadline := time.Now().Add(offerTimeout)
	var offer string
	for offer == "" {
		in, err := read(ctx, c, time.Until(offerDeadline))
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

	v := &viewer{Viewer: Viewer{ID: ViewerID(id), Rights: RightsViewOnly, JoinedAt: m.d.Now()}, ws: c}
	pc, answerSDP, err := m.answer(ctx, v, offer)
	if err != nil {
		slog.Warn("desktop: offer failed", "viewer", id[:8])
		reject(ctx, c, errCodeOfferFailed)
		return
	}
	v.pc = pc
	defer func() { _ = pc.Close() }()
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
		case webrtc.PeerConnectionStateFailed:
			// Close the socket so the viewer rebuilds from scratch, not a zombie
			// that keeps capture running.
			go func() { _ = c.Close(websocket.StatusGoingAway, "media failed") }()
		}
	})

	// Capture starts only after a successful answer, so a refused offer never
	// starts the source.
	if err := m.join(v); err != nil {
		slog.Warn("desktop: join failed", "viewer", id[:8], "err", err)
		_ = c.Close(websocket.StatusInternalError, "capture unavailable")
		return
	}
	defer m.leave(v)

	if err := send(ctx, c, msgAnswer{Type: typeAnswer, SDP: answerSDP}); err != nil {
		return
	}
	m.broadcastState(ctx)
	if w, h, ok := m.size(); ok {
		_ = send(ctx, c, msgSize{Type: typeSize, W: w, H: h})
	}
	slog.Info("desktop: viewer joined", "viewer", id[:8], "viewers", m.Viewers())

	for {
		in, err := read(ctx, c, 0)
		if err != nil {
			if errors.Is(err, errBadMessage) {
				reject(ctx, c, errCodeBadMessage)
			}
			return
		}
		if in.Type == typePing {
			if send(ctx, c, msgPong{Type: typePong, N: in.N}) != nil {
				return
			}
		}
	}
}

// join registers v and starts capture for the first viewer.
func (m *Manager) join(v *viewer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("desktop: manager closed")
	}
	if m.src == nil {
		src := m.d.NewSource()
		sctx, cancel := context.WithCancel(m.ctx)
		if err := src.Start(sctx, m.d.SourceConfig); err != nil {
			cancel()
			return err
		}
		m.src, m.srcStop = src, cancel
		m.fanDone = make(chan struct{})
		go m.fanOut(src, m.fanDone)
	}
	m.vmu.Lock()
	m.viewers[v.ID] = v
	m.vmu.Unlock()
	return nil
}

// leave unregisters v and stops capture when it was the last viewer (capture
// only runs while someone is watching).
func (m *Manager) leave(v *viewer) {
	m.mu.Lock()
	m.vmu.Lock()
	delete(m.viewers, v.ID)
	n := len(m.viewers)
	m.vmu.Unlock()
	if n == 0 {
		m.stopSourceLocked()
	}
	m.mu.Unlock()
	slog.Info("desktop: viewer left", "viewer", string(v.ID)[:8], "viewers", n)
	m.broadcastState(context.Background())
}

// stopSourceLocked (m.mu held) stops the source and waits for the fan-out to
// drain; holding mu serialises a stop against the next join's start.
func (m *Manager) stopSourceLocked() {
	if m.src == nil {
		return
	}
	m.srcStop()
	_ = m.src.Stop()
	<-m.fanDone
	m.src, m.srcStop, m.fanDone = nil, nil, nil
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
			if m.noteSize(au.Width, au.Height) {
				m.broadcast(context.Background(), msgSize{Type: typeSize, W: au.Width, H: au.Height})
			}
			// shortcut: the timestamp step truncates to whole ticks, exact at 30 fps
			// (3000); fractional-tick drift if a source ever runs at an odd rate.
			m.writeUnit(au.Keyframe, pkt.Packetize(au.Data, uint32(au.Duration.Seconds()*float64(h264Capability.ClockRate))))
		case err := <-errs:
			slog.Warn("desktop: capture error", "err", err)
		}
	}
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
	st := m.Status()
	m.broadcast(ctx, msgState{Type: typeState, State: st.State, Viewers: st.Viewers, Locked: st.Locked})
}

// Viewers reports how many viewers are connected.
func (m *Manager) Viewers() int {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	return len(m.viewers)
}

// ViewerList returns a copy of every connected viewer's record, oldest first.
// Later phases read rights from it.
func (m *Manager) ViewerList() []Viewer {
	m.vmu.Lock()
	out := make([]Viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		out = append(out, v.Viewer)
	}
	m.vmu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out
}

// Status returns the link's state. 19-09 derives the other states here.
func (m *Manager) Status() Status {
	n := m.Viewers()
	if n > 0 {
		return Status{State: StateStreaming, Viewers: n}
	}
	return Status{State: StateIdle}
}

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
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.mu.Unlock()

	m.CloseAll(websocket.StatusServiceRestart, "server stopping")
	m.kmu.Lock()
	if m.kfTimer != nil {
		m.kfTimer.Stop()
	}
	m.kmu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopSourceLocked()
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
