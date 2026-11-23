package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/attach"
	"github.com/scottkw/portty/internal/relay"
)

// TestHandoffIT is the two-binary upgrade test (Phase 9 Research, Test
// Strategy "Integration"). It drives prebuilt binaries only, each run against
// its own temp HOME, so it never builds anything (the VM has no Go
// toolchain). Real daemons from two builds must not run on the user's Mac, so
// it is env-gated and run in the office Tart VM (09-VM-CHECKS rows I-01..I-03).
func TestHandoffIT(t *testing.T) {
	if os.Getenv("PORTTY_HANDOFF_IT") != "1" {
		t.Skip("two-binary handoff test: set PORTTY_HANDOFF_IT=1 and PORTTY_HANDOFF_IT_A, _B, _BFAULT, _OLD; run it in the office Tart VM (portty-test), never on the host")
	}
	bins := map[string]string{}
	for _, k := range []string{"A", "B", "BFAULT", "OLD"} {
		v := os.Getenv("PORTTY_HANDOFF_IT_" + k)
		if v == "" {
			t.Fatalf("PORTTY_HANDOFF_IT_%s is not set (the path of a prebuilt binary)", k)
		}
		if _, err := os.Stat(v); err != nil {
			t.Fatalf("PORTTY_HANDOFF_IT_%s: %v", k, err)
		}
		bins[k] = v
	}

	t.Run("upgrade", func(t *testing.T) {
		r := newITRig(t, bins["A"])
		ids := r.liveSessions(2)
		chk := r.newSession(bins["A"])
		pids := r.shellPids(len(ids) + 1)
		before := r.fdCheck(chk, "FDCHKA")
		v := r.viewer(ids[0])
		tick0 := v.waitTicks(t, 5)

		out, errOut, rc := r.run(nil, bins["B"], "list", "--local")
		t.Logf("B list --local: rc=%d stderr=%q", rc, errOut)
		if rc != 0 || strings.Contains(errOut, "couldn't move") {
			t.Fatalf("B list --local: rc=%d, stdout=%q, stderr=%q; want rc 0 and no couldn't-move line", rc, out, errOut)
		}
		h := r.health()
		t.Logf("/health after: %+v", h)
		if h.Version != "v1.2.0-itB" || h.API != APIVersion || h.Handoff != 1 {
			t.Fatalf("/health = %+v; want version v1.2.0-itB, api %d, handoff 1", h, APIVersion)
		}
		after := r.sessionIDs()
		want := append(slices.Clone(ids), chk)
		slices.Sort(want)
		t.Logf("session ids before %v, after %v", want, after)
		if !slices.Equal(after, want) {
			t.Fatalf("session ids after = %v; want %v", after, want)
		}
		for _, p := range pids {
			if !itAlive(p) {
				t.Fatalf("shell %d is gone after the handoff", p)
			}
			if pp := itPpid(t, p); pp != 1 {
				t.Fatalf("shell %d ppid = %d; want 1 (its old daemon exited)", p, pp)
			}
		}
		t.Logf("shell pids %v alive with ppid 1", pids)
		select {
		case <-r.exited:
			if r.exitErr != nil {
				t.Fatalf("A daemon exit: %v; want exit 0", r.exitErr)
			}
			t.Logf("A daemon %d exited 0", r.daemonPid)
		case <-time.After(10 * time.Second):
			t.Fatalf("A daemon %d still running 10 s after the handoff", r.daemonPid)
		}

		last := v.waitTicksAfter(t, tick0+10)
		first, n := v.contiguous(t)
		t.Logf("viewer: reconnects=%d, TICK-1 %d..%d (%d lines, no gap, no repeat), %d at the upgrade", v.reconnects.Load(), first, last, n, tick0)
		if v.reconnects.Load() < 1 {
			t.Fatalf("viewer never reconnected")
		}

		fresh := r.freshOutput(ids[0], "HIST-1-300\r\n")
		if !strings.Contains(fresh, "HIST-1-1\r\n") || !strings.Contains(fresh, "HIST-1-300\r\n") {
			t.Fatalf("a fresh viewer's replay lacks HIST-1-1 or HIST-1-300 (%d bytes)", len(fresh))
		}
		t.Logf("fresh viewer: replay holds HIST-1-1 and HIST-1-300 (%d bytes)", len(fresh))

		// 09-05's open item: no shell sees the status pipe or its variable,
		// whether adopted (started by A) or started by B after the commit.
		adopted := r.fdCheck(chk, "FDCHKC")
		fromB := r.fdCheck(r.newSession(bins["B"]), "FDCHKB")
		t.Logf("fd/env check: A shell before %q, adopted after %q, B shell %q", before, adopted, fromB)
		for _, c := range []string{before, adopted, fromB} {
			if !strings.HasPrefix(c, "env=0 fd3=closed ") {
				t.Fatalf("fd/env check = %q; want env=0 fd3=closed", c)
			}
		}
	})

	t.Run("fault", func(t *testing.T) {
		r := newITRig(t, bins["A"])
		ids := r.liveSessions(2)
		pids := r.shellPids(len(ids))
		v := r.viewer(ids[0])
		tick0 := v.waitTicks(t, 5)

		out, errOut, rc := r.run([]string{"PORTTY_VMPREVIEW_HANDOFF_FAULT=new:before-ready:exit"}, bins["BFAULT"], "list", "--local")
		t.Logf("B-fault list --local: rc=%d stderr=%q", rc, errOut)
		if rc != 1 ||
			!strings.Contains(errOut, "couldn't move the running sessions to version v1.2.0-itB") ||
			!strings.Contains(errOut, "restarting it to version v1.2.0-itB would end them") {
			t.Fatalf("B-fault list --local: rc=%d, stdout=%q, stderr=%q; want rc 1, the couldn't-move line and the DEBT-08 refusal", rc, out, errOut)
		}
		if h := r.health(); h.Version != "v1.2.0-itA" {
			t.Fatalf("/health = %+v; want A still serving", h)
		}
		select {
		case <-r.exited:
			t.Fatalf("A daemon exited (%v); want it serving", r.exitErr)
		default:
		}
		for _, p := range pids {
			if !itAlive(p) || itPpid(t, p) != r.daemonPid {
				t.Fatalf("shell %d: alive=%v; want alive under A daemon %d", p, itAlive(p), r.daemonPid)
			}
		}
		t.Logf("/health still v1.2.0-itA; A daemon %d and shells %v alive", r.daemonPid, pids)
		last := v.waitTicksAfter(t, tick0+10)
		first, n := v.contiguous(t)
		t.Logf("viewer: reconnects=%d, TICK-1 %d..%d (%d lines, no gap, no repeat), %d at the attempt", v.reconnects.Load(), first, last, n, tick0)
		if v.reconnects.Load() != 0 {
			t.Fatalf("viewer reconnected %d times; want 0 (the old daemon kept serving)", v.reconnects.Load())
		}
	})

	t.Run("from-v1.1", func(t *testing.T) {
		r := newITRig(t, bins["OLD"])
		ids := r.liveSessions(2)
		pids := r.shellPids(len(ids))
		v := r.viewer(ids[0])
		tick0 := v.waitTicks(t, 5)

		out, errOut, rc := r.run(nil, bins["B"], "list", "--local")
		t.Logf("B list --local against v1.1: rc=%d stderr=%q", rc, errOut)
		if rc != 1 || !strings.Contains(errOut, "restarting it to version v1.2.0-itB would end them") || strings.Contains(errOut, "couldn't move") {
			t.Fatalf("rc=%d, stdout=%q, stderr=%q; want rc 1, the plain DEBT-08 refusal, no couldn't-move line", rc, out, errOut)
		}
		raw := r.rawHealth()
		t.Logf("/health after: %s", raw)
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("/health %q: %v", raw, err)
		}
		if _, has := m["api"]; has || m["version"] != "v1.1.0-itOld" {
			t.Fatalf("/health = %s; want version v1.1.0-itOld and no api key", raw)
		}
		for _, p := range pids {
			if !itAlive(p) || itPpid(t, p) != r.daemonPid {
				t.Fatalf("shell %d: alive=%v; want alive under the v1.1 daemon %d", p, itAlive(p), r.daemonPid)
			}
		}
		last := v.waitTicksAfter(t, tick0+5)
		first, n := v.contiguous(t)
		t.Logf("v1.1 daemon %d and shells %v alive; viewer: reconnects=%d, TICK-1 %d..%d (%d lines, no gap, no repeat)", r.daemonPid, pids, v.reconnects.Load(), first, last, n)
	})
}

// itRig is one subtest's daemon in its own temp HOME.
type itRig struct {
	t          *testing.T
	home, sock string
	env        []string
	client     *DaemonClient
	daemonPid  int
	exited     chan struct{} // closed once the started daemon was reaped
	exitErr    error         // its Wait result, read after exited
	cmd        *exec.Cmd
	stops      []func()
	sessions   []string
	shells     []int // shellPids' result, reaped in cleanup
}

func newITRig(t *testing.T, bin string) *itRig {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "pit-") // short: the 103-byte socket path limit
	if err != nil {
		t.Fatal(err)
	}
	// A .zshrc keeps zsh's new-user wizard out of the login shells; no
	// history file, so no shell writes into the HOME after cleanup removed it.
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("unset HISTFILE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &itRig{t: t, home: home, sock: filepath.Join(home, "Library", "Application Support", "portty", "daemon.sock")}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if k == "HOME" || k == "XDG_CONFIG_HOME" || strings.HasPrefix(k, "PORTTY_") {
			continue
		}
		r.env = append(r.env, kv)
	}
	r.env = append(r.env, "HOME="+home)
	r.client = NewDaemonClient(r.sock)

	logf, err := os.Create(filepath.Join(home, "started-daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	r.cmd = exec.Command(bin, "daemon")
	r.cmd.Env, r.cmd.Stdout, r.cmd.Stderr = r.env, logf, logf
	r.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := r.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	logf.Close()
	r.daemonPid = r.cmd.Process.Pid
	r.exited = make(chan struct{})
	go func() { r.exitErr = r.cmd.Wait(); close(r.exited) }()
	t.Cleanup(r.cleanup)

	deadline := time.Now().Add(15 * time.Second)
	for r.client.Health() != nil {
		if time.Now().After(deadline) {
			t.Fatalf("daemon %s did not answer within 15 s", bin)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return r
}

// cleanup stops viewers, the daemon this rig started (SIGTERM) and whatever
// daemon still answers the socket (the handed-off one, through /shutdown),
// logs the handoff lines and removes the HOME.
func (r *itRig) cleanup() {
	for _, stop := range slices.Backward(r.stops) {
		stop()
	}
	select {
	case <-r.exited:
	default:
		_ = r.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-r.exited:
		case <-time.After(5 * time.Second):
			_ = r.cmd.Process.Kill()
		}
	}
	if r.client.Health() == nil {
		_ = r.client.ShutdownDaemon()
		for i := 0; i < 50 && r.client.Health() == nil; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	for i := 0; i < 50 && slices.ContainsFunc(r.shells, itAlive); i++ {
		time.Sleep(100 * time.Millisecond) // the shells exit on the hangup
	}
	for _, p := range r.shells {
		if itAlive(p) {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}
	for _, f := range []string{"started-daemon.log", "Library/Application Support/portty/daemon.log", "Library/Application Support/portty/daemon.log.1"} {
		b, _ := os.ReadFile(filepath.Join(r.home, f))
		for _, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "handoff") || strings.Contains(line, "takeover") {
				r.t.Logf("%s: %s", filepath.Base(f), line)
			}
		}
	}
	_ = os.RemoveAll(r.home)
}

// run runs bin with the rig's env plus extra, bounded at 90 s.
func (r *itRig) run(extra []string, bin string, args ...string) (stdout, stderr string, rc int) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(slices.Clone(r.env), extra...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	rc = 0
	if err != nil {
		rc = -1
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		}
	}
	return o.String(), e.String(), rc
}

// newSession runs `bin new` and returns the id.
func (r *itRig) newSession(bin string) string {
	r.t.Helper()
	out, errOut, rc := r.run(nil, bin, "new")
	id := strings.TrimSpace(out)
	if rc != 0 || id == "" {
		r.t.Fatalf("%s new: rc=%d stdout=%q stderr=%q", bin, rc, out, errOut)
	}
	return id
}

// liveSessions creates n sessions; session k gets a HIST-k-1..300 burst and
// a 200 ms TICK-k counter. The typed text never contains the markers.
func (r *itRig) liveSessions(n int) []string {
	r.t.Helper()
	ids := make([]string, n)
	for k := range n {
		ids[k] = r.newSession(r.cmd.Path)
		r.sessions = append(r.sessions, ids[k])
		r.typeLine(ids[k], fmt.Sprintf(
			"for j in $(seq 1 300); do printf 'HIST-%%s-%%s\\n' %[1]d $j; done; i=0; while :; do i=$((i+1)); printf 'TICK-%%s %%s\\n' %[1]d $i; sleep 0.2; done", k+1))
	}
	return ids
}

// shellPids returns the started daemon's children (the session shells).
func (r *itRig) shellPids(want int) []int {
	r.t.Helper()
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(r.daemonPid)).Output()
	var pids []int
	for _, f := range strings.Fields(string(out)) {
		p, _ := strconv.Atoi(f)
		pids = append(pids, p)
	}
	if len(pids) != want {
		r.t.Fatalf("pgrep -P %d = %v; want the session shells", r.daemonPid, pids)
	}
	r.t.Logf("shell pids under daemon %d: %v", r.daemonPid, pids)
	r.shells = pids
	return pids
}

// relayURL re-reads the relay port; 0 when the daemon cannot say (no
// t.Fatal: attach.Run calls it from its own goroutine).
func (r *itRig) relayURL(id string) string {
	port, _ := r.client.GetRelayPort()
	return fmt.Sprintf("ws://127.0.0.1:%d/sessions/%s/ws", port, id)
}

// dial opens a relay WebSocket and returns it with a channel of its output.
func (r *itRig) dial(id string) (*websocket.Conn, <-chan []byte) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, r.relayURL(id), nil)
	if err != nil {
		r.t.Fatalf("dial %s: %v", id, err)
	}
	conn.SetReadLimit(relay.ViewerReadLimit)
	out := make(chan []byte, 1024)
	go func() {
		defer close(out)
		for {
			_, msg, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			if typ, p, err := relay.ParseFrame(msg); err == nil && typ == relay.MsgOutput {
				out <- p
			}
		}
	}()
	return conn, out
}

// typeLine waits for the shell's first output (its prompt), then types line.
func (r *itRig) typeLine(id, line string) {
	r.t.Helper()
	conn, out := r.dial(id)
	defer conn.Close(websocket.StatusNormalClosure, "")
	select {
	case <-out:
	case <-time.After(10 * time.Second):
		r.t.Fatalf("session %s printed nothing within 10 s", id)
	}
	time.Sleep(300 * time.Millisecond) // let zle finish drawing the prompt
	if err := conn.Write(context.Background(), websocket.MessageBinary, relay.MakeInputFrame([]byte(line+"\r"))); err != nil {
		r.t.Fatalf("type into %s: %v", id, err)
	}
}

// fdCheck types a check into an idle shell and returns "env=<count of
// PORTTY_TAKEOVER variables> fd3=<open|closed> ls=<ls /dev/fd>". The shell's
// own fd 3 is tested with the builtin [ (ls /dev/fd always lists its own
// directory fd as 3).
func (r *itRig) fdCheck(id, tag string) string {
	r.t.Helper()
	conn, out := r.dial(id)
	defer conn.Close(websocket.StatusNormalClosure, "")
	time.Sleep(500 * time.Millisecond)
	line := fmt.Sprintf(`p=%s; e=$(env | grep -c PORTTY_TAKEOVER); if [ -e /dev/fd/3 ]; then f=open; else f=closed; fi; echo "$p env=$e fd3=$f ls=$(ls /dev/fd | tr '\n' ' ')"`, tag)
	if err := conn.Write(context.Background(), websocket.MessageBinary, relay.MakeInputFrame([]byte(line+"\r"))); err != nil {
		r.t.Fatalf("type into %s: %v", id, err)
	}
	re := regexp.MustCompile(tag + ` (env=\d+ fd3=\w+ ls=[0-9 ]*)`)
	var buf []byte
	deadline := time.After(10 * time.Second)
	for {
		select {
		case p, ok := <-out:
			if !ok {
				r.t.Fatalf("session %s closed during the fd check", id)
			}
			buf = append(buf, p...)
			if m := re.FindSubmatch(buf); m != nil {
				return strings.TrimSpace(string(m[1]))
			}
		case <-deadline:
			r.t.Fatalf("fd check %s: no result within 10 s; output %q", tag, buf)
		}
	}
}

// freshOutput collects a new viewer's output until it holds until (or 10 s).
func (r *itRig) freshOutput(id, until string) string {
	r.t.Helper()
	conn, out := r.dial(id)
	defer conn.Close(websocket.StatusNormalClosure, "")
	var buf []byte
	deadline := time.After(10 * time.Second)
	for !bytes.Contains(buf, []byte(until)) {
		select {
		case p, ok := <-out:
			if !ok {
				return string(buf)
			}
			buf = append(buf, p...)
		case <-deadline:
			return string(buf)
		}
	}
	return string(buf)
}

func (r *itRig) sessionIDs() []string {
	r.t.Helper()
	ss, err := r.client.ListSessions()
	if err != nil {
		r.t.Fatalf("list sessions: %v", err)
	}
	var ids []string
	for _, s := range ss {
		if s.State != "stopped" {
			ids = append(ids, s.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func (r *itRig) health() HealthResponse {
	r.t.Helper()
	h, err := r.client.HealthInfo()
	if err != nil {
		r.t.Fatalf("/health: %v", err)
	}
	return h
}

func (r *itRig) rawHealth() string {
	r.t.Helper()
	resp, err := r.client.http.Get(r.client.base + "/health")
	if err != nil {
		r.t.Fatalf("/health: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(b))
}

// itViewer is a session-1 viewer on the CLI's own reconnect loop (attach.Run).
type itViewer struct {
	mu         sync.Mutex
	buf        bytes.Buffer
	reconnects atomic.Int32
}

func (v *itViewer) Write(p []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.buf.Write(p)
}

var itTickRE = regexp.MustCompile(`TICK-1 (\d+)\r?\n`)

func (v *itViewer) ticks() []int {
	v.mu.Lock()
	defer v.mu.Unlock()
	var ns []int
	for _, m := range itTickRE.FindAllSubmatch(v.buf.Bytes(), -1) {
		n, _ := strconv.Atoi(string(m[1]))
		ns = append(ns, n)
	}
	return ns
}

// waitTicks waits for n ticks and returns the last one.
func (v *itViewer) waitTicks(t *testing.T, n int) int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if ts := v.ticks(); len(ts) >= n {
			return ts[len(ts)-1]
		}
		if time.Now().After(deadline) {
			t.Fatalf("viewer saw %d ticks within 20 s; want %d", len(v.ticks()), n)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// waitTicksAfter waits for a tick of at least min and returns the last one.
func (v *itViewer) waitTicksAfter(t *testing.T, min int) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if ts := v.ticks(); len(ts) > 0 && ts[len(ts)-1] >= min {
			return ts[len(ts)-1]
		}
		if time.Now().After(deadline) {
			ts := v.ticks()
			t.Fatalf("viewer's ticks did not reach %d within 30 s (%d seen, reconnects %d)", min, len(ts), v.reconnects.Load())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// contiguous fails on a gap or a repeat and returns the first tick and count.
func (v *itViewer) contiguous(t *testing.T) (first, n int) {
	t.Helper()
	ts := v.ticks()
	for i := 1; i < len(ts); i++ {
		if ts[i] != ts[i-1]+1 {
			t.Fatalf("viewer TICK-1 sequence breaks at index %d: %d then %d (%v)", i, ts[i-1], ts[i], ts)
		}
	}
	return ts[0], len(ts)
}

func (r *itRig) viewer(id string) *itViewer {
	r.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	first, _, err := websocket.Dial(ctx, r.relayURL(id), nil)
	if err != nil {
		r.t.Fatalf("viewer dial: %v", err)
	}
	v := &itViewer{}
	stdinR, stdinW := io.Pipe() // never written: the viewer only watches
	done := make(chan error, 1)
	go func() {
		done <- attach.Run(ctx, first, attach.RunOptions{
			URL: func() string { return r.relayURL(id) },
			OnState: func(s string) {
				if s == "reconnecting" {
					v.reconnects.Add(1)
				}
			},
			Stdin:     stdinR,
			Stdout:    v,
			DetachKey: 0x1C,
		})
	}()
	r.stops = append(r.stops, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		stdinW.Close()
	})
	return v
}

func itAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func itPpid(t *testing.T, pid int) int {
	t.Helper()
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return -1
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}
