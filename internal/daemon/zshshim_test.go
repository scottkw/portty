//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/relay"
)

const shimWait = 15 * time.Second

func requireZsh(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/zsh"); err != nil {
		t.Skip("/bin/zsh not present")
	}
}

// markerFiles returns the four user startup files; each appends its own name
// to orderLog, and the .zshrc exists so zsh-newuser-install stays quiet.
func markerFiles(orderLog string) map[string]string {
	m := map[string]string{}
	for _, n := range []string{".zshenv", ".zprofile", ".zshrc", ".zlogin"} {
		m[n] = "print -r -- " + n + " >> '" + orderLog + "'\n"
	}
	return m
}

// shimHome makes a temp HOME holding files (name -> content), points HOME at
// it, and unsets any ZDOTDIR the test binary inherited. It returns the
// resolved HOME path.
func shimHome(t *testing.T, files map[string]string) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "") // registers the restore of the inherited value
	if err := os.Unsetenv("ZDOTDIR"); err != nil {
		t.Fatal(err)
	}
	return home
}

// newShimEngine returns an engine that runs the real /bin/zsh -l with the
// shim installed under a config dir whose name has a space, like
// "Application Support"; its sessions are killed in cleanup.
func newShimEngine(t *testing.T) (*SessionEngine, string) {
	t.Helper()
	requireZsh(t)
	e := NewSessionEngine()
	cfg := filepath.Join(t.TempDir(), "Application Support", "portty")
	if err := os.MkdirAll(cfg, 0700); err != nil {
		t.Fatal(err)
	}
	e.ConfigDirForTest(cfg)
	t.Cleanup(func() { killAllSessions(e) })
	return e, cfg
}

// startShimSession starts a session in workDir and returns its id.
func startShimSession(t *testing.T, e *SessionEngine, workDir string) string {
	t.Helper()
	id, err := e.CreateSession(context.Background(), "shim", workDir, 300, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return id
}

// typeLine types line (a newline is appended) into the session.
func typeLine(t *testing.T, e *SessionEngine, id, line string) {
	t.Helper()
	hub, ok := e.manager.Get(id)
	if !ok {
		t.Fatal("hub not found")
	}
	if err := hub.OpenInput(nil).Write([]byte(line + "\n")); err != nil {
		t.Fatalf("input Write: %v", err)
	}
}

// rawOutput is everything the session printed so far, escape bytes included.
func rawOutput(t *testing.T, e *SessionEngine, id string) string {
	t.Helper()
	hub, ok := e.manager.Get(id)
	if !ok {
		t.Fatal("hub not found")
	}
	var b strings.Builder
	for _, c := range hub.ScrollbackSnapshot() {
		if c != relay.MsgOutput {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// waitShimOutput fails the test unless marker shows up in the session output.
func waitShimOutput(t *testing.T, e *SessionEngine, id, marker string) {
	t.Helper()
	if !hubReadUntil(t, e, id, marker, shimWait) {
		t.Fatalf("%q never appeared; output so far: %q", marker, rawOutput(t, e, id))
	}
}

// syncShell types cmd followed by a marker that only real output can contain
// (the typed echo shows the unexpanded $((…)) form), waits for it and returns
// the raw output so far. tag must be unique per call in one session.
func syncShell(t *testing.T, e *SessionEngine, id, cmd, tag string) string {
	t.Helper()
	typeLine(t, e, id, cmd+"; print -r -- "+tag+"_$((40+2))")
	waitShimOutput(t, e, id, tag+"_42")
	return rawOutput(t, e, id)
}

func readOrderLog(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("order.log: %v", err)
	}
	return strings.Fields(string(b))
}

// A new Portty zsh session reports its working directory with OSC 7 with no
// setup by the user, and the user's startup files run as without Portty.
func TestZshShim_SessionReportsCwd(t *testing.T) {
	requireZsh(t)
	orderLog := filepath.Join(t.TempDir(), "order.log")
	home := shimHome(t, markerFiles(orderLog))
	e, cfg := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := syncShell(t, e, id, `print -r -- "Z=[$ZDOTDIR] P=[${PORTTY_ZSH_ZDOTDIR-unset}]"`, "VARS")
	if !strings.Contains(out, "\x1b]7;file:///") {
		t.Errorf("no OSC 7 report at startup; output %q", out)
	}
	if !strings.Contains(out, "Z=[] P=[unset]") {
		t.Errorf("ZDOTDIR/PORTTY_ZSH_ZDOTDIR not restored inside the shell; output %q", out)
	}

	out = syncShell(t, e, id, "cd /usr", "CDUSR")
	if !strings.Contains(out, "\x1b]7;file:///usr\x07") {
		t.Errorf("cd /usr did not report /usr; output %q", out)
	}

	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zshrc .zlogin" {
		t.Errorf("startup order = %q", got)
	}

	shimDir := filepath.Join(cfg, "zsh")
	if fi, err := os.Stat(shimDir); err != nil || fi.Mode().Perm() != 0700 {
		t.Errorf("shim dir mode = %v (%v), want 0700", fi, err)
	}
	f := filepath.Join(shimDir, ".zshenv")
	if fi, err := os.Stat(f); err != nil || fi.Mode().Perm() != 0600 {
		t.Errorf(".zshenv mode = %v (%v), want 0600", fi, err)
	}
	if b, err := os.ReadFile(f); err != nil || string(b) != zshShim {
		t.Errorf(".zshenv differs from the embedded script (%v)", err)
	}
}

const osc7Home = "\x1b]7;file:///"

// markersIn writes the four marker startup files into dir.
func markersIn(t *testing.T, dir, orderLog string) {
	t.Helper()
	for name, content := range markerFiles(orderLog) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// printVars prints $ZDOTDIR, $HISTFILE and the shim variable behind a marker.
func printVars(t *testing.T, e *SessionEngine, id, tag string) string {
	t.Helper()
	return syncShell(t, e, id, `print -r -- "Z=[$ZDOTDIR] H=[$HISTFILE] P=[${PORTTY_ZSH_ZDOTDIR-unset}]"`, tag)
}

// A custom ZDOTDIR exported to the daemon: its four files run, HISTFILE
// follows it, and OSC 7 is still emitted.
func TestZshShim_CustomZDOTDIR(t *testing.T) {
	requireZsh(t)
	home := shimHome(t, nil)
	zd := t.TempDir()
	orderLog := filepath.Join(t.TempDir(), "order.log")
	markersIn(t, zd, orderLog)
	t.Setenv("ZDOTDIR", zd)
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := printVars(t, e, id, "CUSTOM")
	if want := "Z=[" + zd + "] H=[" + zd + "/.zsh_history] P=[unset]"; !strings.Contains(out, want) {
		t.Errorf("want %q in output %q", want, out)
	}
	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zshrc .zlogin" {
		t.Errorf("startup order = %q", got)
	}
	if !strings.Contains(out, osc7Home) {
		t.Errorf("no OSC 7 report; output %q", out)
	}
}

// A user ~/.zshenv that sets ZDOTDIR itself: the later files come from there.
func TestZshShim_ZDOTDIRSetInUserZshenv(t *testing.T) {
	requireZsh(t)
	other := t.TempDir()
	orderLog := filepath.Join(t.TempDir(), "order.log")
	home := shimHome(t, map[string]string{
		".zshenv": "print -r -- .zshenv >> '" + orderLog + "'\nexport ZDOTDIR='" + other + "'\n",
	})
	markersIn(t, other, orderLog)
	if err := os.Remove(filepath.Join(other, ".zshenv")); err != nil {
		t.Fatal(err)
	}
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := printVars(t, e, id, "ZSETZ")
	if want := "Z=[" + other + "] H=[" + other + "/.zsh_history]"; !strings.Contains(out, want) {
		t.Errorf("want %q in output %q", want, out)
	}
	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zshrc .zlogin" {
		t.Errorf("startup order = %q", got)
	}
}

// A user ~/.zshenv with a syntax error or a `return 1` neither hides the rest
// of the startup files nor the OSC 7 hooks.
func TestZshShim_BrokenUserZshenv(t *testing.T) {
	for name, zshenv := range map[string]string{
		"syntax-error": "echo (\n",
		"return-1":     "return 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			requireZsh(t)
			orderLog := filepath.Join(t.TempDir(), "order.log")
			files := markerFiles(orderLog)
			files[".zshenv"] = zshenv
			home := shimHome(t, files)
			e, _ := newShimEngine(t)
			id := startShimSession(t, e, home)

			syncShell(t, e, id, "true", "BROKENA")
			out := syncShell(t, e, id, "cd /usr", "BROKENB")
			if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zprofile .zshrc .zlogin" {
				t.Errorf("startup order = %q", got)
			}
			if !strings.Contains(out, "\x1b]7;file:///usr\x07") {
				t.Errorf("cd /usr did not report; output %q", out)
			}
		})
	}
}

// Reports go to /dev/tty from the parent shell only: a subshell or command
// substitution reports nothing and captures no escape bytes; a redirect on
// the cd does not hide the report.
func TestZshShim_SubshellAndRedirect(t *testing.T) {
	requireZsh(t)
	home := shimHome(t, markerFiles(filepath.Join(t.TempDir(), "order.log")))
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	syncShell(t, e, id, "x=$(cd /tmp && pwd)", "SUBA")
	out := syncShell(t, e, id, `print -r -- "X=[$x]"`, "SUBB")
	if !strings.Contains(out, "X=[/tmp]") {
		t.Errorf("command substitution captured something else; output %q", out)
	}
	out = syncShell(t, e, id, "(cd /usr)", "SUBC")
	if strings.Contains(out, "file:///usr") || strings.Contains(out, "file:///tmp") {
		t.Errorf("a subshell cd reported; output %q", out)
	}
	out = syncShell(t, e, id, "cd /tmp >/dev/null", "SUBD")
	if !strings.Contains(out, "\x1b]7;file:///tmp\x07") {
		t.Errorf("redirected cd did not report /tmp; output %q", out)
	}
}

// A user .zshrc that empties chpwd_functions gets reporting back at the next
// prompt (the precmd hook re-adds it).
func TestZshShim_RcResetsChpwd(t *testing.T) {
	requireZsh(t)
	orderLog := filepath.Join(t.TempDir(), "order.log")
	files := markerFiles(orderLog)
	files[".zshrc"] += "chpwd_functions=()\n"
	home := shimHome(t, files)
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	syncShell(t, e, id, "true", "RCA")
	out := syncShell(t, e, id, "cd /usr", "RCB")
	if !strings.Contains(out, "\x1b]7;file:///usr\x07") {
		t.Errorf("cd /usr did not report after the rc emptied chpwd_functions; output %q", out)
	}
}

// A nested interactive zsh has no hooks (the shim unset its own variables)
// but runs the user's files; a non-interactive login zsh emits no OSC 7.
func TestZshShim_NestedAndNonInteractive(t *testing.T) {
	requireZsh(t)
	orderLog := filepath.Join(t.TempDir(), "order.log")
	home := shimHome(t, markerFiles(orderLog))
	e, cfg := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := syncShell(t, e, id, `/bin/zsh -i -c 'print -r -- "N=[${+functions[__portty_osc7]}]"'`, "NESTED")
	if !strings.Contains(out, "N=[0]") {
		t.Errorf("nested shell has the hook function; output %q", out)
	}
	n := 0
	for _, f := range readOrderLog(t, orderLog) {
		if f == ".zshrc" {
			n++
		}
	}
	if n != 2 {
		t.Errorf(".zshrc ran %d times, want 2 (session plus nested shell)", n)
	}

	if err := os.WriteFile(orderLog, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/zsh", "-l", "-c", "cd /usr")
	cmd.Env = []string{
		"PATH=/usr/bin:/bin", "HOME=" + home,
		"ZDOTDIR=" + filepath.Join(cfg, "zsh"), "PORTTY_ZSH_ZDOTDIR=",
	}
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh -l -c: %v (output %q)", err, got)
	}
	if bytes.Contains(got, []byte("\x1b]7")) {
		t.Errorf("non-interactive shell emitted OSC 7: %q", got)
	}
	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zlogin" {
		t.Errorf("non-interactive startup order = %q", got)
	}
}

// A stale PORTTY_ZSH_ZDOTDIR inherited by the daemon is overwritten, not honoured.
func TestZshShim_StaleInheritedVar(t *testing.T) {
	requireZsh(t)
	orderLog := filepath.Join(t.TempDir(), "order.log")
	home := shimHome(t, markerFiles(orderLog))
	t.Setenv("PORTTY_ZSH_ZDOTDIR", "/nonexistent-stale")
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := printVars(t, e, id, "STALE")
	if !strings.Contains(out, "Z=[] H=["+home+"/.zsh_history] P=[unset]") {
		t.Errorf("ZDOTDIR not empty inside the shell; output %q", out)
	}
	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zshrc .zlogin" {
		t.Errorf("startup order = %q", got)
	}
}

// A HOME with only an empty .zshrc starts without any zsh error line.
func TestZshShim_CleanStartupNoErrors(t *testing.T) {
	requireZsh(t)
	home := shimHome(t, map[string]string{".zshrc": ""})
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := syncShell(t, e, id, "true", "CLEAN")
	if strings.Contains(out, "zsh:") || strings.Contains(out, ".zshenv:") {
		t.Errorf("startup printed an error; output %q", out)
	}
}

// The daemon never touches the user's own startup files or HOME.
func TestZshShim_UserFilesUntouched(t *testing.T) {
	requireZsh(t)
	files := markerFiles(filepath.Join(t.TempDir(), "order.log"))
	home := shimHome(t, files)
	type snap struct {
		data string
		mode os.FileMode
	}
	before := map[string]snap{}
	for name := range files {
		fi, err := os.Stat(filepath.Join(home, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = snap{files[name], fi.Mode()}
	}
	e, _ := newShimEngine(t)
	id := startShimSession(t, e, home)
	syncShell(t, e, id, "cd /usr", "UNTOUCHED")

	for name, b := range before {
		got, err := os.ReadFile(filepath.Join(home, name))
		fi, err2 := os.Stat(filepath.Join(home, name))
		if err != nil || err2 != nil || string(got) != b.data || fi.Mode() != b.mode {
			t.Errorf("%s changed (%v %v)", name, err, err2)
		}
	}
	ents, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range ents {
		// Library: NewSessionEngine creates its own config dir under HOME.
		if _, ok := before[en.Name()]; !ok && en.Name() != ".zsh_history" && en.Name() != ".zsh_sessions" && en.Name() != "Library" {
			t.Errorf("unexpected new entry in HOME: %s", en.Name())
		}
	}
}
