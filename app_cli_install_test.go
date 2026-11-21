package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// installFixture points the install seams at a temp dir: exe is a resolved
// fake Portty binary, link a not-yet-existing bin/portty. runs counts runner
// calls; dialogs records every alert, confirms every Continue/Cancel question,
// which gets answer.
type installFixture struct {
	exe, link string
	runs      [][2]string
	dialogs   []runtime.MessageDialogOptions
	confirms  []runtime.MessageDialogOptions
	answer    string
	app       *App
}

func newInstallFixture(t *testing.T, run func(exe, link string) error) *installFixture {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir()) // /var/folders → /private/var/folders
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "Applications", "Portty.app", "Contents", "MacOS", "Portty")
	if err := os.MkdirAll(filepath.Dir(exe), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	f := &installFixture{exe: exe, link: filepath.Join(dir, "usr", "local", "bin", "portty")}
	f.answer = "Continue"
	f.app = &App{messageDialogFunc: func(o runtime.MessageDialogOptions) (string, error) {
		if o.Type == runtime.QuestionDialog { // the confirm before the admin prompt (11-REVIEW IN-04)
			f.confirms = append(f.confirms, o)
			return f.answer, nil
		}
		f.dialogs = append(f.dialogs, o)
		return "OK", nil
	}}
	origLink, origExe, origRun := cliInstallLink, cliInstallExecutable, cliInstallRun
	t.Cleanup(func() { cliInstallLink, cliInstallExecutable, cliInstallRun = origLink, origExe, origRun })
	cliInstallLink = f.link
	cliInstallExecutable = func() (string, error) { return f.exe, nil }
	cliInstallRun = func(exe, link string) error {
		f.runs = append(f.runs, [2]string{exe, link})
		if run == nil {
			t.Error("the privileged runner ran, want no prompt")
			return nil
		}
		return run(exe, link)
	}
	return f
}

// unprivilegedLink does what the osascript script does, without privileges.
func unprivilegedLink(exe, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	return os.Symlink(exe, link)
}

func (f *installFixture) wantDialog(t *testing.T, typ runtime.DialogType, title, message string) {
	t.Helper()
	if len(f.dialogs) != 1 {
		t.Fatalf("dialogs = %+v, want exactly one", f.dialogs)
	}
	d := f.dialogs[0]
	if d.Type != typ || d.Title != title || d.Message != message {
		t.Errorf("dialog = %s %q / %q\nwant      %s %q / %q", d.Type, d.Title, d.Message, typ, title, message)
	}
	if strings.Join(d.Buttons, "|") != "OK" || d.DefaultButton != "OK" {
		t.Errorf("buttons %v default %q, want one OK", d.Buttons, d.DefaultButton)
	}
}

func (f *installFixture) mkLinkDir(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f.link), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestInstallCLI_Absent(t *testing.T) {
	f := newInstallFixture(t, unprivilegedLink)
	f.app.installCommandLineTool()
	if len(f.runs) != 1 || f.runs[0] != [2]string{f.exe, f.link} {
		t.Fatalf("runner calls = %v, want one with (%s, %s)", f.runs, f.exe, f.link)
	}
	if got, err := os.Readlink(f.link); err != nil || got != f.exe {
		t.Errorf("link = %q (%v), want %q", got, err, f.exe)
	}
	f.wantDialog(t, runtime.InfoDialog, "The portty command is installed.", f.link+" now points to Portty. Try “portty list” in Terminal.")
	if len(f.confirms) != 1 {
		t.Fatalf("confirms = %+v, want one before the admin prompt", f.confirms)
	}
	c := f.confirms[0]
	if c.Title != "Install Command-Line Tool" || c.Message != "macOS will ask for your password to put the portty command in /usr/local/bin." ||
		strings.Join(c.Buttons, "|") != "Continue|Cancel" || c.DefaultButton != "Continue" || c.CancelButton != "Cancel" {
		t.Errorf("confirm = %+v", c)
	}
}

// 11-REVIEW IN-04: Cancel at the confirm shows no admin prompt, changes nothing and says nothing.
func TestInstallCLI_ConfirmCancel(t *testing.T) {
	f := newInstallFixture(t, nil)
	f.answer = "Cancel"
	f.app.installCommandLineTool()
	if len(f.confirms) != 1 || len(f.runs) != 0 || len(f.dialogs) != 0 {
		t.Errorf("confirms %d, runner calls %d, dialogs %+v; want 1, 0, none", len(f.confirms), len(f.runs), f.dialogs)
	}
	if _, err := os.Lstat(f.link); err == nil {
		t.Error("Cancel left a link behind")
	}
}

func TestInstallCLI_AlreadyOurs(t *testing.T) {
	f := newInstallFixture(t, nil)
	if err := unprivilegedLink(f.exe, f.link); err != nil {
		t.Fatal(err)
	}
	f.app.installCommandLineTool()
	f.wantDialog(t, runtime.InfoDialog, "The portty command is already installed.", f.link+" already points to this copy of Portty.")
}

func testForeignLink(t *testing.T, target string) {
	f := newInstallFixture(t, nil)
	f.mkLinkDir(t)
	if err := os.Symlink(target, f.link); err != nil {
		t.Fatal(err)
	}
	f.app.installCommandLineTool()
	f.wantDialog(t, runtime.WarningDialog, "Another portty command is already installed.",
		f.link+" points to "+target+". Portty didn't change it. To use this copy instead, remove that link, then choose Install Command-Line Tool… again.")
	if got, _ := os.Readlink(f.link); got != target {
		t.Errorf("link changed to %q, want %q", got, target)
	}
}

func TestInstallCLI_ForeignLink(t *testing.T) {
	other := filepath.Join(t.TempDir(), "Other Portty.app", "portty")
	os.MkdirAll(filepath.Dir(other), 0o700)
	if err := os.WriteFile(other, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	testForeignLink(t, other)
}

func TestInstallCLI_DanglingLink(t *testing.T) {
	testForeignLink(t, "../gone/portty") // raw readlink value, relative and missing
}

func TestInstallCLI_ForeignFile(t *testing.T) {
	f := newInstallFixture(t, nil)
	f.mkLinkDir(t)
	if err := os.WriteFile(f.link, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.installCommandLineTool()
	f.wantDialog(t, runtime.WarningDialog, "Another portty command is already installed.",
		f.link+" is a file Portty didn't install. Portty didn't change it. To use this copy instead, remove that file, then choose Install Command-Line Tool… again.")
	if b, _ := os.ReadFile(f.link); string(b) != "#!/bin/sh\n" {
		t.Errorf("file changed to %q", b)
	}
}

func TestInstallCLI_DiskImageAndTranslocated(t *testing.T) {
	for exe, folder := range map[string]string{
		"/Volumes/X/Portty.app/Contents/MacOS/Portty":                                  "/Volumes/X",
		"/private/var/folders/x/AppTranslocation/y/d/Portty.app/Contents/MacOS/Portty": "/private/var/folders/x/AppTranslocation/y/d",
	} {
		f := newInstallFixture(t, nil)
		f.exe = exe
		f.app.installCommandLineTool()
		f.wantDialog(t, runtime.WarningDialog, "Move Portty to the Applications folder first.",
			"Portty is running from "+folder+", so a command installed now would stop working. Move Portty to Applications, open it from there, then try again.")
		if _, err := os.Lstat(f.link); err == nil {
			t.Errorf("%s: a link was created", exe)
		}
	}
}

func TestInstallCLI_Cancel(t *testing.T) {
	f := newInstallFixture(t, func(_, _ string) error {
		return &installError{code: 1, stderr: "0:212: execution error: User canceled. (-128)"}
	})
	f.app.installCommandLineTool()
	if len(f.runs) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(f.runs))
	}
	if len(f.dialogs) != 0 {
		t.Errorf("dialogs = %+v, want none after a cancel", f.dialogs)
	}
	if _, err := os.Lstat(f.link); err == nil {
		t.Error("a cancel left a link behind")
	}
}

// 11-REVIEW IN-01: choosing the item again while the first prompt is up starts no second run.
func TestInstallCLI_OneAtATime(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	f := newInstallFixture(t, func(exe, link string) error {
		close(entered)
		<-release
		return unprivilegedLink(exe, link)
	})
	done := make(chan struct{})
	go func() { f.app.installCommandLineTool(); close(done) }()
	<-entered
	f.app.installCommandLineTool() // returns at once: the first run holds the guard
	if len(f.runs) != 1 {
		t.Errorf("runner calls = %d while the first prompt is up, want 1", len(f.runs))
	}
	close(release)
	<-done
	f.wantDialog(t, runtime.InfoDialog, "The portty command is installed.", f.link+" now points to Portty. Try “portty list” in Terminal.")
	// The guard is released: a later run checks again (and finds the link already ours).
	f.dialogs = nil
	f.app.installCommandLineTool()
	f.wantDialog(t, runtime.InfoDialog, "The portty command is already installed.", f.link+" already points to this copy of Portty.")
}

func TestInstallCLI_Failure(t *testing.T) {
	for _, c := range []struct {
		err    error
		detail string
	}{
		{&installError{code: 1, stderr: "0:212: execution error: ln: /usr/local/bin/portty: File exists (1)"}, "0:212: execution error: ln: /usr/local/bin/portty: File exists (1)"},
		{&installError{code: 2}, "osascript exited with status 2"},
		{errors.New("fork/exec /usr/bin/osascript: no such file or directory."), "fork/exec /usr/bin/osascript: no such file or directory"},
	} {
		f := newInstallFixture(t, func(_, _ string) error { return c.err })
		f.app.installCommandLineTool()
		f.wantDialog(t, runtime.WarningDialog, "Couldn't install the portty command.",
			c.detail+`. Try again, or create the link yourself in Terminal: sudo ln -s "`+f.exe+`" /usr/local/bin/portty`)
	}
}

func TestInstallCLI_OsascriptArgv(t *testing.T) {
	exe := `/Applications/Ken's "Portty" $(rm -rf ~).app/Contents/MacOS/Portty`
	link := "/usr/local/bin/portty"
	args := osascriptInstallArgs(exe, link)
	if len(args) != 8 || args[0] != "-e" || args[2] != "-e" || args[4] != "-e" || args[6] != exe || args[7] != link {
		t.Fatalf("args = %q, want -e three script lines, then the two paths", args)
	}
	if args[1] != "on run argv" || args[5] != "end run" {
		t.Errorf("script frame = %q … %q", args[1], args[5])
	}
	script := args[1] + "\n" + args[3] + "\n" + args[5]
	for _, want := range []string{
		"quoted form of item 1 of argv", "quoted form of item 2 of argv",
		"/bin/mkdir -p /usr/local/bin", "/bin/ln -s ", "with administrator privileges",
		`with prompt "Portty wants to install the portty command in /usr/local/bin."`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "ln -sf") || strings.Contains(script, "ln -fs") || strings.Contains(script, "Ken's") || strings.Contains(script, link) {
		t.Errorf("script forces the link or holds a path:\n%s", script)
	}
	if !(&installError{code: 1, stderr: "User canceled. (-128)"}).canceled() || (&installError{code: 1, stderr: "File exists"}).canceled() {
		t.Error("cancel detection wrong")
	}
}
