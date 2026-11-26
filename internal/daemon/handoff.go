package daemon

// Phase 9 session handoff (UPGR-01, UPGR-03; Research Patterns 7 and 8).
//
// A takeover daemon (the new build) dials the running daemon's socket and
// upgrades POST /handoff to a framed protocol on the hijacked unix conn:
//
//	HELLO -> HELLO-OK | REFUSE
//	GLOBAL + fds [lock, API listener, relay listener, share listener?]
//	SESSION + fds [master, slave], once per live session
//	READY {digests} | ABORT
//	COMMIT -> (the takeover serves) -> COMMITTED
//
// Only one process ever reads a PTY: the old daemon until FREEZE, nobody
// until COMMIT, the takeover after COMMIT. Before COMMIT the old daemon keeps
// every fd and resumes on any failure. After COMMIT it resumes only once the
// takeover process has exited (watched from HELLO by its kernel-attested
// pid), and a takeover that cannot confirm COMMITTED to a living old daemon
// exits at once.
//
// shortcut: a crash between COMMIT and COMMITTED loses the few output bytes
// the takeover read before it died (CONTEXT decision, Research A6); the
// window is milliseconds and the shells survive either way. Shrinking it
// further needs the takeover to buffer reads until COMMITTED.
//
// shortcut: a daemon started by `portty daemon install` (launchd, opt-in,
// KeepAlive false) hands off like any other, and afterwards `portty daemon
// stop` (launchctl) no longer stops the takeover, which launchd does not
// manage. Nothing in Portty installs the agent; special-case it if it ever
// becomes the default.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/fdpass"
	"github.com/scottkw/portty/internal/pty"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"golang.org/x/sys/unix"
)

// HandoffVersion is the handoff wire protocol, advertised as "handoff" in
// /health. Two independently installed builds speak it, so a change must
// keep speaking version 1 or refuse cleanly (the takeover then falls back to
// the DEBT-08 restart question).
const HandoffVersion = 1

const handoffUpgrade = "portty-handoff/1"

// Timeouts, vars so tests can shorten them. The longest the old daemon
// answers writes with 503 is writeDrainTimeout + freezeTimeout +
// stateSendTimeout + readyTimeout + committedWatchdog, plus restartDrain after
// COMMITTED (kept-alive clients still reach it) = 19 s. One 1 s Retry-After
// wait later the client's 20 s frozenRetryBudget (client.go) still has a
// retry left, so a write that met the freeze gate's 503 still lands
// (TestHandoff_TimeoutsFitClientRetry).
var (
	writeDrainTimeout   = 1 * time.Second  // old side: writes in flight at the freeze finish (WR-01)
	freezeTimeout       = 2 * time.Second  // relay Hub.Freeze's own bound; hubs freeze concurrently
	stateSendTimeout    = 5 * time.Second  // old side: one write deadline for the whole STATE send
	readyTimeout        = 5 * time.Second  // old side: READY after the state was sent
	committedWatchdog   = 5 * time.Second  // new side: COMMIT to COMMITTED, else the process exits
	takeoverStepTimeout = 10 * time.Second // new side: read deadline per frame before COMMIT
	handoffDialTimeout  = 2 * time.Second
	certWarmTimeout     = 10 * time.Second
	restartDrain        = 1 * time.Second        // old side: wait for the viewers' 1012 close handshakes
	oldGoneWait         = 500 * time.Millisecond // new side: how long a dying old daemon gets to be gone
)

// maxFrozen is the worst-case 503 window (IN-01 it2: restartDrain included)
// and bounds the SIGTERM guard's wait for a handoff's outcome.
func maxFrozen() time.Duration {
	return writeDrainTimeout + freezeTimeout + stateSendTimeout + readyTimeout + committedWatchdog + restartDrain
}

// Outcome codes of runTakeover; 09-05 maps them to the couldn't-move reason.
const (
	takeoverCommitted = 0
	takeoverFailed    = 1 // anything else
	takeoverBusy      = 3
	takeoverRefused   = 4 // uid, protocol version, older release, or a pid the kernel does not confirm
	takeoverImport    = 5 // the state could not be loaded (incl. "share certificate unavailable")
	takeoverTimedOut  = 6 // timed out, aborted, or the old daemon went away alive before COMMIT
)

// moveFailedReason is RestartQuestion.MoveFailed for an outcome code.
func moveFailedReason(code int) string {
	switch code {
	case takeoverBusy:
		return "another update is already moving them"
	case takeoverRefused:
		return "the running version refused"
	case takeoverImport:
		return "the new version could not load them"
	case takeoverTimedOut:
		return "the handoff timed out"
	}
	return "the new version exited"
}

// Seams. In a test both daemons share one process, so the peer is the test
// process itself: tests replace these. handoffFault is nil in release builds
// (handoff_vmpreview.go sets it in VM builds).
var (
	// handoffFault is called at every step on both sides with side "old" or
	// "new". An error wrapping errHandoffDie makes that side die there (exit
	// seam, connection closed); any other error makes it fail (roll back or
	// abort). After COMMIT every error on the new side is a death.
	handoffFault func(side, step string) error

	peerUID = fdpass.PeerUID
	peerPID = fdpass.PeerPID
	// New side: the old daemon's pid as the kernel reports it for the handoff
	// connection (LOCAL_PEERPID; the dialing side sees the accepting process).
	// Tests replace it; never reuse peerPID, which is the old side's lookup.
	takeoverPeerPID = fdpass.PeerPID
	watchPeerExit   = pty.WatchExit // old side: the takeover's exit
	watchOldExit    = pty.WatchExit // new side: the old daemon's exit (holdInputUntilOldExits)
	pidAlive        = func(pid int) bool {
		err := unix.Kill(pid, 0)
		return err == nil || errors.Is(err, unix.EPERM)
	}
	handoffExit  = os.Exit // old side, after COMMITTED (0) or a fault death
	takeoverExit = os.Exit // new side: watchdog, undeliverable COMMITTED, fault death
)

var errHandoffDie = errors.New("handoff fault: die here")

func handoffStep(side, step string) error {
	if handoffFault == nil {
		return nil
	}
	return handoffFault(side, step)
}

// Frame types. A frame is 1 type byte, a 4-byte big-endian length and a gob
// payload; an fd frame is a fixed 9-byte header (type, index, count) sent
// with SCM_RIGHTS, so fds never share a message with a large payload.
const (
	frameHello byte = iota + 1
	frameHelloOK
	frameRefuse
	frameGlobal
	frameSession
	frameReady
	frameAbort
	frameCommit
	frameCommitted
	frameFDs
)

const (
	maxFrame           = 64 << 20
	maxSessionFrame    = relay.HandoffStateMaxBytes // its 1 MiB slack also covers the session's metadata
	maxHandoffSessions = 256
	maxHandoffText     = 4 << 10 // session name, workDir
	// IN-01: HELLO is read before the peer is checked and READY carries only
	// a digest per session, so neither gets maxFrame.
	maxHelloFrame = 4 << 10
	maxReadyFrame = maxHandoffSessions * (maxHandoffText + 128)
)

type hsHello struct {
	Version int
	Build   string
	PID     int
}

type hsHelloOK struct{ PID int }

type hsRefuse struct {
	Code   int
	Reason string
}

type hsGlobal struct {
	Sessions      int
	ShareListener bool
	Shares        map[string]hsShare // session id -> record
}

type hsShare struct{ GrantID, Token string }

type hsSession struct {
	ID, Name, WorkDir, CLI string
	CreatedAt              time.Time
	Pid                    int
	Hub                    relay.HubState
}

type hsReady struct{ Digests map[string][32]byte }

type hsAbort struct{ Reason string }

// abortedError is an ABORT received in place of the expected frame.
type abortedError struct{ reason string }

func (e *abortedError) Error() string { return "aborted by the other side: " + e.reason }

// refusedError is a REFUSE received in place of HELLO-OK.
type refusedError struct {
	code   int
	reason string
}

func (e *refusedError) Error() string { return "refused: " + e.reason }

// frameError is a malformed, unexpected or over-cap frame: the peer's state
// is unusable, as opposed to a connection failure.
type frameError struct{ msg string }

func (e *frameError) Error() string { return "handoff: " + e.msg }

func writeFrame(w io.Writer, typ byte, v any) error {
	buf := bytes.NewBuffer([]byte{typ, 0, 0, 0, 0})
	if v != nil {
		if err := gob.NewEncoder(buf).Encode(v); err != nil {
			return err
		}
	}
	b := buf.Bytes()
	limit := maxFrame
	if typ == frameSession { // the receiver's cap, checked here so the log names the session (WR-02)
		limit = maxSessionFrame
	}
	if len(b)-5 > limit {
		return fmt.Errorf("handoff: frame type %d of %d bytes over the %d byte cap", typ, len(b)-5, limit)
	}
	binary.BigEndian.PutUint32(b[1:5], uint32(len(b)-5))
	_, err := w.Write(b)
	return err
}

// readFrame reads the next frame, which must be of type want, into v (nil:
// no payload). ABORT and REFUSE come back as *abortedError and
// *refusedError, a bad frame as *frameError, a connection failure as is. The
// length is checked against limit before anything is allocated.
func readFrame(r io.Reader, want byte, v any, limit int) error {
	var h [5]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(h[1:])
	if int64(n) > int64(limit) {
		return &frameError{fmt.Sprintf("frame type %d of %d bytes over the %d byte cap", h[0], n, limit)}
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(r, p); err != nil {
		return err
	}
	switch {
	case h[0] == frameAbort && want != frameAbort:
		var ab hsAbort
		_ = gob.NewDecoder(bytes.NewReader(p)).Decode(&ab)
		return &abortedError{ab.Reason}
	case h[0] == frameRefuse && want == frameHelloOK:
		var rf hsRefuse
		_ = gob.NewDecoder(bytes.NewReader(p)).Decode(&rf)
		return &refusedError{rf.Code, rf.Reason}
	case h[0] != want:
		return &frameError{fmt.Sprintf("got frame type %d, want %d", h[0], want)}
	case v == nil:
		return nil
	}
	if err := gob.NewDecoder(bytes.NewReader(p)).Decode(v); err != nil {
		return &frameError{fmt.Sprintf("frame type %d: %v", want, err)}
	}
	return nil
}

func sendFDs(c *net.UnixConn, idx int, conns ...syscall.Conn) error {
	var h [9]byte
	h[0] = frameFDs
	binary.BigEndian.PutUint32(h[1:5], uint32(idx))
	binary.BigEndian.PutUint32(h[5:9], uint32(len(conns)))
	return fdpass.Send(c, h[:], conns...)
}

// recvFDs reads fd frame idx carrying exactly n fds. Nothing read at all is
// a connection failure (io.EOF); a wrong header or count is a *frameError.
// On error no received fd stays open (fdpass.Recv closes them).
func recvFDs(c *net.UnixConn, idx, n int) ([]*os.File, error) {
	var h [9]byte
	got, files, err := fdpass.Recv(c, h[:], n)
	switch {
	case got == 0 && err != nil:
		return nil, fmt.Errorf("%w (%v)", io.EOF, err)
	case err != nil:
		return nil, &frameError{err.Error()}
	case got != len(h) || h[0] != frameFDs || binary.BigEndian.Uint32(h[1:5]) != uint32(idx) || binary.BigEndian.Uint32(h[5:9]) != uint32(n):
		closeFiles(files)
		return nil, &frameError{fmt.Sprintf("malformed fd frame %d", idx)}
	}
	return files, nil
}

func closeFiles(files []*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}

// releaseRE matches the versions olderRelease compares (v1.2.0, v1.2.0-vmA).
var releaseRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.]+))?$`)

// olderRelease reports whether release a is older than b: numeric core
// first, then a version without a suffix is newer than one with, then
// suffixes compare as strings. Unparseable versions ("dev") are never older.
// A downgrade handoff is unsupported (CONTEXT deferred), so an older GUI
// relaunched after an upgrade cannot pull the sessions back; it gets the
// DEBT-08 question instead.
func olderRelease(a, b string) bool {
	ma, mb := releaseRE.FindStringSubmatch(a), releaseRE.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return x < y
		}
	}
	switch {
	case ma[4] == mb[4], ma[4] == "":
		return false
	case mb[4] == "":
		return true
	}
	return ma[4] < mb[4]
}

// ---- old side --------------------------------------------------------------

func (a *API) beginHandoff() bool {
	a.handoffMu.Lock()
	defer a.handoffMu.Unlock()
	if a.handoffDone != nil {
		return false
	}
	a.handoffDone = make(chan struct{})
	return true
}

func (a *API) endHandoff() {
	a.handoffMu.Lock()
	defer a.handoffMu.Unlock()
	close(a.handoffDone)
	a.handoffDone = nil
}

// waitHandoff waits up to max for a running handoff to end and reports
// whether this daemon must exit without cleanup: handed off, or committed
// and still unresolved (the takeover may own the sessions and the socket).
func (a *API) waitHandoff(max time.Duration) bool {
	a.handoffMu.Lock()
	done := a.handoffDone
	a.handoffMu.Unlock()
	if done != nil {
		t := time.NewTimer(max)
		defer t.Stop()
		select {
		case <-done:
		case <-t.C:
		}
	}
	return a.handedOff.Load() || a.committed.Load()
}

// closeListenerCopies closes this daemon's copies of the listeners a
// takeover now serves. The socket path is never unlinked (Pattern 7,
// Pitfall 4): it names the socket the takeover serves.
func (a *API) closeListenerCopies() {
	if ul, ok := a.ln.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
		_ = ul.Close()
		a.ln = nil
	}
	if a.relayLn != nil {
		_ = a.relayLn.Close()
		a.relayLn = nil
	}
	a.mu.Lock()
	ws := a.webServer
	a.webServer = nil
	a.mu.Unlock()
	if ws != nil {
		_ = ws.Stop() // hijacked viewer WebSockets survive Stop and get 1012 next
	}
	// Loopback forwards for remote localhost URLs are not carried by a handoff:
	// the next click asks again and reopens them.
	a.portForwards.CloseAll()
}

// handleHandoff upgrades POST /handoff (unix socket only) to the handoff
// protocol on the hijacked connection.
func (a *API) handleHandoff(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), handoffUpgrade) {
		http.Error(w, "handoff needs Upgrade: "+handoffUpgrade, http.StatusUpgradeRequired)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "handoff needs a hijackable connection", http.StatusInternalServerError)
		return
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		return
	}
	c, ok := conn.(*net.UnixConn)
	if !ok || brw.Reader.Buffered() > 0 { // fds can only cross a unix conn, and nothing may sit in a buffer
		_ = conn.Close()
		return
	}
	defer c.Close()
	if _, err := io.WriteString(c, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: "+handoffUpgrade+"\r\n\r\n"); err != nil {
		return
	}
	a.serveHandoff(c)
}

// oldHandoff is one handoff on the old daemon's side.
type oldHandoff struct {
	a      *API
	c      *net.UnixConn
	pid    int // the takeover's, from LOCAL_PEERPID
	frozen []handoffEntry
	start  time.Time
	gated  bool                // holds a.writeGate (drainWrites) until resume
	sent   map[string][32]byte // session id -> sha256 of the SESSION frame as sent (WR-02 it2)
}

func (a *API) serveHandoff(c *net.UnixConn) {
	start := time.Now()
	_ = c.SetDeadline(start.Add(takeoverStepTimeout))
	var hello hsHello
	if err := readFrame(c, frameHello, &hello, maxHelloFrame); err != nil {
		log.Printf("handoff: no HELLO: %v", err)
		return
	}
	// A refusal freezes nothing and sends no fd.
	refuse := func(code int, why string) {
		log.Printf("handoff: refused a takeover from build %q: %s", hello.Build, why)
		_ = writeFrame(c, frameRefuse, hsRefuse{Code: code, Reason: why})
	}
	if uid, err := peerUID(c); err != nil || uid != uint32(os.Getuid()) {
		refuse(takeoverRefused, "the takeover runs as another user")
		return
	}
	if hello.Version != HandoffVersion {
		refuse(takeoverRefused, fmt.Sprintf("handoff version %d, this daemon speaks %d", hello.Version, HandoffVersion))
		return
	}
	if olderRelease(hello.Build, BuildVersion) {
		refuse(takeoverRefused, "an older release cannot take the sessions")
		return
	}
	pid, err := peerPID(c)
	if err != nil || pid <= 0 {
		refuse(takeoverRefused, "the takeover's pid is unknown")
		return
	}
	if !a.beginHandoff() {
		refuse(takeoverBusy, "another handoff is in progress")
		return
	}
	defer a.endHandoff()
	// B1: watched from HELLO, while this connection holds the pid, so a later
	// reuse of the number cannot fool the watch.
	peerGone := watchPeerExit(pid)
	if err := writeFrame(c, frameHelloOK, hsHelloOK{PID: os.Getpid()}); err != nil {
		log.Printf("handoff: HELLO-OK to pid %d failed: %v", pid, err)
		return
	}
	o := &oldHandoff{a: a, c: c, pid: pid, start: start}
	log.Printf("handoff: takeover pid %d (build %q) accepted", pid, hello.Build)
	if o.fault("after-hello") {
		return
	}

	// FREEZE: the gate first, so no write lands between a hub's last read and
	// its export, and the writes already past it drain (WR-01); then every
	// live hub concurrently.
	a.frozen.Store(true)
	if !a.drainWrites(writeDrainTimeout) {
		o.rollback("freeze", "a write in flight did not finish")
		return
	}
	o.gated = true
	entries := a.engine.handoffEntries()
	errs := make([]error, len(entries))
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Go(func() { errs[i] = e.hub.Freeze() })
	}
	wg.Wait()
	var freezeErr error
	for i, e := range entries {
		switch {
		case errs[i] == nil:
			o.frozen = append(o.frozen, e)
		case errors.Is(errs[i], relay.ErrHubEnded): // its shell exited during the freeze: stopped, not moved
		default:
			freezeErr = errs[i]
		}
	}
	if freezeErr != nil {
		o.rollback("freeze", freezeErr.Error())
		return
	}
	if o.fault("after-freeze") {
		return
	}

	// STATE under one write deadline.
	_ = c.SetDeadline(time.Time{})
	_ = c.SetWriteDeadline(time.Now().Add(stateSendTimeout))
	g, conns, err := o.global()
	if err == nil {
		err = writeFrame(c, frameGlobal, g)
	}
	if err == nil {
		err = sendFDs(c, 0, conns...)
	}
	for i := 0; err == nil && i < len(o.frozen); i++ {
		if i == 1 && o.fault("mid-state-send") {
			return
		}
		err = o.sendSession(i)
	}
	if err != nil {
		o.rollback("state", err.Error())
		return
	}
	if o.fault("after-state-sent") {
		return
	}

	// READY with a digest per session, each equal to the frame sent (WR-02 it2).
	_ = c.SetWriteDeadline(time.Time{})
	_ = c.SetReadDeadline(time.Now().Add(readyTimeout))
	var ready hsReady
	if err := readFrame(c, frameReady, &ready, maxReadyFrame); err != nil {
		o.rollback("ready", err.Error())
		return
	}
	if why := o.checkDigests(ready.Digests); why != "" {
		o.rollback("ready", why)
		return
	}
	if o.fault("after-ready-before-commit") {
		return
	}

	// COMMIT. From here this daemon never resumes reading on a timer, only
	// once the takeover process is gone (B1). Accepted overlap: until
	// COMMITTED both processes accept on the shared listeners; this one
	// answers reads from the frozen state and 503 for writes, and its viewers
	// get 1012 when it leaves, so nothing is lost.
	a.committed.Store(true)
	_ = c.SetDeadline(time.Time{})
	if err := writeFrame(c, frameCommit, nil); err != nil {
		log.Printf("handoff: COMMIT to pid %d failed (%v); waiting for it to exit before resuming", pid, err)
		<-peerGone
		o.resumeAfterCommit()
		return
	}
	if handoffStep("old", "after-commit-before-committed") != nil {
		o.die("after-commit-before-committed")
		return
	}
	got := make(chan error, 1)
	go func() { got <- readFrame(c, frameCommitted, nil, maxFrame) }()
	select {
	case err := <-got:
		if err == nil {
			o.finish(peerGone)
			return
		}
		log.Printf("handoff: no COMMITTED from pid %d (%v); waiting for it to exit before resuming", pid, err)
		<-peerGone
	case <-peerGone:
	}
	o.resumeAfterCommit()
}

// fault runs the fault seam at an old-side step before COMMIT and reports
// whether the handoff ended there.
func (o *oldHandoff) fault(step string) bool {
	err := handoffStep("old", step)
	switch {
	case err == nil:
		return false
	case errors.Is(err, errHandoffDie):
		o.die(step)
	default:
		o.rollback(step, "fault: "+err.Error())
	}
	return true
}

// die is a fault death: the exit seam, and then (only where the seam
// returns, in tests) what the dead process no longer holds.
func (o *oldHandoff) die(step string) {
	log.Printf("handoff: dying at old:%s (fault)", step)
	handoffExit(1)
	o.a.closeListenerCopies()
	_ = o.c.Close()
}

// global collects the GLOBAL frame and its fds: lock, API listener, relay
// listener and, while the share server is needed (a shared session being moved
// or the always-available desktop), the share listener. The wire format is the
// same either way: a desktop-only handoff is ShareListener with no Shares.
func (o *oldHandoff) global() (hsGlobal, []syscall.Conn, error) {
	a := o.a
	ul, okU := a.ln.(*net.UnixListener)
	rl, okR := a.relayLn.(*net.TCPListener)
	if a.lock == nil || !okU || !okR {
		return hsGlobal{}, nil, errors.New("this daemon holds no lock or listeners to hand over")
	}
	conns := []syscall.Conn{a.lock, ul, rl}
	g := hsGlobal{Sessions: len(o.frozen), Shares: map[string]hsShare{}}
	a.mu.RLock()
	for _, e := range o.frozen {
		if rec, ok := a.shares[e.id]; ok {
			g.Shares[e.id] = hsShare{GrantID: rec.grantID, Token: rec.token}
		}
	}
	ws := a.webServer
	needed := a.serverNeededLocked() // a terminal share or always-available: the link must survive
	a.mu.RUnlock()
	if ws != nil && needed {
		if tl, ok := ws.Listener().(*net.TCPListener); ok {
			conns = append(conns, tl)
			g.ShareListener = true
		}
	}
	return g, conns, nil
}

// sendSession exports frozen session i (which reseeds the hub from its own
// export, so a rollback resumes on exactly what was sent) and sends it with
// its master and slave.
func (o *oldHandoff) sendSession(i int) error {
	e := o.frozen[i]
	st, err := e.hub.Export()
	if err != nil {
		return err
	}
	master, slave, pid, err := e.sess.HandoffFiles()
	if err != nil {
		return err
	}
	m := hsSession{ID: e.id, Name: e.name, WorkDir: e.workDir, CLI: e.sess.CLI, CreatedAt: e.sess.CreatedAt, Pid: pid, Hub: st}
	sum, err := writeSessionFrame(o.c, m)
	if err != nil {
		return fmt.Errorf("session %s: %w", relay.DiagID(e.id), err)
	}
	if o.sent == nil {
		o.sent = map[string][32]byte{}
	}
	o.sent[e.id] = sum
	return sendFDs(o.c, i+1, master, slave)
}

// writeSessionFrame and readSessionFrame hash the SESSION frame's bytes as
// they cross the socket (WR-02 it2), so READY catches a changed or reordered
// transfer, while a HubState schema difference gob tolerates between two
// builds still matches: neither side re-encodes its own struct.
func writeSessionFrame(w io.Writer, m hsSession) ([32]byte, error) {
	h := sha256.New()
	err := writeFrame(io.MultiWriter(w, h), frameSession, m)
	return [32]byte(h.Sum(nil)), err
}

func readSessionFrame(r io.Reader, m *hsSession) ([32]byte, error) {
	h := sha256.New()
	err := readFrame(io.TeeReader(r, h), frameSession, m, maxSessionFrame)
	return [32]byte(h.Sum(nil)), err
}

func (o *oldHandoff) checkDigests(d map[string][32]byte) string {
	if len(d) != len(o.frozen) {
		return fmt.Sprintf("READY has %d digests for %d sessions", len(d), len(o.frozen))
	}
	for _, e := range o.frozen {
		if got, ok := d[e.id]; !ok || got != o.sent[e.id] {
			return "digest mismatch for session " + relay.DiagID(e.id)
		}
	}
	return ""
}

// rollback ends a handoff the takeover never committed to. ABORT goes out
// before any Thaw (threat I1): a takeover still waiting never proceeds on a
// snapshot this daemon has since moved past.
func (o *oldHandoff) rollback(step, reason string) {
	_ = o.c.SetWriteDeadline(time.Now().Add(time.Second))
	_ = writeFrame(o.c, frameAbort, hsAbort{Reason: reason})
	_ = handoffStep("old", "abort-written") // trace point for tests; its error means nothing
	o.resume()
	_ = o.c.Close()
	log.Printf("handoff: rolled back at %s after %v: %s (sessions=%d, takeover pid %d)",
		step, time.Since(o.start).Round(time.Millisecond), reason, len(o.frozen), o.pid)
}

// resume thaws every frozen hub (each already holds its own export) and
// lifts the freeze gate.
func (o *oldHandoff) resume() {
	for _, e := range o.frozen {
		e.hub.Thaw()
	}
	o.a.committed.Store(false)
	o.a.frozen.Store(false)
	if o.gated {
		o.gated = false
		o.a.writeGate.Unlock()
	}
}

// resumeAfterCommit runs once the takeover process has exited after COMMIT
// without COMMITTED: it can no longer read, so this daemon takes over again.
func (o *oldHandoff) resumeAfterCommit() {
	_ = o.c.Close()
	o.resume()
	log.Printf("handoff: takeover pid %d exited after COMMIT; serving %d sessions again (output it read before exiting is lost)",
		o.pid, len(o.frozen))
}

// finish runs on COMMITTED: close the listener copies without unlinking,
// tell every viewer 1012 service restart, give the close handshakes up to
// restartDrain, then exit without stopping a session. Never api.Stop,
// Hub.Shutdown, HubManager.Shutdown, KillSession or a context cancel here:
// each would unlink the socket, send 1000 session ended, or end shells.
//
// CR-01: a takeover that has exited since it wrote COMMITTED no longer reads
// the PTYs, and this daemon exiting would leave nobody holding them. While
// the listener copies are still open it serves again instead.
func (o *oldHandoff) finish(peerGone <-chan int) {
	_ = handoffStep("old", "committed-received") // trace point for tests; its error means nothing
	select {
	case <-peerGone:
		log.Printf("handoff: takeover pid %d exited right after COMMITTED", o.pid)
		o.resumeAfterCommit()
		return
	default:
	}
	a := o.a
	a.handedOff.Store(true)
	a.closeListenerCopies()
	// Desktop viewers are not handed over: they get 1012 (the viewer's wake
	// signal) and redial the takeover on the same link. Run beside the terminal
	// viewers' drain, since each close handshake can take seconds.
	desktopDone := make(chan struct{})
	go func() {
		defer close(desktopDone)
		a.closeDesktopForHandoff()
	}()
	m := a.engine.Manager()
	dones := []<-chan struct{}{desktopDone}
	for _, id := range m.SessionIDs() {
		if h, ok := m.Get(id); ok {
			dones = append(dones, h.RestartViewers())
		}
	}
	// One deadline for the whole tail: the viewers' close handshakes, then the
	// input they queued reaching the PTYs. Never longer (maxFrozen counts it).
	deadline := time.Now().Add(restartDrain)
	drain := time.NewTimer(restartDrain)
	defer drain.Stop()
wait:
	for _, d := range dones {
		select {
		case <-d:
		case <-drain.C:
			break wait
		}
	}
	// shortcut: an upgrade during a slow multi-MB paste can lose the bytes still
	// queued here when the old daemon exits, after a pong told the client they
	// were queued, so no notice shows; the loss is logged. Handing the queue to
	// the takeover would close it. Nothing reorders: the takeover writes no
	// input until this process is gone (holdInputUntilOldExits, WR-02).
	waitHubsInputIdle(m, deadline)
	if a.lock != nil {
		_ = a.lock.Close() // the takeover holds its own copy
	}
	log.Printf("handoff: committed: %d sessions moved to pid %d in %v; exiting",
		len(o.frozen), o.pid, time.Since(o.start).Round(time.Millisecond))
	handoffExit(0)
}

// closeDesktopForHandoff ends this daemon's desktop for good after COMMITTED:
// every desktop viewer is closed with 1012, capture stops (which clears
// captureActive, so the takeover's crash breaker counts no abnormal exit) and
// the media socket is released for the takeover to bind. The Manager is taken
// under a.mu and closed outside it: Close waits on each viewer's close
// handshake. Never on a rollback path.
func (a *API) closeDesktopForHandoff() {
	a.mu.Lock()
	mgr := a.desktopMgr
	a.desktopMgr, a.desktopBuiltIP, a.desktopBuiltDomain = nil, "", ""
	a.mu.Unlock()
	if mgr != nil {
		// Close clears captureActive first, then sends the same 1012 to every
		// viewer. A CloseAll here would put the slow handshakes ahead of the flag.
		_ = mgr.Close()
	}
}

// desktopWhenOldGone brings the desktop up on this (takeover) daemon the moment
// the old daemon has exited and so freed the media port, instead of waiting for
// the poller's next 10 s tick: a viewer redials as soon as it gets 1012.
func (a *API) desktopWhenOldGone(gone <-chan int) {
	select {
	case <-gone:
		a.desktopTick(a.desktopCtx)
	case <-a.desktopCtx.Done():
	}
}

// waitHubsInputIdle waits until every hub's queued viewer input reached its
// PTY or the deadline passes, logs each session still holding some, and
// returns those sessions' queued bytes (INPT-01, Research Pitfall 7).
func waitHubsInputIdle(m *relay.HubManager, deadline time.Time) map[string]int {
	var lost map[string]int
	for _, id := range m.SessionIDs() {
		h, ok := m.Get(id)
		if !ok {
			continue
		}
		if idle, queued := h.WaitInputIdle(time.Until(deadline)); !idle {
			log.Printf("handoff: input still queued at exit session=%s bytes=%d", relay.DiagID(id), queued)
			if lost == nil {
				lost = make(map[string]int)
			}
			lost[id] = queued
		}
	}
	return lost
}

// ---- new side --------------------------------------------------------------

// takeoverDeps is what runTakeover needs: the socket of the running daemon
// and a fresh API over a fresh engine that serves nothing yet.
type takeoverDeps struct {
	socketPath string
	lockPath   string // daemon.lock: the received lock fd must be it (WR-05)
	api        *API
	build      string // the release HELLO announces (BuildVersion)
}

type importedSession struct {
	id     string
	sess   *pty.Session
	hub    *relay.Hub
	digest [32]byte // sha256 of the SESSION frame as received (WR-02 it2)
}

// takeover is runTakeover's state: everything received, so that a failure
// before COMMIT can give it all back.
type takeover struct {
	a          *API
	c          *net.UnixConn
	oldPID     int
	socketPath string

	health webserver.TailscaleHealth
	certOK bool

	global               hsGlobal
	lock                 *os.File
	ln, relayLn, shareLn net.Listener
	sessions             []importedSession
	ws                   *webserver.WebServer
	finalized            bool

	dead     atomic.Bool
	exitOnce sync.Once
}

// runTakeover takes the sessions of the daemon on d.socketPath into d.api
// and returns an outcome code (moveFailedReason). On 0 d.api serves the
// received socket and relay listeners and every session; the caller then
// runs the serve loop (serveDaemon). On any other code everything received
// was closed and the old daemon keeps serving.
func runTakeover(ctx context.Context, d takeoverDeps) int {
	t := &takeover{a: d.api, socketPath: d.socketPath}
	// W4: before HELLO, so outside the freeze and the old side's READY timer.
	t.warmCert(ctx)

	if err := t.dial(ctx, d.socketPath); err != nil {
		log.Printf("handoff: cannot open a handoff with the running daemon: %v", err)
		t.cleanup()
		return takeoverFailed
	}
	if err := writeFrame(t.c, frameHello, hsHello{Version: HandoffVersion, Build: d.build, PID: os.Getpid()}); err != nil {
		t.cleanup()
		return takeoverFailed
	}
	var ok hsHelloOK
	if err := t.read(frameHelloOK, &ok, maxFrame); err != nil {
		var rf *refusedError
		if errors.As(err, &rf) {
			log.Printf("handoff: the running daemon refused: %s", rf.reason)
			t.cleanup()
			if rf.code == takeoverBusy {
				return takeoverBusy
			}
			return takeoverRefused
		}
		return t.readFailed(err)
	}
	// The pid in HELLO-OK is written by the peer; only the kernel's answer is
	// trusted. A failed lookup refuses too, never falls back to HELLO-OK.
	kernelPID, perr := takeoverPeerPID(t.c)
	if perr != nil || kernelPID != ok.PID {
		return t.fail(takeoverRefused, fmt.Sprintf("HELLO-OK pid %d does not match the socket peer (%d, %v)", ok.PID, kernelPID, perr))
	}
	t.oldPID = kernelPID
	if code, done := t.preStep("after-hello"); done {
		return code
	}

	if err := t.read(frameGlobal, &t.global, maxFrame); err != nil {
		return t.readFailed(err)
	}
	if n := t.global.Sessions; n < 0 || n > maxHandoffSessions {
		return t.fail(takeoverImport, fmt.Sprintf("session count %d out of range", n))
	}
	nfds := 3
	if t.global.ShareListener {
		nfds++
	}
	_ = t.c.SetReadDeadline(time.Now().Add(takeoverStepTimeout))
	files, err := recvFDs(t.c, 0, nfds)
	if err != nil {
		return t.readFailed(err)
	}
	if err := t.takeGlobalFiles(files, d.socketPath, d.lockPath); err != nil {
		return t.fail(takeoverImport, err.Error())
	}
	if code, done := t.preStep("after-global"); done {
		return code
	}

	for i := 0; i < t.global.Sessions; i++ {
		var m hsSession
		_ = t.c.SetReadDeadline(time.Now().Add(takeoverStepTimeout))
		digest, err := readSessionFrame(t.c, &m)
		if err != nil {
			return t.readFailed(err)
		}
		_ = t.c.SetReadDeadline(time.Now().Add(takeoverStepTimeout))
		files, err := recvFDs(t.c, i+1, 2)
		if err != nil {
			return t.readFailed(err)
		}
		if why := t.invalid(m); why != "" {
			closeFiles(files)
			return t.fail(takeoverImport, why)
		}
		meta := pty.AdoptMeta{ID: m.ID, CLI: m.CLI, CreatedAt: m.CreatedAt, Pid: m.Pid}
		sess, hub, err := t.a.engine.importSession(meta, m.Name, m.WorkDir, m.Hub, files[0], files[1])
		if err != nil {
			return t.fail(takeoverImport, err.Error())
		}
		t.sessions = append(t.sessions, importedSession{id: m.ID, sess: sess, hub: hub, digest: digest})
	}
	if code, done := t.preStep("after-state"); done {
		return code
	}
	if err := t.finalize(); err != nil {
		return t.fail(takeoverImport, err.Error())
	}
	if code, done := t.preStep("before-ready"); done {
		return code
	}

	digests := make(map[string][32]byte, len(t.sessions))
	for _, s := range t.sessions {
		digests[s.id] = s.digest
	}
	if err := writeFrame(t.c, frameReady, hsReady{Digests: digests}); err != nil {
		return t.lost(err)
	}
	if err := t.read(frameCommit, nil, maxFrame); err != nil {
		return t.readFailed(err)
	}
	return t.commit(true)
}

// warmCert fetches the share certificate when Tailscale reports a domain, so
// the share server serves from tailscaled's cache after COMMIT. A failure is
// fatal only if share records arrive (finalize).
func (t *takeover) warmCert(ctx context.Context) {
	t.a.mu.RLock()
	health, warm := t.a.shareHealth, t.a.warmCert
	t.a.mu.RUnlock()
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	t.health = health(hctx)
	cancel()
	if !t.health.HasCerts || t.health.Domain == "" {
		return
	}
	start := time.Now()
	wctx, cancel := context.WithTimeout(ctx, certWarmTimeout)
	err := warm(wctx, t.health.Domain)
	cancel()
	t.certOK = err == nil
	log.Printf("handoff: cert warm took %dms (ok=%v)", time.Since(start).Milliseconds(), t.certOK)
}

// dial opens the handoff: POST /handoff with the upgrade header, then the
// 101 read byte by byte, since nothing past it may sit in a buffer (fd
// frames follow on this conn).
func (t *takeover) dial(ctx context.Context, path string) error {
	d := net.Dialer{Timeout: handoffDialTimeout}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return err
	}
	t.c = conn.(*net.UnixConn)
	_ = t.c.SetDeadline(time.Now().Add(takeoverStepTimeout))
	req := "POST /handoff HTTP/1.1\r\nHost: daemon\r\nUpgrade: " + handoffUpgrade + "\r\nConnection: Upgrade\r\nContent-Length: 0\r\n\r\n"
	if _, err := io.WriteString(t.c, req); err != nil {
		return err
	}
	var resp []byte
	b := make([]byte, 1)
	for !bytes.HasSuffix(resp, []byte("\r\n\r\n")) {
		if len(resp) > 4096 {
			return errors.New("oversized upgrade response")
		}
		if _, err := t.c.Read(b); err != nil {
			return err
		}
		resp = append(resp, b[0])
	}
	if !bytes.HasPrefix(resp, []byte("HTTP/1.1 101 ")) {
		status, _, _ := bytes.Cut(resp, []byte("\r\n"))
		return fmt.Errorf("the handoff route answered %q", status)
	}
	return nil
}

// read reads one frame with the per-step deadline.
func (t *takeover) read(want byte, v any, limit int) error {
	_ = t.c.SetReadDeadline(time.Now().Add(takeoverStepTimeout))
	return readFrame(t.c, want, v, limit)
}

// readFailed sorts a failed read: an ABORT ends at once without reading, a
// bad frame aborts the import, a connection failure goes to lost.
func (t *takeover) readFailed(err error) int {
	var ab *abortedError
	var fe *frameError
	switch {
	case errors.As(err, &ab):
		log.Printf("handoff: the running daemon aborted: %s", ab.reason)
		t.cleanup()
		return takeoverTimedOut
	case errors.As(err, &fe):
		return t.fail(takeoverImport, fe.Error())
	}
	return t.lost(err)
}

// takeGlobalFiles keeps the lock and rebuilds the listeners, and refuses
// them unless they are what this daemon would hold (checkGlobalFiles). On
// error every file not yet taken is closed (the rest closes in cleanup).
func (t *takeover) takeGlobalFiles(files []*os.File, socketPath, lockPath string) error {
	t.lock = files[0]
	rest := files[1:]
	defer func() { closeFiles(rest) }()
	take := func(network string) (net.Listener, error) {
		f := rest[0]
		rest = rest[1:]
		ln, err := net.FileListener(f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		if ln.Addr().Network() != network {
			_ = ln.Close()
			return nil, fmt.Errorf("received a %s listener, want %s", ln.Addr().Network(), network)
		}
		return ln, nil
	}
	var err error
	if t.ln, err = take("unix"); err != nil {
		return err
	}
	t.ln.(*net.UnixListener).SetUnlinkOnClose(false) // the path is shared with the old daemon
	if t.relayLn, err = take("tcp"); err != nil {
		return err
	}
	if len(rest) > 0 {
		if t.shareLn, err = take("tcp"); err != nil {
			return err
		}
	}
	// The share listener is checked against the share server's own address
	// in finalize (WebServer.CheckListener).
	return checkGlobalFiles(t.lock, lockPath, t.ln, t.relayLn, socketPath)
}

// checkGlobalFiles refuses received fds that are not what this daemon holds
// itself (WR-05): the lock must be daemon.lock, the API listener the daemon
// socket, and the relay, which serves unauthenticated full-write shells,
// loopback only.
func checkGlobalFiles(lock *os.File, lockPath string, ln, relayLn net.Listener, socketPath string) error {
	got, err := lock.Stat()
	want, wantErr := os.Stat(lockPath)
	if err != nil || wantErr != nil || !os.SameFile(got, want) {
		return fmt.Errorf("the received lock is not %s", lockPath)
	}
	if ua, ok := ln.Addr().(*net.UnixAddr); !ok || ua.Name != socketPath {
		return fmt.Errorf("the received API listener is %v, not the daemon socket", ln.Addr())
	}
	if ta, ok := relayLn.Addr().(*net.TCPAddr); !ok || !ta.IP.IsLoopback() {
		return fmt.Errorf("the received relay listener is on %v, not loopback", relayLn.Addr())
	}
	return nil
}

// invalid validates a SESSION frame's metadata (NewHubFromState validates
// the hub state).
func (t *takeover) invalid(m hsSession) string {
	switch {
	case m.ID == "" || m.Hub.SessionID != m.ID:
		return "session frame with a bad id"
	case t.imported(m.ID):
		return "duplicate session " + relay.DiagID(m.ID)
	case m.Pid <= 1: // IN-02: killAdopted signals -pid, and kill(-1) reaches every process of the user
		return "session " + relay.DiagID(m.ID) + " without a valid pid"
	case len(m.Name) > maxHandoffText || len(m.WorkDir) > maxHandoffText || len(m.CLI) > maxHandoffText:
		return "session " + relay.DiagID(m.ID) + " metadata over the cap"
	}
	return ""
}

func (t *takeover) imported(id string) bool {
	for _, s := range t.sessions {
		if s.id == id {
			return true
		}
	}
	return false
}

// finalize reloads what lives on disk (settings, the capability key: a
// settings PATCH was gated during the freeze, so the disk is current) and
// builds, without starting, the share server for the share records of the
// sessions that arrived. It never fetches a certificate: warmCert did.
func (t *takeover) finalize() error {
	if t.finalized {
		return nil
	}
	t.finalized = true
	a := t.a
	a.engine.loadSettingsFromDisk(a.engine.configDir)
	if err := a.BootstrapCapabilityState(); err != nil {
		return err
	}
	t.loadDesktopConfig()
	shares := map[string]shareRecord{}
	for id, s := range t.global.Shares {
		if t.imported(id) {
			shares[id] = shareRecord{grantID: s.GrantID, token: s.Token}
		}
	}
	// With no terminal share the only reason for a server is always-available,
	// which never fails a handoff: the sessions matter more, and the poller
	// starts the server once the old daemon has let go of the port.
	desktopOnly := len(shares) == 0
	if desktopOnly {
		a.mu.RLock()
		on := a.desktopCfg.AlwaysAvailable
		a.mu.RUnlock()
		if !on {
			t.dropShares()
			return nil
		}
		if t.shareLn == nil {
			log.Printf("handoff: always-available server starts after the old daemon exits")
			t.dropShares()
			return nil
		}
	}
	switch {
	case t.shareLn == nil:
		return errors.New("share records came without the share listener")
	case !t.certOK || t.health.IP == "":
		if desktopOnly {
			log.Printf("handoff: share certificate unavailable; always-available server starts after the old daemon exits")
			t.dropShares()
			return nil
		}
		return errors.New("share certificate unavailable")
	}
	a.mu.RLock()
	newServer := a.newShareServer
	a.mu.RUnlock()
	ws, err := newServer(t.health, a.engine.Manager())
	if err == nil {
		err = ws.CheckListener(t.shareLn) // WR-05: before COMMIT, so the old daemon keeps serving
	}
	if err != nil {
		if desktopOnly {
			log.Printf("handoff: share server unusable (%v); always-available server starts after the old daemon exits", err)
			t.dropShares()
			return nil
		}
		return err
	}
	// wireShareServer denies a.relayPort, but commit serves the relay (and sets
	// it) only later: take it from the received listener now (CR-01).
	a.relayPort = t.relayLn.Addr().(*net.TCPAddr).Port
	a.wireShareServer(ws, t.health)
	a.mu.Lock()
	a.shares = shares
	a.mu.Unlock()
	t.ws = ws
	return nil
}

// loadDesktopConfig reads desktop.json afresh: a PUT /desktop was gated during
// the freeze, so the disk is current. An unusable file reads as off with the
// reason kept for GET /desktop, as NewAPI does.
func (t *takeover) loadDesktopConfig() {
	a := t.a
	if a.engine.configDir == "" {
		return
	}
	cfg, err := desktop.LoadConfig(a.engine.configDir)
	if err != nil {
		log.Printf("handoff: desktop.json is unusable (%v); the desktop stays off", err)
		cfg = desktop.Config{}
	}
	a.mu.Lock()
	a.desktopCfg, a.desktopErr = cfg, err
	a.mu.Unlock()
}

// dropShares gives up the share listener and any records.
func (t *takeover) dropShares() {
	if t.shareLn != nil {
		_ = t.shareLn.Close()
		t.shareLn = nil
	}
	t.ws = nil
	t.a.mu.Lock()
	t.a.shares = map[string]shareRecord{}
	t.a.mu.Unlock()
}

// preStep runs the fault seam at a new-side step before COMMIT.
func (t *takeover) preStep(step string) (int, bool) {
	err := handoffStep("new", step)
	switch {
	case err == nil:
		return 0, false
	case errors.Is(err, errHandoffDie):
		t.die(1)
		t.cleanup()
		return takeoverFailed, true
	}
	return t.fail(takeoverFailed, "fault at new:"+step+": "+err.Error()), true
}

// fail aborts before COMMIT: ABORT to the old daemon, everything given back.
func (t *takeover) fail(code int, reason string) int {
	log.Printf("handoff: takeover aborted (code %d): %s", code, reason)
	_ = t.c.SetWriteDeadline(time.Now().Add(time.Second))
	_ = writeFrame(t.c, frameAbort, hsAbort{Reason: reason})
	t.cleanup()
	return code
}

// lost handles the connection failing before COMMIT. An ABORT already
// received wins (threat I1). Otherwise, with the old daemon gone nobody else
// can read the PTYs, so it proceeds as committed with every session that
// fully arrived (Pattern 8 extra rule); with it alive, the old daemon
// resumes by itself and everything is given back.
func (t *takeover) lost(cause error) int {
	_ = t.c.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if readFrame(t.c, frameAbort, &hsAbort{}, maxFrame) == nil {
		log.Printf("handoff: the running daemon aborted")
		t.cleanup()
		return takeoverTimedOut
	}
	if t.ln == nil || !t.oldGone() {
		log.Printf("handoff: lost the running daemon before COMMIT (%v); it keeps the sessions", cause)
		t.cleanup()
		return takeoverTimedOut
	}
	if n := t.global.Sessions - len(t.sessions); n > 0 {
		log.Printf("handoff: old daemon pid %d died mid-state: %d of %d sessions were lost with it", t.oldPID, n, t.global.Sessions)
	}
	log.Printf("handoff: old daemon pid %d is gone (%v); proceeding as committed with %d sessions", t.oldPID, cause, len(t.sessions))
	if err := t.finalize(); err != nil {
		log.Printf("handoff: %v; the sessions move without their shares", err)
		t.dropShares()
	}
	return t.commit(false)
}

// oldGone polls the old daemon's pid for up to oldGoneWait.
func (t *takeover) oldGone() bool {
	if t.oldPID <= 0 {
		return false
	}
	deadline := time.Now().Add(oldGoneWait)
	for pidAlive(t.oldPID) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// commit serves everything received and, when the old daemon is still
// there, confirms with COMMITTED under the watchdog.
func (t *takeover) commit(withCommitted bool) int {
	a := t.a
	// CR-01: the watchdog and the COMMITTED write share one state. The
	// watchdog exits only while the write has not started; once it has, the
	// write's outcome decides, so a delivered COMMITTED is never followed by an
	// exit (the old daemon exits on it).
	const (
		cPending int32 = iota
		cWriting
		cDead
	)
	var state atomic.Int32
	var wd *time.Timer
	if withCommitted {
		wd = time.AfterFunc(committedWatchdog, func() {
			if state.CompareAndSwap(cPending, cDead) {
				log.Printf("handoff: COMMITTED not written within %v of COMMIT", committedWatchdog)
				t.die(1)
			}
			_ = t.c.Close() // a write still in flight fails and goes to the oldGone check
		})
		defer wd.Stop()
	}
	if t.postStep("after-commit-before-serve") {
		t.cleanup()
		return takeoverFailed
	}
	if withCommitted {
		t.holdInputUntilOldExits() // before any viewer can reach these hubs
	}
	a.lock = t.lock
	a.serveOn(t.ln, t.relayLn)
	if t.ws != nil {
		if err := t.ws.StartOn(t.shareLn); err != nil {
			log.Printf("handoff: the share server did not start (%v); shares are off", err)
			t.dropShares()
		} else {
			a.installShareServer(t.ws)
		}
	}
	onExit := a.sessionOnExit()
	for _, s := range t.sessions {
		s.hub.Start()
		a.engine.watchSessionExit(s.id, s.sess, s.hub, onExit)
	}
	if withCommitted && t.oldPID > 0 {
		go a.desktopWhenOldGone(watchOldExit(t.oldPID))
	}
	if t.postStep("after-serve-before-committed") {
		t.cleanup()
		return takeoverFailed
	}
	if withCommitted {
		if !state.CompareAndSwap(cPending, cWriting) { // the watchdog fired first
			t.cleanup()
			return takeoverFailed
		}
		err := handoffStep("new", "committed-write-fails")
		if err == nil {
			err = writeFrame(t.c, frameCommitted, nil)
		}
		_ = handoffStep("new", "committed-written") // trace point for tests; its error means nothing
		wd.Stop()
		if err != nil {
			// A living old daemon waits for this process to exit before it
			// resumes, so two readers never coexist. The poll lets an old
			// daemon that is itself dying (why a write fails) be gone first.
			if !t.oldGone() {
				log.Printf("handoff: COMMITTED not delivered (%v) and the old daemon pid %d is alive; exiting so it resumes", err, t.oldPID)
				t.die(1)
				t.cleanup()
				return takeoverFailed
			}
			log.Printf("handoff: COMMITTED not delivered (%v) but the old daemon is gone; keeping the sessions", err)
		}
	}
	_ = t.c.Close()
	tightenSocketMode(t.socketPath)
	log.Printf("handoff: committed: serving %d sessions handed over by pid %d", len(t.sessions), t.oldPID)
	return takeoverCommitted
}

// holdInputUntilOldExits keeps the imported hubs' input writers from writing
// until the old daemon process is gone (WR-02). After COMMITTED its finish
// still writes the input its own viewers queued to the same PTYs, for up to
// restartDrain, so a write from here could splice into that backlog. Frames
// queue here meanwhile (nothing is dropped), and the old daemon's input
// all lands before any of this side's. Draining the old queue before COMMIT
// instead would not close it: viewers keep typing into the old daemon until
// its 1012, so it would have to hold or drop those bytes, and they would die
// with it. maxFrozen bounds the hold if the exit is never seen.
func (t *takeover) holdInputUntilOldExits() {
	if t.oldPID <= 0 {
		return
	}
	hubs := make([]*relay.Hub, len(t.sessions))
	for i, s := range t.sessions {
		hubs[i] = s.hub
		s.hub.HoldInput()
	}
	gone, limit, pid := watchOldExit(t.oldPID), maxFrozen(), t.oldPID // read here: tests change the timeouts
	go func() {
		timer := time.NewTimer(limit)
		defer timer.Stop()
		select {
		case <-gone:
		case <-timer.C:
			log.Printf("handoff: old daemon pid %d still alive %v after COMMIT; releasing input", pid, limit)
		}
		for _, h := range hubs {
			h.ReleaseInput()
		}
	}()
}

// postStep runs the fault seam at a new-side step after COMMIT, where any
// error is a death, and reports whether this process is dead.
func (t *takeover) postStep(step string) bool {
	if handoffStep("new", step) != nil {
		t.die(1)
	}
	return t.dead.Load()
}

// die exits the takeover process (watchdog, undeliverable COMMITTED, fault).
func (t *takeover) die(code int) {
	t.exitOnce.Do(func() {
		t.dead.Store(true)
		log.Printf("handoff: takeover exiting with code %d", code)
		takeoverExit(code)
	})
}

// cleanup closes everything received without signalling anything: the
// listeners (the socket is never unlinked), the lock, each adopted session's
// files and the connection. Before COMMIT the old daemon holds its own
// copies, so nothing is lost. After a death it is what the exit would do,
// reached only where the exit seam returns (tests).
func (t *takeover) cleanup() {
	if t.ws != nil {
		_ = t.ws.Stop()
	}
	for _, l := range []net.Listener{t.ln, t.relayLn, t.shareLn} {
		if l != nil {
			_ = l.Close()
		}
	}
	for _, s := range t.sessions {
		closeSessionFiles(s.sess)
	}
	if t.lock != nil {
		_ = t.lock.Close()
	}
	if t.c != nil {
		_ = t.c.Close()
	}
}
