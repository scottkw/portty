package daemon

// Phase 19 (DSHR-01): the daemon runs as Portty's own LaunchAgent so the
// always-available desktop survives a quit GUI and a crash. launchd is the
// primary start path; the detached spawn (startDetachedDaemon) is the
// fallback when the agent cannot be used, so terminals always start.
//
// Safety rule: nothing here may end a daemon that holds sessions. No kickstart
// kill flag anywhere, no bootout of a job whose process holds the daemon lock,
// and a changed plist is re-bootstrapped only while its job has no process.

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// LaunchdLabel is the job label and the plist's base name.
	LaunchdLabel = "app.portty.Portty.daemon"
	// LegacyLabel is the kardianos job `portty daemon install` used to write.
	LegacyLabel = "portty-daemon"
)

// Typed agent errors. Each message is the repair sentence the CLI prints and
// the desktop shows; none names a path. ErrAgentLaunchctl and ErrAgentRefused
// are wrapped with the underlying launchctl error.
var (
	ErrAgentUnsafePath = errors.New("Portty is running from a disk image. Move it to Applications, then open it again.")                                                  //nolint:staticcheck // user-facing sentence
	ErrAgentMissing    = errors.New("Portty's background service isn't installed. Open Portty.app from Applications once to install it.")                                 //nolint:staticcheck // user-facing sentence
	ErrAgentDisabled   = errors.New("Portty's background service is turned off in Login Items. Turn on Portty in System Settings, General, Login Items, then try again.") //nolint:staticcheck // user-facing sentence
	ErrAgentRefused    = errors.New("macOS didn't let Portty start its background service. Check Portty in System Settings, General, Login Items, then try again.")       //nolint:staticcheck // user-facing sentence
	ErrAgentLaunchctl  = errors.New("macOS didn't let Portty start its background service. Check Portty in System Settings, General, Login Items, then try again.")       //nolint:staticcheck // user-facing sentence
	ErrAgentRunning    = errors.New("Portty's background service is running. Run portty daemon stop first, then uninstall.")                                              //nolint:staticcheck // user-facing sentence
)

// launchctlFunc runs launchctl with args and returns its combined output.
type launchctlFunc func(ctx context.Context, args ...string) ([]byte, error)

// defaultLaunchctl runs /bin/launchctl. A test binary never reaches the
// user's launchd domain: it refuses unless a test injects a recorder.
var defaultLaunchctl launchctlFunc = func(ctx context.Context, args ...string) ([]byte, error) {
	if testing.Testing() {
		return nil, errors.New("refusing to run launchctl from a test binary")
	}
	return exec.CommandContext(ctx, "/bin/launchctl", args...).CombinedOutput()
}

var runLaunchctl = defaultLaunchctl

// SetLaunchctlForTest replaces the launchctl seam; restore puts it back.
func SetLaunchctlForTest(fn func(ctx context.Context, args ...string) ([]byte, error)) (restore func()) {
	old := runLaunchctl
	runLaunchctl = fn
	return func() { runLaunchctl = old }
}

func agentDomain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

func agentTarget(label string) string { return agentDomain() + "/" + label }

// lcFail describes a failed launchctl call, with its output when it has any.
func lcFail(op string, out []byte, err error) error {
	if s := strings.TrimSpace(string(out)); s != "" {
		err = fmt.Errorf("%w: %s", err, s)
	}
	return fmt.Errorf("launchctl %s: %w", op, err)
}

func wrapLaunchctl(err error) error { return fmt.Errorf("%w (%w)", ErrAgentLaunchctl, err) }

// RenderAgentPlist is the LaunchAgent plist for the daemon at exe. No
// ProcessType key (research R04) and no log paths (the daemon opens
// daemon.log itself). KeepAlive SuccessfulExit false restarts only a failed
// exit; Quit All and `portty daemon stop` exit 0 and stay down.
func RenderAgentPlist(exe string) ([]byte, error) {
	if !filepath.IsAbs(exe) {
		return nil, fmt.Errorf("launchd agent: executable %q is not absolute", exe)
	}
	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, []byte(exe)); err != nil {
		return nil, err
	}
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>AbandonProcessGroup</key>
	<true/>
</dict>
</plist>
`, LaunchdLabel, esc.String()), nil
}

// agentDir is ~/Library/LaunchAgents. A test binary refuses a home outside
// the OS temp dir, so no test can write the user's real LaunchAgents folder.
func agentDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if testing.Testing() {
		tmp := filepath.Clean(os.TempDir())
		if home != tmp && !strings.HasPrefix(home, tmp+string(filepath.Separator)) {
			return "", ErrAgentUnsafePath
		}
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

func plistPath(label string) (string, error) {
	dir, err := agentDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, label+".plist"), nil
}

// AgentPath is where the daemon's plist lives.
func AgentPath() (string, error) { return plistPath(LaunchdLabel) }

// unsafeExe reports an executable the agent must not point at: a mounted
// disk image or an App Translocation copy (the app is moved first).
func unsafeExe(exe string) bool {
	return strings.HasPrefix(exe, "/Volumes/") || strings.Contains(exe, "/AppTranslocation/")
}

var pidRE = regexp.MustCompile(`(?m)^\s*pid = (\d+)\s*$`)

// jobLoaded reports whether launchd has label loaded and its running pid (0
// when idle).
func jobLoaded(ctx context.Context, label string) (loaded bool, pid int, err error) {
	out, err := runLaunchctl(ctx, "print", agentTarget(label))
	if bytes.Contains(out, []byte("Could not find service")) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, lcFail("print", out, err)
	}
	if m := pidRE.FindSubmatch(out); m != nil {
		pid, _ = strconv.Atoi(string(m[1]))
	}
	return true, pid, nil
}

// AgentLoaded reports whether the daemon's job is loaded, whether the user
// disabled it in Login Items, and its running pid (0 when idle).
func AgentLoaded(ctx context.Context) (loaded, disabled bool, pid int, err error) {
	if loaded, pid, err = jobLoaded(ctx, LaunchdLabel); err != nil {
		return false, false, 0, err
	}
	out, err := runLaunchctl(ctx, "print-disabled", agentDomain())
	if err != nil {
		return false, false, 0, lcFail("print-disabled", out, err)
	}
	// "label" => disabled on current macOS; "label" => true on older formats.
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(LaunchdLabel) + `"\s*=>\s*(disabled|true)\b`)
	return loaded, re.Match(out), pid, nil
}

// AgentState is what EnsureAgent left behind.
type AgentState int

const (
	AgentReady          AgentState = iota
	AgentReloadDeferred            // the plist changed while the job runs; applied by a later call once it is idle
)

// writeAtomic writes data to path through a temp file in the same directory.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".portty-agent-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // a no-op after the rename
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(0o644)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// EnsureAgent writes the plist for exe when it is missing or different and
// bootstraps the job when launchd does not have it. A loaded job is
// re-bootstrapped for a changed plist only when it has no running process.
func EnsureAgent(ctx context.Context, exe string) (AgentState, error) {
	if unsafeExe(exe) {
		return AgentReady, ErrAgentUnsafePath
	}
	want, err := RenderAgentPlist(exe)
	if err != nil {
		return AgentReady, err
	}
	path, err := AgentPath()
	if err != nil {
		return AgentReady, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return AgentReady, err
	}
	have, _ := os.ReadFile(path)
	changed := !bytes.Equal(have, want)
	loaded, disabled, pid, err := AgentLoaded(ctx)
	if err != nil {
		return AgentReady, wrapLaunchctl(err)
	}
	// The file stays as it is while the job runs: launchd's loaded copy cannot
	// be read back, so a rewrite now would make a later call think nothing changed.
	// shortcut: a daemon that never goes idle never picks up a changed plist;
	// upgrade to comparing `launchctl print`'s program arguments if that bites.
	if changed && loaded && pid > 0 {
		return AgentReloadDeferred, nil
	}
	if changed {
		if err := writeAtomic(path, want); err != nil {
			return AgentReady, err
		}
	}
	if disabled {
		return AgentReady, ErrAgentDisabled
	}
	switch {
	case !loaded:
		return AgentReady, bootstrapAgent(ctx, path)
	case changed:
		if out, err := runLaunchctl(ctx, "bootout", agentTarget(LaunchdLabel)); err != nil {
			return AgentReady, fmt.Errorf("%w (%w)", ErrAgentRefused, lcFail("bootout", out, err))
		}
		return AgentReady, bootstrapAgent(ctx, path)
	}
	return AgentReady, nil
}

func bootstrapAgent(ctx context.Context, path string) error {
	if out, err := runLaunchctl(ctx, "bootstrap", agentDomain(), path); err != nil {
		return fmt.Errorf("%w (%w)", ErrAgentRefused, lcFail("bootstrap", out, err))
	}
	return nil
}

// Kickstart starts the daemon's job. Never with the kill flag: that would end
// a running daemon and every session it holds. Plain kickstart leaves a
// running job alone (research A3, proven in the VM by 19-07).
func Kickstart(ctx context.Context) error {
	if out, err := runLaunchctl(ctx, "kickstart", agentTarget(LaunchdLabel)); err != nil {
		return wrapLaunchctl(lcFail("kickstart", out, err))
	}
	return nil
}

// agentStartable returns why launchd cannot start the daemon for the
// executable at exe (a typed error), or nil when a kickstart is worth trying.
// A job that is not loaded but whose plist is installed is bootstrapped; no
// plist is never written here (only the GUI installs it, EnsureAgent).
func agentStartable(ctx context.Context, exe string) error {
	if unsafeExe(exe) {
		return ErrAgentUnsafePath
	}
	loaded, disabled, _, err := AgentLoaded(ctx)
	switch {
	case err != nil:
		return wrapLaunchctl(err)
	case disabled:
		return ErrAgentDisabled
	case loaded:
		return nil
	}
	path, err := AgentPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return ErrAgentMissing
	}
	return bootstrapAgent(ctx, path)
}

// ManagedByLaunchd reports whether this process is the daemon's launchd job:
// launchd names the job in XPC_SERVICE_NAME (research A4), else the job's pid
// is ours.
func ManagedByLaunchd(ctx context.Context) bool {
	if os.Getenv("XPC_SERVICE_NAME") == LaunchdLabel {
		return true
	}
	_, pid, err := jobLoaded(ctx, LaunchdLabel)
	return err == nil && pid == os.Getpid()
}

// LegacyCleanup removes the kardianos agent `portty daemon install` used to
// write (label portty-daemon). Its plist file always goes; its job is booted
// out only when its process is not the lock holder, since bootout SIGTERMs
// that process and a daemon holding the lock holds every terminal session
// (research Pitfall 5). An unknown holder (lockHolderPID <= 0) counts as the
// holder. done is true once the job and the file are both gone; false means
// call again at the next start, when the holder may have changed.
func LegacyCleanup(ctx context.Context, lockHolderPID int) (done bool, err error) {
	path, err := plistPath(LegacyLabel)
	if err != nil {
		return false, err
	}
	loaded, pid, err := jobLoaded(ctx, LegacyLabel)
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if pid > 0 && (pid == lockHolderPID || lockHolderPID <= 0) {
		return false, nil
	}
	if loaded {
		if out, err := runLaunchctl(ctx, "bootout", agentTarget(LegacyLabel)); err != nil {
			return false, lcFail("bootout", out, err)
		}
	}
	return true, nil
}

// launchctlTimeout bounds one `portty daemon <action>`.
const launchctlTimeout = 10 * time.Second

// AgentControl is `portty daemon install|uninstall|start|stop`, on Portty's
// own LaunchAgent. stop is a SIGTERM to the job's process: the daemon exits 0
// and KeepAlive SuccessfulExit false leaves it down.
func AgentControl(action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), launchctlTimeout)
	defer cancel()
	switch action {
	case "install":
		exe, err := executablePath()
		if err != nil {
			return err
		}
		_, err = EnsureAgent(ctx, exe)
		return err
	case "uninstall":
		return uninstallAgent(ctx)
	case "start":
		return Kickstart(ctx)
	case "stop":
		if out, err := runLaunchctl(ctx, "kill", "SIGTERM", agentTarget(LaunchdLabel)); err != nil {
			return wrapLaunchctl(lcFail("kill", out, err))
		}
		return nil
	}
	return fmt.Errorf("unknown daemon action %q", action)
}

// uninstallAgent boots the idle job out and removes both plists. A running
// job is refused: ending it would end every session its daemon holds.
func uninstallAgent(ctx context.Context) error {
	loaded, pid, err := jobLoaded(ctx, LaunchdLabel)
	if err != nil {
		return wrapLaunchctl(err)
	}
	if pid > 0 {
		return ErrAgentRunning
	}
	if loaded {
		if out, err := runLaunchctl(ctx, "bootout", agentTarget(LaunchdLabel)); err != nil {
			return wrapLaunchctl(lcFail("bootout", out, err))
		}
	}
	path, err := AgentPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err = LegacyCleanup(ctx, 0)
	return err
}
