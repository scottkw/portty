package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/fdpass"
	"github.com/scottkw/portty/internal/relay"
)

// Two daemons in one test process (09-04). The old daemon serves a temp
// socket under the TestMain HOME; each takeover is a fresh engine and API
// calling runTakeover. fds sent over the socket arrive as duplicates in the
// same process, which is what the kernel does across processes. The exit
// seams record instead of exiting; the peer-exit watch is a channel the test
// closes once it declares the takeover dead.

// tickScript prints three HIST- lines, then T1, T2, ... every 20 ms; alt
// switches into the alt screen first.
func tickScript(alt bool) string {
	pre := ""
	if alt {
		pre = `printf '\033[?1049h'; `
	}
	return pre + `for j in 1 2 3; do echo HIST-$j; done; i=0; while :; do i=$((i+1)); echo T$i; sleep 0.02; done`
}

var tickRE = regexp.MustCompile(`T(\d+)\r\n`)

func tickNums(b []byte) []int {
	var out []int
	for _, m := range tickRE.FindAllSubmatch(b, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		out = append(out, n)
	}
	return out
}

// ringTicks reads the ticks in a hub's ring (frame type bytes removed).
func ringTicks(a *API, id string) []int {
	h, ok := a.engine.Manager().Get(id)
	if !ok {
		return nil
	}
	return tickNums(bytes.ReplaceAll(h.ScrollbackSnapshot(), []byte{relay.MsgOutput}, nil))
}

func lastTick(ns []int) int {
	if len(ns) == 0 {
		return 0
	}
	return ns[len(ns)-1]
}

// union merges tick lists (sorted, without repeats).
func union(lists ...[]int) []int {
	var all []int
	for _, l := range lists {
		all = append(all, l...)
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// checkContiguous fails on a missing or repeated tick.
func checkContiguous(t *testing.T, what string, ns []int) {
	t.Helper()
	if len(ns) == 0 {
		t.Errorf("%s: no ticks", what)
		return
	}
	for i := 1; i < len(ns); i++ {
		if ns[i] != ns[i-1]+1 {
			t.Errorf("%s: tick %d follows %d (ticks %v..%v)", what, ns[i], ns[i-1], ns[0], ns[len(ns)-1])
			return
		}
	}
}

type handoffScene struct {
	t        *testing.T
	cfg      string
	socket   string
	lockPath string
	old      *API
	client   *DaemonClient
	ids      []string
	pids     []int

	peerExit    chan int
	peerClosed  sync.Once
	holdPeer    bool // a non-zero takeover does not close peerExit by itself
	newExits    chan int
	oldExits    chan int
	oldGone     chan int // closed at the old daemon's first exit (watchOldExit)
	oldGoneOnce sync.Once
	oldAlive    atomic.Bool // what pidAlive reports for the old daemon
	lockFree    atomic.Bool

	mu        sync.Mutex
	fault     func(side, step string) error
	warm      func(context.Context, string) error
	events    []string
	times     map[string]time.Time
	takeovers []*API
	extraPids []int
	restore   func()
}

// newHandoffScene starts the old daemon (relay, socket, lock, share seams)
// with one session per script.
func newHandoffScene(t *testing.T, scripts ...string) *handoffScene {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	s := &handoffScene{
		t: t, cfg: t.TempDir(),
		peerExit: make(chan int), newExits: make(chan int, 4), oldExits: make(chan int, 4), oldGone: make(chan int),
		times: map[string]time.Time{},
	}
	s.oldAlive.Store(true)
	s.lockPath = filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := lockDaemon(s.lockPath, 0)
	if err != nil {
		t.Fatalf("lockDaemon: %v", err)
	}
	s.installSeams()
	t.Cleanup(s.cleanup)
	s.old = s.newAPI(false)
	if _, err := s.old.StartRelay(); err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	s.socket = shortSocketPath(t, "ho.sock")
	if err := s.old.Start(s.socket); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.old.lock = lock
	s.client = NewDaemonClient(s.socket)
	wd := t.TempDir()
	for i, sc := range scripts {
		s.old.engine.SetSpawnCommandForTest("/bin/sh", []string{"-c", sc})
		id, err := s.client.CreateSession(fmt.Sprintf("s%d", i), wd, 80, 24)
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		sess, _ := s.old.engine.registry.Get(id)
		s.ids = append(s.ids, id)
		s.pids = append(s.pids, sess.Pid())
		waitFor(t, "first ticks", 5*time.Second, func() bool { return len(ringTicks(s.old, id)) >= 3 })
	}
	return s
}

func (s *handoffScene) newAPI(takeover bool) *API {
	e := NewSessionEngine()
	e.configDir = s.cfg
	e.SetSpawnCommandForTest("/bin/sh", []string{"-c", tickScript(false)})
	a := NewAPI(e)
	if err := a.BootstrapCapabilityState(); err != nil {
		s.t.Fatalf("BootstrapCapabilityState: %v", err)
	}
	warm := noWarm
	if takeover {
		warm = s.warmSeam
	}
	a.SetShareSeamsForTest(healthyTailnet, loopbackShareServer, warm)
	return a
}

func (s *handoffScene) warmSeam(ctx context.Context, domain string) error {
	s.event("new:warm")
	s.mu.Lock()
	f := s.warm
	s.mu.Unlock()
	if f != nil {
		return f(ctx, domain)
	}
	return nil
}

func (s *handoffScene) installSeams() {
	f, uid, pid, watch, watchOld, alive, hx, tx := handoffFault, peerUID, peerPID, watchPeerExit, watchOldExit, pidAlive, handoffExit, takeoverExit
	s.restore = func() {
		handoffFault, peerUID, peerPID, watchPeerExit, watchOldExit, pidAlive, handoffExit, takeoverExit = f, uid, pid, watch, watchOld, alive, hx, tx
	}
	watchOldExit = func(int) <-chan int { return s.oldGone }
	peerPID = func(c *net.UnixConn) (int, error) {
		s.event("old:peer-pid")
		return fdpass.PeerPID(c)
	}
	watchPeerExit = func(int) <-chan int {
		s.event("old:watch-peer")
		return s.peerExit
	}
	pidAlive = func(int) bool { return s.oldAlive.Load() }
	handoffExit = func(code int) {
		s.event("old:exit")
		s.oldGoneOnce.Do(func() { close(s.oldGone) })
		s.oldExits <- code
	}
	takeoverExit = func(code int) {
		s.event("new:exit")
		s.newExits <- code
	}
	handoffFault = func(side, step string) error {
		name := side + ":" + step
		s.event(name)
		if !lockHeld(s.lockPath) {
			s.lockFree.Store(true)
			s.event("LOCK FREE at " + name)
		}
		s.mu.Lock()
		f := s.fault
		s.mu.Unlock()
		if f != nil {
			return f(side, step)
		}
		return nil
	}
}

func (s *handoffScene) setFault(f func(side, step string) error) {
	s.mu.Lock()
	s.fault = f
	s.mu.Unlock()
}

// failAt makes the named step fail with err.
func (s *handoffScene) failAt(name string, err error) {
	s.setFault(func(side, step string) error {
		if side+":"+step == name {
			return err
		}
		return nil
	})
}

func (s *handoffScene) event(e string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if _, ok := s.times[e]; !ok {
		s.times[e] = time.Now()
	}
}

func (s *handoffScene) idx(e string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Index(s.events, e)
}

func (s *handoffScene) count(e string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, x := range s.events {
		if x == e {
			n++
		}
	}
	return n
}

func (s *handoffScene) at(e string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.times[e]
}

func (s *handoffScene) closePeer() { s.peerClosed.Do(func() { close(s.peerExit) }) }

// takeover runs runTakeover with a fresh API. A non-zero outcome means the
// takeover process is gone, so the old side's watch fires (unless held).
func (s *handoffScene) takeover(build string) (*API, int) {
	n := s.newAPI(true)
	s.mu.Lock()
	s.takeovers = append(s.takeovers, n)
	s.mu.Unlock()
	code := runTakeover(context.Background(), takeoverDeps{socketPath: s.socket, lockPath: s.lockPath, api: n, build: build})
	s.event(fmt.Sprintf("new:returned %d", code))
	if code != 0 && !s.holdPeer {
		s.closePeer()
	}
	return n, code
}

func (s *handoffScene) waitExit(ch chan int, what string, want int) {
	s.t.Helper()
	select {
	case got := <-ch:
		if got != want {
			s.t.Errorf("%s exit code = %d, want %d", what, got, want)
		}
	case <-time.After(5 * time.Second):
		s.t.Fatalf("%s exit seam never ran (events %v)", what, s.events)
	}
}

// createSession creates a session through c (proving that daemon takes
// writes) and tracks its shell for cleanup.
func (s *handoffScene) createSession(c *DaemonClient, a *API) string {
	s.t.Helper()
	id, err := c.CreateSession("after", "", 80, 24)
	if err != nil {
		s.t.Fatalf("CreateSession after the outcome: %v", err)
	}
	if sess, ok := a.engine.registry.Get(id); ok {
		s.mu.Lock()
		s.extraPids = append(s.extraPids, sess.Pid())
		s.mu.Unlock()
	}
	return id
}

func (s *handoffScene) cleanup() {
	s.mu.Lock()
	pids := append(slices.Clone(s.pids), s.extraPids...)
	takeovers := slices.Clone(s.takeovers)
	s.mu.Unlock()
	s.closePeer()
	// Release every engine's PTYs (in production the old daemon's exit, or a
	// dead takeover's, does; in-process they piled up across -count runs).
	// Live sessions are killed before any shell dies, so no exit watcher
	// starts a Wait; the SIGKILL then ends shells whose hubs a shutdown
	// already ended, and their sessions go once their watchers are done.
	var engines []*SessionEngine
	if s.old != nil {
		s.old.waitHandoff(5 * time.Second)
		engines = append(engines, s.old.engine) // first: it forked the shells
	}
	for _, n := range takeovers {
		engines = append(engines, n.engine)
	}
	for _, e := range engines {
		killSessions(e, true)
	}
	for _, pid := range pids {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	for _, e := range engines {
		killAllSessions(e)
	}
	for _, n := range takeovers {
		_ = n.Stop()
	}
	if s.old != nil {
		_ = s.old.Stop()
	}
	s.restore()
	if s.lockFree.Load() {
		s.t.Errorf("the daemon lock was free at a handoff step (events %v)", s.events)
	}
}

// stampConn records when bytes last arrived, so a viewer's close frame can
// be ordered against the old daemon's exit.
type stampConn struct {
	net.Conn
	last *atomic.Int64
}

func (c *stampConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.last.Store(time.Now().UnixNano())
	}
	return n, err
}

type tickViewer struct {
	mu       sync.Mutex
	out      []byte
	off      uint64
	offOK    bool
	closeErr error
	done     chan struct{}
	lastRead atomic.Int64
}

// dialViewer joins a relay session (resume: "" for a full replay) and reads
// in the background, tracking the resume offset as clients do.
func (s *handoffScene) dialViewer(port int, id, resume string) *tickViewer {
	s.t.Helper()
	v := &tickViewer{done: make(chan struct{})}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &stampConn{Conn: c, last: &v.lastRead}, nil
	}}}
	u := fmt.Sprintf("ws://127.0.0.1:%d/sessions/%s/ws", port, id)
	if resume != "" {
		u += "?resume=" + resume
	}
	ctx, cancel := context.WithCancel(context.Background())
	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPClient: client})
	if err != nil {
		cancel()
		s.t.Fatalf("dial viewer: %v", err)
	}
	conn.SetReadLimit(1 << 20)
	s.t.Cleanup(func() { conn.CloseNow(); cancel() })
	go func() {
		defer close(v.done)
		for {
			_, m, err := conn.Read(ctx)
			v.mu.Lock()
			if err != nil {
				v.closeErr = err
				v.mu.Unlock()
				return
			}
			switch {
			case len(m) > 0 && m[0] == relay.MsgOutput:
				v.out = append(v.out, m[1:]...)
				if v.offOK {
					v.off += uint64(len(m))
				}
			case len(m) > 0 && m[0] == relay.MsgMeta:
				var p relay.MetaPayload
				if json.Unmarshal(m[1:], &p) == nil && p.Offset != nil {
					v.off, v.offOK = *p.Offset, true
				}
			}
			v.mu.Unlock()
		}
	}()
	return v
}

func (v *tickViewer) state() ([]byte, uint64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return bytes.Clone(v.out), v.off
}

func (v *tickViewer) ticks() []int {
	out, _ := v.state()
	return tickNums(out)
}

func (v *tickViewer) has(s string) bool {
	out, _ := v.state()
	return bytes.Contains(out, []byte(s))
}

func (v *tickViewer) closeStatus() websocket.StatusCode {
	v.mu.Lock()
	defer v.mu.Unlock()
	return websocket.CloseStatus(v.closeErr)
}

func waitTicks(t *testing.T, what string, get func() []int, n int) {
	t.Helper()
	waitFor(t, what, 5*time.Second, func() bool { return len(get()) >= n })
}

func listByID(t *testing.T, c *DaemonClient) map[string]SessionInfo {
	t.Helper()
	list, err := c.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	m := map[string]SessionInfo{}
	for _, s := range list {
		m[s.ID] = s
	}
	return m
}

// handoffFD identifies one descriptor a takeover can receive.
type handoffFD struct {
	fd       int
	dev, ino uint64
	rdev     int64
}

// handoffFDs lists the descriptors a takeover can receive: PTYs and other
// character devices, listening sockets and the daemon lock. Pollers' sockets
// come and go and kqueue exit watchers live until their shell exits, so
// neither is listed.
func handoffFDs(t *testing.T, lockPath string) map[handoffFD]bool {
	t.Helper()
	lockFI, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lock := lockFI.Sys().(*syscall.Stat_t)
	dir, err := os.Open("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		t.Fatal(err)
	}
	fds := map[handoffFD]bool{}
	for _, name := range names {
		fd, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) != nil {
			continue
		}
		id := handoffFD{fd: fd, dev: uint64(st.Dev), ino: st.Ino, rdev: int64(st.Rdev)}
		switch st.Mode & syscall.S_IFMT {
		case syscall.S_IFCHR:
			fds[id] = true
		case syscall.S_IFREG:
			if st.Dev == lock.Dev && st.Ino == lock.Ino {
				fds[id] = true
			}
		case syscall.S_IFSOCK:
			if v, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN); err == nil && v != 0 {
				fds[id] = true
			}
		}
	}
	return fds
}

// handoffFDBaseline lists the receivable descriptors once pending *os.File
// finalizers have run (ptyFDBaseline's GC barrier), so a file an earlier
// test dropped is not in the baseline only to close mid-measurement.
func handoffFDBaseline(t *testing.T, lockPath string) map[handoffFD]bool {
	t.Helper()
	_ = ptyFDBaseline(t) // for its GC barrier
	return handoffFDs(t, lockPath)
}

// checkNewHandoffFDs fails on receivable descriptors open now that were not
// open at base: a leak. A baseline descriptor that closed meanwhile is fine
// (another test's leftover, or a file finalized late); the callers prove the
// old daemon kept its own PTYs by checking that it still serves.
func checkNewHandoffFDs(t *testing.T, lockPath string, base map[handoffFD]bool, what string) {
	t.Helper()
	var extra []string
	for id := range handoffFDs(t, lockPath) {
		if !base[id] {
			extra = append(extra, fmt.Sprintf("fd %d (dev %d ino %d rdev %d)", id.fd, id.dev, id.ino, id.rdev))
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Errorf("%s: %d descriptors not open before: %s", what, len(extra), strings.Join(extra, ", "))
	}
}

// T-D1: a live session (and one in the alt screen) and its share move from
// one daemon to another with every identity, offset and listener intact.
func TestHandoff_HappyPath(t *testing.T) {
	logs := captureDiag(t)
	s := newHandoffScene(t, tickScript(false), tickScript(true))
	shared := mustShare(t, s.client, s.ids[0])
	_, token := linkParts(t, shared.URL)
	relayPort := s.old.relayPort
	before, err := os.Stat(s.socket)
	if err != nil {
		t.Fatal(err)
	}
	oldInfo := listByID(t, s.client)
	var viewers []*tickViewer
	for _, id := range s.ids {
		v := s.dialViewer(relayPort, id, "")
		waitTicks(t, "viewer ticks", v.ticks, 3)
		viewers = append(viewers, v)
	}

	n, code := s.takeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0 (events %v)", code, s.events)
	}
	s.waitExit(s.oldExits, "old daemon", 0)
	exitAt := s.at("old:exit")

	fresh := NewDaemonClient(s.socket)
	newInfo := listByID(t, fresh)
	for i, id := range s.ids {
		o, nw := oldInfo[id], newInfo[id]
		if nw.ID != id || nw.Name != o.Name || nw.WorkDir != o.WorkDir || nw.CreatedAt != o.CreatedAt || nw.State != "running" {
			t.Errorf("session %d after = %+v, before %+v", i, nw, o)
		}
		sess, ok := n.engine.registry.Get(id)
		if !ok || sess.Pid() != s.pids[i] {
			t.Errorf("session %d pid after = %v, want %d", i, sess, s.pids[i])
		}
		if err := syscall.Kill(s.pids[i], 0); err != nil {
			t.Errorf("shell %d (pid %d) is gone: %v", i, s.pids[i], err)
		}
	}

	n.mu.RLock()
	rec := n.shares[s.ids[0]]
	n.mu.RUnlock()
	if rec.token != token {
		t.Errorf("share token changed across the handoff")
	}
	if code, loc := linkStatus(t, shared.URL); code != http.StatusFound || !strings.HasPrefix(loc, "/app/?session=") {
		t.Errorf("share link after the handoff = %d %q, want 302 to /app/", code, loc)
	}
	wrong := strings.Replace(shared.URL, url.QueryEscape(token), url.QueryEscape(token[:len(token)-2]+"xx"), 1)
	if code, _ := linkStatus(t, wrong); code == http.StatusFound || code < 400 {
		t.Errorf("a wrong token got %d after the handoff, want refused", code)
	}

	after, err := os.Stat(s.socket)
	if err != nil || !os.SameFile(before, after) {
		t.Errorf("socket inode changed or vanished: %v", err)
	}
	if n.relayPort != relayPort {
		t.Errorf("relay port = %d, want %d", n.relayPort, relayPort)
	}
	h, err := fresh.HealthInfo()
	if err != nil || h.Handoff != HandoffVersion {
		t.Errorf("health after = %+v, %v; want handoff %d", h, err, HandoffVersion)
	}
	s.createSession(fresh, n) // the old daemon is gated: only the new one takes a write

	for i, v := range viewers {
		<-v.done
		if got := v.closeStatus(); got != websocket.StatusServiceRestart {
			t.Errorf("viewer %d closed with %v, want 1012", i, got)
		}
		if last := time.Unix(0, v.lastRead.Load()); last.After(exitAt) {
			t.Errorf("viewer %d's close frame arrived %v after the old daemon exited", i, last.Sub(exitAt))
		}
		out, off := v.state()
		v2 := s.dialViewer(relayPort, s.ids[i], strconv.FormatUint(off, 10))
		waitTicks(t, "resumed ticks", v2.ticks, 3)
		resumed, _ := v2.state()
		checkContiguous(t, fmt.Sprintf("viewer %d across the handoff", i), tickNums(append(out, resumed...)))
		v3 := s.dialViewer(relayPort, s.ids[i], "")
		waitFor(t, "full replay history", 5*time.Second, func() bool { return v3.has("HIST-1") })
	}

	for _, id := range s.ids {
		hub, _ := s.old.engine.Manager().Get(id)
		select {
		case <-hub.Done():
			t.Errorf("old hub %s Done closed: viewers were told the session ended", id)
		default:
		}
	}
	if !lockHeld(s.lockPath) {
		t.Errorf("nobody holds the daemon lock after the handoff")
	}
	if !(s.idx("old:peer-pid") >= 0 && s.idx("old:peer-pid") < s.idx("old:watch-peer") && s.idx("old:watch-peer") < s.idx("old:after-hello")) {
		t.Errorf("peer pid not read and watched at HELLO before FREEZE: %v", s.events)
	}
	if strings.Contains(string(logs.bytes()), token) {
		t.Errorf("a share token appears in the log")
	}
}

// W5: a viewer that never answers the close handshake delays the old
// daemon's exit by the restart drain, no more.
func TestHandoff_RestartDrainBounded(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/sessions/%s/ws", s.old.relayPort, s.ids[0]), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow() // never read: the close handshake is never answered
	hub, _ := s.old.engine.Manager().Get(s.ids[0])
	waitFor(t, "silent viewer subscribed", 5*time.Second, func() bool { return hub.SubscriberCount() == 1 })

	if _, code := s.takeover(BuildVersion); code != 0 {
		t.Fatalf("runTakeover = %d", code)
	}
	s.waitExit(s.oldExits, "old daemon", 0)
	d := s.at("old:exit").Sub(s.at("new:after-serve-before-committed"))
	if d > 1500*time.Millisecond || d < restartDrain-100*time.Millisecond {
		t.Errorf("old daemon exited %v after COMMITTED; want about restartDrain (%v), at most 1.5 s", d, restartDrain)
	}
}

// TestHandoff_TakeoverHoldsInputUntilOldExits (WR-02): after COMMIT the
// takeover queues its viewers' input but writes none of it to the PTYs until
// the old daemon, which still drains its own viewers' backlog there, exits.
func TestHandoff_TakeoverHoldsInputUntilOldExits(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	release := make(chan struct{})
	s.setFault(func(side, step string) error {
		if side+":"+step == "old:committed-received" {
			<-release // the old daemon is still in finish
		}
		return nil
	})
	n, code := s.takeover(BuildVersion)
	if code != 0 {
		t.Fatalf("runTakeover = %d", code)
	}
	hub, ok := n.engine.Manager().Get(s.ids[0])
	if !ok {
		t.Fatal("the takeover has no hub for the session")
	}
	if err := hub.OpenInput(nil).Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if idle, queued := hub.WaitInputIdle(200 * time.Millisecond); idle || queued != 1 {
		t.Fatalf("takeover input before the old daemon exited: idle=%t queued=%d, want held (false, 1)", idle, queued)
	}
	close(release)
	s.waitExit(s.oldExits, "old daemon", 0)
	if idle, queued := hub.WaitInputIdle(2 * time.Second); !idle {
		t.Fatalf("takeover input still held after the old daemon exited: queued=%d", queued)
	}
}

// TestHandoff_FinishWaitsForInput (INPT-01, Research Pitfall 7): before the
// old daemon exits, finish waits for queued input to reach each PTY within the
// shared deadline, and reports (and logs) whatever is still queued at it.
func TestHandoff_FinishWaitsForInput(t *testing.T) {
	diag := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(diag)
	t.Cleanup(func() { log.SetOutput(prev) })

	m := relay.NewHubManager()
	t.Cleanup(m.Shutdown)
	slowR, slowW := io.Pipe()
	stuckR, stuckW := io.Pipe() // never read: a shell that stopped reading
	t.Cleanup(func() { _ = slowR.Close(); _ = stuckR.Close() })
	outR, outW := io.Pipe() // PTY output that never ends: EOF would end the hubs
	t.Cleanup(func() { _ = outW.Close() })
	slow := m.Create("slow-session", outR, slowW, nil)
	stuck := m.Create("stuck-session", outR, stuckW, nil)

	in := slow.OpenInput(nil)
	for i := 0; i < 20; i++ {
		if err := in.Write(bytes.Repeat([]byte{'s'}, 1024)); err != nil {
			t.Fatal(err)
		}
	}
	go func() { // drains, slowly
		buf := make([]byte, 1024)
		for {
			if _, err := slowR.Read(buf); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	start := time.Now()
	if lost := waitHubsInputIdle(m, start.Add(2*time.Second)); len(lost) != 0 {
		t.Fatalf("lost %v, want nothing (the slow PTY drains well within the deadline)", lost)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("waited %v for a queue that drains in ~100 ms", d)
	}

	sin := stuck.OpenInput(nil)
	for i := 0; i < 3; i++ {
		if err := sin.Write(bytes.Repeat([]byte{'x'}, 100)); err != nil {
			t.Fatal(err)
		}
	}
	start = time.Now()
	lost := waitHubsInputIdle(m, start.Add(200*time.Millisecond))
	if d := time.Since(start); d > 600*time.Millisecond {
		t.Fatalf("returned %v after a 200 ms deadline", d)
	}
	if len(lost) != 1 || lost["stuck-session"] != 300 {
		t.Fatalf("lost %v, want stuck-session: 300 bytes", lost)
	}
	if want := "handoff: input still queued at exit session=stuck-se bytes=300"; !diag.has(want) {
		t.Fatalf("log lacks %q:\n%s", want, diag.bytes())
	}
}

// W4: the share certificate is warmed before HELLO, once; with shares and a
// failed warm the takeover aborts and the old daemon serves on.
func TestHandoff_CertWarmBeforeHello(t *testing.T) {
	t.Run("warm first", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false))
		mustShare(t, s.client, s.ids[0])
		if _, code := s.takeover(BuildVersion); code != 0 {
			t.Fatalf("runTakeover = %d", code)
		}
		if s.count("new:warm") != 1 || s.idx("new:warm") > s.idx("old:after-hello") {
			t.Errorf("warm not once before HELLO: %v", s.events)
		}
	})
	t.Run("warm fails with shares", func(t *testing.T) {
		logs := captureDiag(t)
		s := newHandoffScene(t, tickScript(false))
		mustShare(t, s.client, s.ids[0])
		s.warm = func(context.Context, string) error { return errors.New("no certificate") }
		if _, code := s.takeover(BuildVersion); code != takeoverImport {
			t.Fatalf("runTakeover = %d, want %d", code, takeoverImport)
		}
		if !logs.has("share certificate unavailable") {
			t.Errorf("log lacks the reason:\n%s", logs.bytes())
		}
		waitFor(t, "old daemon thawed", 5*time.Second, func() bool { return !s.old.frozen.Load() })
		s.createSession(s.client, s.old)
		last := lastTick(ringTicks(s.old, s.ids[0]))
		waitFor(t, "old ticks advance", 5*time.Second, func() bool { return lastTick(ringTicks(s.old, s.ids[0])) >= last+3 })
		if s.idx("old:exit") >= 0 {
			t.Errorf("old daemon exited")
		}
	})
}

// T-09-16: the route exists only on the unix socket.
func TestHandoff_RouteOnlyOnUnixSocket(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	shared := mustShare(t, s.client, s.ids[0])
	u, _ := linkParts(t, shared.URL)
	for _, tc := range []struct {
		name, url string
		client    *http.Client
	}{
		{"relay port", fmt.Sprintf("http://127.0.0.1:%d/handoff", s.old.relayPort), &http.Client{Timeout: 5 * time.Second}},
		{"share server", "https://" + u.Host + "/handoff", insecureNoRedirectClient()},
	} {
		req, _ := http.NewRequest(http.MethodPost, tc.url, nil)
		req.Header.Set("Upgrade", handoffUpgrade)
		req.Header.Set("Connection", "Upgrade")
		resp, err := tc.client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		resp.Body.Close()
		// The share server's "GET /" catch-all turns a POST into 405.
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: POST /handoff = %d, want 404 (or 405)", tc.name, resp.StatusCode)
		}
	}
	if s.idx("old:after-hello") >= 0 {
		t.Errorf("a handoff started: %v", s.events)
	}
}

// T-09-15: a peer of another uid is refused before anything freezes or any
// fd moves.
func TestHandoff_RefusesOtherUID(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	peerUID = func(*net.UnixConn) (uint32, error) { return uint32(os.Getuid()) + 1, nil }
	base := handoffFDBaseline(t, s.lockPath)
	if _, code := s.takeover(BuildVersion); code != takeoverRefused {
		t.Fatalf("runTakeover = %d, want %d", code, takeoverRefused)
	}
	if s.idx("old:after-hello") >= 0 || s.idx("old:watch-peer") >= 0 || s.old.frozen.Load() {
		t.Errorf("a refused HELLO went on: %v", s.events)
	}
	checkNewHandoffFDs(t, s.lockPath, base, "after the refusal")
	last := lastTick(ringTicks(s.old, s.ids[0]))
	waitFor(t, "old ticks advance", 5*time.Second, func() bool { return lastTick(ringTicks(s.old, s.ids[0])) >= last+3 })
}

// T-09-23: an older release cannot pull the sessions back.
func TestHandoff_RefusesOlderRelease(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		older bool
	}{
		{"v1.1.0", "v1.2.0", true},
		{"v1.2.0-vmA", "v1.2.0-vmB", true},
		{"v1.2.0-vmB", "v1.2.0-vmA", false},
		{"v1.2.0-rc1", "v1.2.0", true},
		{"v1.2.0", "v1.2.0-rc1", false},
		{"v1.10.0", "v1.9.0", false},
		{"v1.2.0", "v1.2.0", false},
		{"dev", "v1.2.0", false},
		{"v1.2.0", "dev", false},
	} {
		if got := olderRelease(tc.a, tc.b); got != tc.older {
			t.Errorf("olderRelease(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.older)
		}
	}

	prev := BuildVersion
	t.Cleanup(func() { BuildVersion = prev })
	s := newHandoffScene(t, tickScript(false))
	s.holdPeer = true // the refused takeovers never reach the watch; the last one commits
	BuildVersion = "v1.2.0"
	if _, code := s.takeover("v1.1.0"); code != takeoverRefused {
		t.Errorf("v1.1.0 -> v1.2.0 = %d, want %d", code, takeoverRefused)
	}
	BuildVersion = "v1.2.0-vmB"
	if _, code := s.takeover("v1.2.0-vmA"); code != takeoverRefused {
		t.Errorf("v1.2.0-vmA -> v1.2.0-vmB = %d, want %d", code, takeoverRefused)
	}
	if s.idx("old:after-hello") >= 0 {
		t.Errorf("a refused HELLO went on: %v", s.events)
	}
	BuildVersion = "v1.2.0-vmA"
	if _, code := s.takeover("v1.2.0-vmB"); code != takeoverCommitted {
		t.Errorf("v1.2.0-vmB -> v1.2.0-vmA = %d, want 0", code)
	}
}

// IN-01: HELLO is read under a small cap, before anything about the peer is
// known: a header claiming a large frame is refused at once, without the
// daemon waiting for (or allocating) its body.
func TestHandoff_HelloCapped(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	tk := &takeover{}
	if err := tk.dial(context.Background(), s.socket); err != nil {
		t.Fatal(err)
	}
	defer tk.c.Close()
	if _, err := tk.c.Write([]byte{frameHello, 0, 0x10, 0, 0}); err != nil { // a 1 MiB HELLO
		t.Fatal(err)
	}
	_ = tk.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := tk.c.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("read after an oversized HELLO = %v, want EOF (the daemon dropped it)", err)
	}
	if s.idx("old:peer-pid") >= 0 {
		t.Errorf("an oversized HELLO went on: %v", s.events)
	}
}

// WR-02 (it2): READY's digest is over the SESSION frame bytes. One changed
// payload byte that still decodes fails it; a frame from a build whose
// hsSession has a field this build lacks (gob drops it) still matches.
func TestHandoff_SessionFrameDigest(t *testing.T) {
	m := hsSession{ID: "s", Pid: 42, Hub: relay.HubState{SessionID: "s", Ring: relay.RingState{Buf: []byte("ring-MARKER-bytes")}}}
	var buf bytes.Buffer
	sent, err := writeSessionFrame(&buf, m)
	if err != nil {
		t.Fatal(err)
	}
	frame := buf.Bytes()
	var got hsSession
	if sum, err := readSessionFrame(bytes.NewReader(frame), &got); err != nil || sum != sent {
		t.Fatalf("unchanged frame: err %v, digest match %v", err, sum == sent)
	}

	bad := bytes.Clone(frame)
	i := bytes.Index(bad, []byte("MARKER"))
	if i < 0 {
		t.Fatal("setup: marker not in the frame")
	}
	bad[i] ^= 0x20 // 'M' -> 'm': still decodes
	got = hsSession{}
	sum, err := readSessionFrame(bytes.NewReader(bad), &got)
	if err != nil {
		t.Fatalf("changed byte did not decode: %v", err)
	}
	if string(got.Hub.Ring.Buf) != "ring-mARKER-bytes" {
		t.Fatalf("setup: decoded ring %q", got.Hub.Ring.Buf)
	}
	if sum == sent {
		t.Fatal("a changed payload byte kept the digest")
	}

	type futureSession struct { // a later build's hsSession, one field more
		ID, Name, WorkDir, CLI string
		CreatedAt              time.Time
		Pid                    int
		Hub                    relay.HubState
		CursorStyle            int
	}
	buf.Reset()
	h := sha256.New()
	if err := writeFrame(io.MultiWriter(&buf, h), frameSession, futureSession{ID: "s", Pid: 42, Hub: m.Hub, CursorStyle: 3}); err != nil {
		t.Fatal(err)
	}
	got = hsSession{}
	if sum, err := readSessionFrame(&buf, &got); err != nil || sum != [32]byte(h.Sum(nil)) || got.ID != "s" {
		t.Fatalf("newer-schema frame: err %v, digest match %v, id %q", err, sum == [32]byte(h.Sum(nil)), got.ID)
	}
}

// IN-02: pid 1 (or less) is never adopted: killAdopted signals -pid, and
// kill(-1) reaches every process the user owns.
func TestHandoff_RejectsPidOne(t *testing.T) {
	for _, pid := range []int{-1, 0, 1} {
		m := hsSession{ID: "s", Pid: pid, Hub: relay.HubState{SessionID: "s"}}
		if why := (&takeover{}).invalid(m); why == "" {
			t.Errorf("a session frame with pid %d was accepted", pid)
		}
	}
	if why := (&takeover{}).invalid(hsSession{ID: "s", Pid: 2, Hub: relay.HubState{SessionID: "s"}}); why != "" {
		t.Errorf("pid 2 refused: %s", why)
	}
}

// fakeAddrListener is a listener that reports addr, for addresses a test
// cannot bind (a relay on every interface).
type fakeAddrListener struct {
	net.Listener
	addr net.Addr
}

func (l fakeAddrListener) Addr() net.Addr { return l.addr }

// WR-05: the takeover refuses received fds that are not what the daemon
// holds itself: daemon.lock, the daemon socket and a loopback relay (which
// serves unauthenticated, full-write shells). Refused, it aborts and the old
// daemon serves on.
func TestHandoff_RefusesForeignFiles(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")
	lock, err := lockDaemon(lockPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lock.Close() })
	other, err := os.Create(filepath.Join(dir, "other.lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	sock := shortSocketPath(t, "wr05.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	otherSock := shortSocketPath(t, "other.sock")
	otherLn, err := net.Listen("unix", otherSock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { otherLn.Close() })
	loopRelay := fakeAddrListener{addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4000}}
	for name, tc := range map[string]struct {
		lock         *os.File
		api, relayLn net.Listener
		ok           bool
	}{
		"as held":             {lock, ln, loopRelay, true},
		"another lock file":   {other, ln, loopRelay, false},
		"another socket":      {lock, otherLn, loopRelay, false},
		"relay on every host": {lock, ln, fakeAddrListener{addr: &net.TCPAddr{IP: net.IPv4zero, Port: 4000}}, false},
		"relay on the LAN":    {lock, ln, fakeAddrListener{addr: &net.TCPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 4000}}, false},
		"relay not TCP":       {lock, ln, otherLn, false},
	} {
		if err := checkGlobalFiles(tc.lock, lockPath, tc.api, tc.relayLn, sock); (err == nil) != tc.ok {
			t.Errorf("%s: checkGlobalFiles = %v, want ok %v", name, err, tc.ok)
		}
	}

	s := newHandoffScene(t, tickScript(false))
	base := handoffFDBaseline(t, s.lockPath)
	n := s.newAPI(true)
	s.mu.Lock()
	s.takeovers = append(s.takeovers, n)
	s.mu.Unlock()
	code := runTakeover(context.Background(), takeoverDeps{socketPath: s.socket, lockPath: other.Name(), api: n, build: BuildVersion})
	s.closePeer()
	if code != takeoverImport {
		t.Fatalf("runTakeover with a foreign lock = %d, want %d", code, takeoverImport)
	}
	waitFor(t, "old daemon thawed", 5*time.Second, func() bool { return !s.old.frozen.Load() })
	checkNewHandoffFDs(t, s.lockPath, base, "after the refusal")
	checkOldServes(t, s, n, nil, false)
}

// T-09-20: the log of a handoff with a shared session holds no token.
func TestHandoff_NoSecretsInLog(t *testing.T) {
	logs := captureDiag(t)
	s := newHandoffScene(t, tickScript(false))
	shared := mustShare(t, s.client, s.ids[0])
	_, token := linkParts(t, shared.URL)
	if _, code := s.takeover(BuildVersion); code != 0 {
		t.Fatalf("runTakeover = %d", code)
	}
	s.waitExit(s.oldExits, "old daemon", 0)
	out := string(logs.bytes())
	if !strings.Contains(out, "handoff: committed") {
		t.Fatalf("handoff log not captured:\n%s", out)
	}
	if strings.Contains(out, token) {
		t.Errorf("a share token appears in the handoff log")
	}
}
