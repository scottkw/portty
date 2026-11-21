package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Seams for Install Command-Line Tool… (PREV-05); tests point them at temp
// dirs and fake runners. The real admin prompt runs only in a VM (11-10).
var (
	cliInstallLink       = "/usr/local/bin/portty"
	cliInstallExecutable = func() (string, error) {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(exe)
	}
	cliInstallRun = runOsascriptInstall
)

const cliInstallPrompt = "Portty wants to install the portty command in /usr/local/bin."

// installError is a failed privileged run: osascript's exit code and stderr.
type installError struct {
	code   int
	stderr string
}

func (e *installError) Error() string {
	if e.stderr != "" {
		return e.stderr
	}
	return fmt.Sprintf("osascript exited with status %d", e.code)
}

// canceled: the user dismissed the admin prompt (exit 1, error -128 "User canceled").
func (e *installError) canceled() bool { return e.code == 1 && strings.Contains(e.stderr, "-128") }

// osascriptInstallArgs passes both paths as argv items that AppleScript quotes
// itself (quoted form of), so no Go string is ever spliced into shell text.
// ln -s without -f fails rather than overwrite a file that appeared meanwhile.
func osascriptInstallArgs(exe, link string) []string {
	return []string{
		"-e", "on run argv",
		"-e", `do shell script "/bin/mkdir -p /usr/local/bin && /bin/ln -s " & quoted form of item 1 of argv & " " & quoted form of item 2 of argv with prompt "` + cliInstallPrompt + `" with administrator privileges`,
		"-e", "end run",
		exe, link,
	}
}

func runOsascriptInstall(exe, link string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("/usr/bin/osascript", osascriptInstallArgs(exe, link)...)
	cmd.Stderr = &stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &installError{code: ee.ExitCode(), stderr: strings.TrimSpace(stderr.String())}
	}
	return err
}

// installCommandLineTool links /usr/local/bin/portty to this copy of Portty
// (UI-SPEC "Install Command-Line Tool"). Everything before the admin prompt is
// unprivileged, and nothing that exists at the link path is ever replaced.
// It shows native alerts, so call it off the main thread.
func (a *App) installCommandLineTool() {
	// A second click while the first run's prompt or alert is up does nothing (11-REVIEW IN-01).
	if !a.cliInstalling.CompareAndSwap(false, true) {
		return
	}
	defer a.cliInstalling.Store(false)
	link := cliInstallLink
	exe, err := cliInstallExecutable()
	if err != nil {
		a.installFailed(exe, err)
		return
	}
	if strings.HasPrefix(exe, "/Volumes/") || strings.Contains(exe, "/AppTranslocation/") {
		folder := filepath.Dir(exe)
		if i := strings.Index(exe, ".app/"); i >= 0 {
			folder = filepath.Dir(exe[:i+len(".app")])
		}
		a.cliAlert(runtime.WarningDialog, "Move Portty to the Applications folder first.",
			"Portty is running from "+folder+", so a command installed now would stop working. Move Portty to Applications, open it from there, then try again.")
		return
	}
	fi, err := os.Lstat(link)
	switch {
	case err == nil && fi.Mode()&fs.ModeSymlink != 0:
		if resolved, err := filepath.EvalSymlinks(link); err == nil && resolved == exe {
			a.cliAlert(runtime.InfoDialog, "The portty command is already installed.", link+" already points to this copy of Portty.")
			return
		}
		target, _ := os.Readlink(link)
		a.cliAlert(runtime.WarningDialog, "Another portty command is already installed.",
			link+" points to "+target+". Portty didn't change it. To use this copy instead, remove that link, then choose Install Command-Line Tool… again.")
		return
	case err == nil:
		a.cliAlert(runtime.WarningDialog, "Another portty command is already installed.",
			link+" is a file Portty didn't install. Portty didn't change it. To use this copy instead, remove that file, then choose Install Command-Line Tool… again.")
		return
	case !errors.Is(err, fs.ErrNotExist):
		a.installFailed(exe, err)
		return
	}
	if err := cliInstallRun(exe, link); err != nil {
		var ie *installError
		if errors.As(err, &ie) && ie.canceled() {
			return // macOS convention: a cancelled prompt changes nothing and says nothing
		}
		log.Printf("cli install: %v", err) // osascript's raw stderr; it never sees the password
		a.installFailed(exe, err)
		return
	}
	a.cliAlert(runtime.InfoDialog, "The portty command is installed.", link+" now points to Portty. Try “portty list” in Terminal.")
}

func (a *App) installFailed(exe string, err error) {
	a.cliAlert(runtime.WarningDialog, "Couldn't install the portty command.",
		strings.TrimRight(err.Error(), ".")+`. Try again, or create the link yourself in Terminal: sudo ln -s "`+exe+`" /usr/local/bin/portty`)
}

func (a *App) cliAlert(typ runtime.DialogType, title, message string) {
	a.dialog(runtime.MessageDialogOptions{Type: typ, Title: title, Message: message, Buttons: []string{"OK"}, DefaultButton: "OK"})
}
