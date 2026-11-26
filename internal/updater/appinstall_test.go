package updater

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// aiErr fails t unless err wraps want (nil: no error) and its text ends with the step name.
func aiErr(t *testing.T, name string, err, want error, step string) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Errorf("%s: err = %v, want nil", name, err)
		}
		return
	}
	if !errors.Is(err, want) || !errors.Is(err, ErrInstall) || !strings.HasSuffix(err.Error(), step) {
		t.Errorf("%s: err = %v, want %v at step %s", name, err, want, step)
	}
	if err != nil && strings.Contains(err.Error(), "/") {
		t.Errorf("%s: error holds a path: %v", name, err)
	}
}

func TestPreflight(t *testing.T) {
	dir := t.TempDir()
	good := siBundle(t, dir, "1.0.0")
	link := filepath.Join(t.TempDir(), BundleName)
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), BundleName)
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ro := t.TempDir()
	roApp := siBundle(t, ro, "1.0.0")
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	for _, c := range []struct {
		name, target string
		want         error
	}{
		{"writable bundle", good, nil},
		{"not yet there", filepath.Join(t.TempDir(), BundleName), nil},
		{"other name", filepath.Join(dir, "Other.app"), ErrLocation},
		{"name prefix", filepath.Join(dir, "Portty.app2"), ErrLocation},
		{"relative", "Applications/Portty.app", ErrLocation},
		{"unclean", dir + "/./" + BundleName, ErrLocation},
		{"trailing slash", good + "/", ErrLocation},
		{"empty", "", ErrLocation},
		{"volume", "/Volumes/Portty/Portty.app", ErrLocation},
		{"translocated", "/private/var/folders/xy/T/AppTranslocation/ABCD/d/Portty.app", ErrLocation},
		{"symlink", link, ErrLocation},
		{"plain file", file, ErrLocation},
	} {
		aiErr(t, c.name, Preflight(c.target), c.want, "preflight")
	}
	if os.Getuid() != 0 {
		aiErr(t, "read-only folder", Preflight(roApp), ErrNoWrite, "preflight")
	}
}

func aiFree(t *testing.T, free uint64, err error) {
	t.Helper()
	old := freeBytes
	freeBytes = func(string) (uint64, error) { return free, err }
	t.Cleanup(func() { freeBytes = old })
}

func TestCheckSpace(t *testing.T) {
	aiFree(t, 1000, nil)
	aiErr(t, "exactly need", CheckSpace(t.TempDir(), 1000), nil, "")
	aiErr(t, "below need", CheckSpace(t.TempDir(), 1001), ErrNoSpace, "space")
	aiFree(t, 0, errors.New("statfs failed"))
	if err := CheckSpace(t.TempDir(), 1); err == nil || errors.Is(err, ErrNoSpace) || !errors.Is(err, ErrInstall) {
		t.Errorf("statfs failure: err = %v, want a plain install error (not the disk-space text)", err)
	}
}

func TestFreeBytesAndAppSize(t *testing.T) {
	if n, err := freeBytes(t.TempDir()); err != nil || n == 0 {
		t.Errorf("freeBytes = %d, %v", n, err)
	}
	app := siBundle(t, t.TempDir(), "12345") // a 5-byte marker
	if err := os.WriteFile(filepath.Join(app, BundleExecutable), make([]byte, 100), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/bin/true", filepath.Join(app, "link")); err != nil { // not followed
		t.Fatal(err)
	}
	if n, err := AppSize(app); err != nil || n != 105 {
		t.Errorf("AppSize = %d, %v; want 105", n, err)
	}
	if _, err := AppSize(filepath.Join(t.TempDir(), "missing.app")); err == nil {
		t.Error("AppSize of a missing app: no error")
	}
}

// aiDVV is a runner whose codesign -dvv answers out; every other call succeeds silently.
func aiDVV(out string, verifyErr error) Runner {
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "-dvv") {
			return []byte(out), nil
		}
		return nil, verifyErr
	}
}

func TestVerifyApp(t *testing.T) {
	const team = "TeamIdentifier=" + TeamID
	for _, c := range []struct {
		name, dvv string
		verify    error
		want      error
		step      string
	}{
		{"good", "Identifier=" + AppIdentifier + "\n" + team + "\n", nil, nil, ""},
		{"surrounding whitespace", "  Identifier=" + AppIdentifier + " \r\n\t" + team + "\n", nil, nil, ""},
		{"codesign fails", "Identifier=" + AppIdentifier + "\n" + team, errors.New("exit status 1"), ErrSafety, "app-codesign"},
		{"other team", "Identifier=" + AppIdentifier + "\nTeamIdentifier=ABCDE12345", nil, ErrSafety, "app-team"},
		{"team with extra", "Identifier=" + AppIdentifier + "\n" + team + "0", nil, ErrSafety, "app-team"},
		{"no identifier", team, nil, ErrSafety, "app-identifier"},
		{"identifier with extra", "Identifier=" + AppIdentifier + ".helper\n" + team, nil, ErrSafety, "app-identifier"},
		{"other identifier", "Identifier=com.example.Other\n" + team, nil, ErrSafety, "app-identifier"},
	} {
		aiErr(t, c.name, VerifyApp(context.Background(), aiDVV(c.dvv, c.verify), "/x/Portty.app"), c.want, c.step)
	}
	if AppIdentifier != "app.portty.Portty" { // the P7 row of 18-VM-CHECKS.md
		t.Errorf("AppIdentifier = %q", AppIdentifier)
	}
}

// aiApp is a staged app as CheckStagedApp sees it through the runner.
type aiApp struct {
	spctl, version, minOS, sysOS, smoke string
	spctlErr, minErr, smokeErr          error
	smokeLeft                           time.Duration // time left on the smoke run's deadline
}

func aiGood() *aiApp {
	return &aiApp{spctl: "/x/Portty.app: accepted\nsource=Notarized Developer ID\n", version: "9.9.9\n",
		minOS: "14.0\n", sysOS: "14.9\n", smoke: "v9.9.9\n"}
}

func (a *aiApp) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	switch {
	case name == "/usr/sbin/spctl" && slices.Equal(args[:5], []string{"-a", "-t", "exec", "-vv", "/x/Portty.app"}):
		return []byte(a.spctl), a.spctlErr
	case name == "/usr/bin/plutil" && slices.Equal(args, []string{"-extract", "CFBundleShortVersionString", "raw", "/x/Portty.app/Contents/Info.plist"}):
		return []byte(a.version), nil
	case name == "/usr/bin/plutil" && slices.Equal(args, []string{"-extract", "LSMinimumSystemVersion", "raw", "/x/Portty.app/Contents/Info.plist"}):
		return []byte(a.minOS), a.minErr
	case name == "/usr/bin/sw_vers" && slices.Equal(args, []string{"-productVersion"}):
		return []byte(a.sysOS), nil
	case name == "/x/Portty.app/"+BundleExecutable && slices.Equal(args, []string{"--version"}):
		if d, ok := ctx.Deadline(); ok {
			a.smokeLeft = time.Until(d)
		}
		return []byte(a.smoke), a.smokeErr
	}
	return nil, errors.New("aiApp: unexpected call " + name + " " + strings.Join(args, " "))
}

func TestCheckStagedApp(t *testing.T) {
	noKey := errors.New("exit status 1")
	for _, c := range []struct {
		name string
		edit func(a *aiApp)
		step string // "" passes
	}{
		{"good", func(*aiApp) {}, ""},
		{"whitespace around values", func(a *aiApp) { a.version, a.smoke = " 9.9.9 \n", "\tv9.9.9\r\n" }, ""},
		{"no min key", func(a *aiApp) {
			a.minOS, a.minErr = "/x/Portty.app/Contents/Info.plist: Could not extract value, error: No value at that key path or invalid key path: LSMinimumSystemVersion\n", noKey
		}, ""},
		{"min equal", func(a *aiApp) { a.minOS = "14.9" }, ""},
		{"min shorter", func(a *aiApp) { a.minOS, a.sysOS = "14", "14.0.1" }, ""},
		{"spctl rejected", func(a *aiApp) { a.spctl = "/x/Portty.app: rejected\nsource=Notarized Developer ID\n" }, "app-spctl"},
		{"spctl not notarized", func(a *aiApp) { a.spctl = "/x/Portty.app: accepted\nsource=Developer ID\n" }, "app-spctl"},
		{"spctl notarized prefix", func(a *aiApp) { a.spctl = "/x/Portty.app: accepted\nsource=Notarized Developer IDx\n" }, "app-spctl"},
		{"spctl fails", func(a *aiApp) { a.spctlErr = errors.New("exit status 3") }, "app-spctl"},
		{"older version", func(a *aiApp) { a.version = "9.9.8" }, "app-version"},
		{"version containing", func(a *aiApp) { a.version = "9.9.90" }, "app-version"},
		{"version v prefix", func(a *aiApp) { a.version = "v9.9.9" }, "app-version"},
		{"version empty", func(a *aiApp) { a.version = "" }, "app-version"},
		{"needs newer macOS", func(a *aiApp) { a.minOS = "14.10" }, "app-min-os"},
		{"needs newer major", func(a *aiApp) { a.minOS, a.sysOS = "28.0", "27.2.1" }, "app-min-os"},
		{"min unparsable", func(a *aiApp) { a.minOS = "fourteen" }, "app-min-os"},
		{"min read fails", func(a *aiApp) { a.minOS, a.minErr = "plutil: error", noKey }, "app-min-os"},
		{"system unparsable", func(a *aiApp) { a.sysOS = "" }, "app-min-os"},
		{"smoke wrong version", func(a *aiApp) { a.smoke = "v9.9.8\n" }, "app-smoke"},
		{"smoke containing", func(a *aiApp) { a.smoke = "v9.9.9-rc\n" }, "app-smoke"},
		{"smoke no v", func(a *aiApp) { a.smoke = "9.9.9\n" }, "app-smoke"},
		{"smoke fails", func(a *aiApp) { a.smokeErr = errors.New("exit status 1") }, "app-smoke"},
		{"smoke no answer", func(a *aiApp) { a.smoke, a.smokeErr = "", context.DeadlineExceeded }, "app-smoke"},
	} {
		a := aiGood()
		c.edit(a)
		err := CheckStagedApp(context.Background(), a.run, "/x/Portty.app", "9.9.9")
		if c.step == "" {
			aiErr(t, c.name, err, nil, "")
		} else {
			aiErr(t, c.name, err, ErrSafety, c.step)
		}
		if c.step == "" || c.step == "app-smoke" {
			if a.smokeLeft <= 0 || a.smokeLeft > SmokeTimeout {
				t.Errorf("%s: smoke run had %v left, want its own deadline of at most %v", c.name, a.smokeLeft, SmokeTimeout)
			}
		}
	}
	// The version must equal the release exactly: 1.8.1 is not 1.8.10 nor v1.8.1-rc.
	for _, v := range []string{"1.8.10", "v1.8.1-rc"} {
		a := aiGood()
		a.version, a.smoke = v, "v"+v
		aiErr(t, "release 1.8.1 vs "+v, CheckStagedApp(context.Background(), a.run, "/x/Portty.app", "1.8.1"), ErrSafety, "app-version")
	}
}

func aiMkdir(t *testing.T, err error) {
	t.Helper()
	old := mkdirTemp
	mkdirTemp = func(string, string) (string, error) { return "", &fs.PathError{Op: "mkdirtemp", Path: "/x", Err: err} }
	t.Cleanup(func() { mkdirTemp = old })
}

func TestCopyApp(t *testing.T) {
	src := siBundle(t, t.TempDir(), "9.9.9")
	target := filepath.Join(t.TempDir(), BundleName)
	ditto := func(out string) Runner {
		return func(context.Context, string, ...string) ([]byte, error) {
			return []byte(out), errors.New("exit status 1")
		}
	}
	for _, c := range []struct {
		name, out string
		want      error
	}{
		{"disk full", "ditto: /x/Portty.app/y: No space left on device\n", ErrNoSpace},
		{"not permitted", "ditto: /x/Portty.app: Operation not permitted\n", ErrAppManagement},
		{"other", "ditto: something else\n", ErrCopy},
	} {
		staged, err := CopyApp(context.Background(), ditto(c.out), src, target)
		aiErr(t, "ditto "+c.name, err, c.want, "copy")
		if staged != "" {
			t.Errorf("%s: staged = %q", c.name, staged)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(target), StagingPrefix+"*")); len(m) != 0 {
		t.Errorf("staging folders left after ditto failures: %v", m)
	}
	ok := func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	for _, c := range []struct {
		name string
		err  error
		want error
	}{
		{"EACCES", syscall.EACCES, ErrNoWrite},
		{"EPERM", syscall.EPERM, ErrAppManagement}, // the parent is writable: App Management (A2)
		{"other", syscall.EIO, ErrCopy},
	} {
		aiMkdir(t, c.err)
		_, err := CopyApp(context.Background(), ok, src, target)
		aiErr(t, "mkdir "+c.name, err, c.want, "copy")
	}
}

func TestInstallFailureText(t *testing.T) {
	cases := map[error]string{
		ErrNoWrite:        "Portty doesn't have permission to replace itself from this account.",
		ErrAppManagement:  "macOS didn't allow Portty to replace itself.",
		ErrMount:          "Portty couldn't open the download.",
		ErrCopy:           "Portty couldn't copy the new version.",
		ErrNoSpace:        "There isn't enough free disk space to install the update.",
		ErrSafety:         "The new version didn't pass the safety check, so it wasn't installed.",
		ErrLocation:       "Portty can't update itself from where it's running. Move Portty to the Applications folder first.",
		ErrDetach:         "Portty couldn't eject the download.",
		errInstallerStart: "Portty couldn't start the installer.",
		ErrInstall:        "Portty couldn't install the update.",
		errors.New("x"):   "Portty couldn't install the update.",
	}
	for err, want := range cases {
		wrapped := installErr(err, "step")
		for _, revealed := range []bool{true, false} {
			suffix := " Try again later."
			if revealed {
				suffix = " The download is shown in Finder."
			}
			got := installFailureText(wrapped, revealed)
			if got != want+suffix {
				t.Errorf("installFailureText(%v, %v) = %q, want %q", wrapped, revealed, got, want+suffix)
			}
			if strings.Contains(got, "/") || strings.Contains(got, "://") {
				t.Errorf("%q holds a slash", got)
			}
		}
	}
}

// aiInstallFailure drives one Download into a self-install that fails in mode, on a Service with
// its Bundle in t.TempDir() and an in-process StartInstaller (newSiFix).
func aiInstallFailure(t *testing.T, mode string) (*siFix, Status, error) {
	t.Helper()
	oldPause := detachPause
	detachPause = 0
	t.Cleanup(func() { detachPause = oldPause })
	f := newSiFix(t)
	switch mode {
	case "mount":
		f.rel.fail = "hdiutil"
	case "copy":
		f.rel.fail, f.rel.failOut = "ditto", "ditto: copy failed\n"
	case "space":
		aiFree(t, 1, nil)
	case "permission":
		if err := os.Chmod(f.root, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(f.root, 0o700) })
	case "app management":
		f.rel.fail, f.rel.failOut = "ditto", "ditto: Portty.app: Operation not permitted\n"
	case "safety":
		f.rel.fail, f.rel.failOut = "smoke", "dyld: crashed\n"
	case "location":
		f.svc.Bundle = filepath.Join(f.root, "Portty 2.app")
	case "start":
		f.startErr = errors.New("spawn failed")
	case "eject":
		f.rel.fail = "detach"
	default:
		t.Fatalf("unknown mode %s", mode)
	}
	if _, err := f.svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Download()
	return f, st, err
}

// aiInstallTexts is every install failure mode with the text its banner shows.
var aiInstallTexts = map[string]error{
	"mount": ErrMount, "copy": ErrCopy, "space": ErrNoSpace, "permission": ErrNoWrite,
	"app management": ErrAppManagement, "safety": ErrSafety, "location": ErrLocation,
	"start": errInstallerStart, "eject": ErrDetach,
}

func TestService_InstallFailuresReveal(t *testing.T) {
	for mode, cause := range aiInstallTexts {
		t.Run(mode, func(t *testing.T) {
			if mode == "permission" && os.Getuid() == 0 {
				t.Skip("root can write anywhere")
			}
			f, st, err := aiInstallFailure(t, mode)
			want := installFailureText(cause, true)
			if err != nil || st.State != "revealed" || st.Error != want {
				t.Errorf("Download = %+v, %v; want revealed with %q", st, err, want)
			}
			if n := f.rel.called("/usr/bin/open", "-R"); n != 1 {
				t.Errorf("Reveal ran %d times", n)
			}
			if mode != "start" && f.started != 0 {
				t.Errorf("StartInstaller ran %d times", f.started)
			}
			if siMarker(t, f.target) != "1.0.0" {
				t.Error("installed app changed")
			}
			if left := siLeftovers(f); len(left) != 0 {
				t.Errorf("left behind: %v", left)
			}
		})
	}
}

// An app that fails the safety check is never swapped in or launched, and is never run before its
// signature passed.
func TestService_SafetyFailureNeverLaunches(t *testing.T) {
	for _, fail := range []string{"dvv-app", "smoke"} {
		f, st, _ := func() (*siFix, Status, error) {
			if fail == "smoke" {
				return aiInstallFailure(t, "safety")
			}
			f := newSiFix(t)
			f.rel.fail = fail
			st := f.download(t)
			return f, st, nil
		}()
		if st.Error != installFailureText(ErrSafety, true) || f.started != 0 || len(f.opened) != 0 {
			t.Errorf("%s: status %+v, installer started %d, opened %v", fail, st, f.started, f.opened)
		}
		if fail == "dvv-app" && f.rel.called("--version") != 0 {
			t.Errorf("an app that failed its signature check was run: %v", f.rel.recorded())
		}
	}
}

func TestService_FallbackWithoutDMG(t *testing.T) {
	f := newSiFix(t)
	f.rel.fail = "hdiutil"
	run := f.svc.Run
	f.svc.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if strings.HasSuffix(name, "hdiutil") && args[0] == "attach" {
			_ = os.Remove(args[len(args)-1]) // the DMG vanishes mid-install
		}
		return run(ctx, name, args...)
	}
	if _, err := f.svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Download()
	want := installFailureText(ErrMount, false)
	if st.State != "available" || st.Error != want || !strings.HasSuffix(st.Error, "Try again later.") {
		t.Errorf("Download = %+v; want available with %q", st, want)
	}
	var e *Error
	if !errors.As(err, &e) || e.Text != want {
		t.Errorf("err = %#v", err)
	}
	if n := f.rel.called("/usr/bin/open", "-R"); n != 0 {
		t.Errorf("Reveal ran %d times with the DMG gone", n)
	}
	if n := f.rel.called("xattr", "-w", "com.apple.quarantine"); n != 1 { // Verify only
		t.Errorf("quarantine written %d times, want 1", n)
	}
}

func TestService_PreflightBeforeStaging(t *testing.T) {
	f, st, _ := aiInstallFailure(t, "location")
	if st.Error != installFailureText(ErrLocation, true) {
		t.Errorf("status = %+v", st)
	}
	if n := f.rel.called("hdiutil"); n != 0 {
		t.Errorf("staging started after a refused location: %v", f.rel.recorded())
	}
}
