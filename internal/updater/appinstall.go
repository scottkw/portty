package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// The installed app's shape, shared by the staging pipeline, the installer and the job checks.
const (
	BundleName       = "Portty.app"
	StagingPrefix    = ".Portty-update-"
	BundleExecutable = "Contents/MacOS/Portty"
	// AppIdentifier is the release app's signing identifier (probe P7 in 18-VM-CHECKS.md).
	AppIdentifier = "app.portty.Portty"
	// SmokeTimeout bounds the staged app's --version run.
	SmokeTimeout = 10 * time.Second
)

// ErrInstall wraps every staging failure. Messages carry the step name only, never a path.
var ErrInstall = errors.New("install failed")

// The install failure causes, each with its own banner text (installFailureText). Each wraps
// ErrInstall.
var (
	ErrNoWrite       = fmt.Errorf("%w: no write access", ErrInstall)
	ErrAppManagement = fmt.Errorf("%w: refused by macOS", ErrInstall)
	ErrNoSpace       = fmt.Errorf("%w: not enough disk space", ErrInstall)
	ErrLocation      = fmt.Errorf("%w: wrong location", ErrInstall)
	ErrSafety        = fmt.Errorf("%w: safety check", ErrInstall)
	ErrMount         = fmt.Errorf("%w: mount", ErrInstall)
	ErrCopy          = fmt.Errorf("%w: copy", ErrInstall)
	ErrDetach        = fmt.Errorf("%w: detach", ErrInstall)
)

// installErr wraps cause (a sentinel above, or ErrInstall) with the step name, for the logs.
func installErr(cause error, step string) error { return fmt.Errorf("%w: %s", cause, step) }

// StageInput is what Stage needs: the verified DMG, the installed bundle (the staging folder is
// made beside it), the folder for the mount point and the version being installed.
type StageInput struct{ DMG, Target, MountRoot, NewVersion string }

// Preflight refuses a target Portty cannot or should not replace itself in, before anything is
// staged: a path that is not an absolute, clean .../Portty.app, one on a mounted volume or
// translocated by Gatekeeper, a symlink or non-folder, or a folder this account cannot write.
// A missing target is fine (the mover installs a first copy). It never asks for a password or
// raises privileges: every refusal falls back to Reveal.
func Preflight(target string) error {
	if !filepath.IsAbs(target) || filepath.Clean(target) != target || filepath.Base(target) != BundleName ||
		strings.HasPrefix(target, "/Volumes/") || strings.Contains(target, "/AppTranslocation/") {
		return installErr(ErrLocation, "preflight")
	}
	fi, err := os.Lstat(target)
	switch {
	case err == nil && (fi.Mode()&fs.ModeSymlink != 0 || !fi.IsDir()):
		return installErr(ErrLocation, "preflight")
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return installErr(ErrNoWrite, "preflight")
	}
	if err := unix.Access(filepath.Dir(target), unix.W_OK); err != nil {
		return installErr(ErrNoWrite, "preflight")
	}
	return nil
}

// freeBytes is the space this account may use on path's volume; a var so tests set it.
var freeBytes = func(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// AppSize sums the sizes of the regular files in app; symlinks are not followed.
func AppSize(app string) (int64, error) {
	var n int64
	err := filepath.WalkDir(app, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		fi, err := d.Info()
		if err == nil {
			n += fi.Size()
		}
		return err
	})
	return n, err
}

// CheckSpace refuses when parent's volume has less than need bytes free. A volume it cannot ask
// is a plain install failure, not the disk-space text.
func CheckSpace(parent string, need int64) error {
	free, err := freeBytes(parent)
	if err != nil {
		return installErr(ErrInstall, "space")
	}
	if need > 0 && free < uint64(need) {
		return installErr(ErrNoSpace, "space")
	}
	return nil
}

// Mount attaches dmg read-only at mountDir (which must exist) with no Finder window, no checksum
// pass and no prompt (exec gives stdin /dev/null), and requires a real Portty.app directory in it.
func Mount(ctx context.Context, run Runner, dmg, mountDir string) error {
	if _, err := run(ctx, "/usr/bin/hdiutil", "attach", "-nobrowse", "-readonly", "-noverify", "-noautoopen", "-mountpoint", mountDir, dmg); err != nil {
		return installErr(ErrMount, "mount")
	}
	if fi, err := os.Lstat(filepath.Join(mountDir, BundleName)); err != nil || !fi.IsDir() {
		_ = Detach(ctx, run, mountDir)
		return installErr(ErrMount, "mount")
	}
	return nil
}

// detachPause is the wait between detach attempts; a var so tests do not wait.
var detachPause = time.Second

// Detach ejects the image at mountDir and removes the folder. A folder that is already gone is
// success (a repeated cleanup must not fail a good install); a busy image gets three plain tries
// a second apart, then one -force.
func Detach(ctx context.Context, run Runner, mountDir string) error {
	if _, err := os.Lstat(mountDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(detachPause):
			}
		}
		if _, err = run(ctx, "/usr/bin/hdiutil", "detach", mountDir); err == nil {
			break
		}
	}
	if err != nil {
		_, err = run(ctx, "/usr/bin/hdiutil", "detach", "-force", mountDir)
	}
	if err != nil {
		_ = os.Remove(mountDir) // fails harmlessly while the image is still mounted
		return installErr(ErrDetach, "detach")
	}
	if err := os.Remove(mountDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return installErr(ErrDetach, "detach")
	}
	return nil
}

// mkdirTemp is os.MkdirTemp; a var so tests can return EACCES and EPERM.
var mkdirTemp = os.MkdirTemp

// CopyApp copies srcApp into a new 0700 staging folder beside target and returns the staged app.
// ditto, never cp or a tree walk, keeps symlinks, xattrs and the signature seal. EPERM where the
// folder is writable is taken to be App Management (A2); a wrong guess only changes the wording.
func CopyApp(ctx context.Context, run Runner, srcApp, target string) (string, error) {
	dir, err := mkdirTemp(filepath.Dir(target), StagingPrefix)
	switch {
	case errors.Is(err, syscall.EACCES):
		return "", installErr(ErrNoWrite, "copy")
	case errors.Is(err, syscall.EPERM) && unix.Access(filepath.Dir(target), unix.W_OK) == nil:
		return "", installErr(ErrAppManagement, "copy")
	case errors.Is(err, syscall.EPERM):
		return "", installErr(ErrNoWrite, "copy")
	case err != nil:
		return "", installErr(ErrCopy, "copy")
	}
	staged := filepath.Join(dir, BundleName)
	if out, err := run(ctx, "/usr/bin/ditto", srcApp, staged); err != nil {
		_ = os.RemoveAll(dir)
		switch {
		case bytes.Contains(out, []byte("No space left on device")):
			return "", installErr(ErrNoSpace, "copy")
		case bytes.Contains(out, []byte("Operation not permitted")):
			return "", installErr(ErrAppManagement, "copy")
		}
		return "", installErr(ErrCopy, "copy")
	}
	return staged, nil
}

// VerifyApp is the one place an app bundle's signature is checked: a strict deep codesign verify,
// the release Team ID and the app's signing identifier.
func VerifyApp(ctx context.Context, run Runner, app string) error {
	if _, err := run(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app); err != nil {
		return installErr(ErrSafety, "app-codesign")
	}
	out, err := run(ctx, "/usr/bin/codesign", "-dvv", app)
	if err != nil || !hasLine(out, "TeamIdentifier="+TeamID) {
		return installErr(ErrSafety, "app-team")
	}
	if !hasLine(out, "Identifier="+AppIdentifier) {
		return installErr(ErrSafety, "app-identifier")
	}
	return nil
}

// CheckStagedApp holds the staged app to what a manual install would face, after VerifyApp: a
// notarized exec assessment, the release's exact version, a minimum macOS this Mac meets, and a
// --version run that answers v+version within SmokeTimeout. Only this staged, signature-checked
// new build is ever run, never an installed older one (Pitfall 7).
func CheckStagedApp(ctx context.Context, run Runner, app, version string) error {
	out, err := run(ctx, "/usr/sbin/spctl", "-a", "-t", "exec", "-vv", app)
	if err != nil || !bytes.Contains(out, []byte(": accepted")) || !hasLine(out, "source=Notarized Developer ID") {
		return installErr(ErrSafety, "app-spctl")
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	out, err = run(ctx, "/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", plist)
	if err != nil || strings.TrimSpace(string(out)) != version {
		return installErr(ErrSafety, "app-version")
	}
	out, err = run(ctx, "/usr/bin/plutil", "-extract", "LSMinimumSystemVersion", "raw", plist)
	if err == nil {
		sys, serr := run(ctx, "/usr/bin/sw_vers", "-productVersion")
		if serr != nil || !osAtLeast(strings.TrimSpace(string(sys)), strings.TrimSpace(string(out))) {
			return installErr(ErrSafety, "app-min-os")
		}
	} else if !bytes.Contains(out, []byte("No value at that key path")) { // only a missing key passes
		return installErr(ErrSafety, "app-min-os")
	}
	sctx, cancel := context.WithTimeout(ctx, SmokeTimeout)
	defer cancel()
	out, err = run(sctx, filepath.Join(app, BundleExecutable), "--version")
	if err != nil || strings.TrimSpace(string(out)) != "v"+version {
		return installErr(ErrSafety, "app-smoke")
	}
	return nil
}

// osAtLeast reports whether macOS version have is min or newer, comparing dotted components as
// integers (14.10 is newer than 14.9; a missing component is 0). Anything unparsable is false.
func osAtLeast(have, min string) bool {
	h, m := strings.Split(have, "."), strings.Split(min, ".")
	for i := 0; i < max(len(h), len(m)); i++ {
		a, aerr := osPart(h, i)
		b, berr := osPart(m, i)
		if aerr != nil || berr != nil {
			return false
		}
		if a != b {
			return a > b
		}
	}
	return true
}

func osPart(parts []string, i int) (uint64, error) {
	if i >= len(parts) {
		return 0, nil
	}
	return strconv.ParseUint(parts[i], 10, 32)
}

// Stage turns a verified DMG into a staged copy of the app beside in.Target and returns its path.
// The DMG's quarantine is cleared first so the mounted volume raises no system dialog, and the
// staged copy's only after the mounted app passed VerifyApp (UPD-09). The target's volume needs
// twice the app's size free, and the staged copy must pass CheckStagedApp. The image is detached on
// every path after a good mount; on any failure the staging folder is removed too. The caller
// writes the DMG's quarantine back before it falls back to Reveal.
func Stage(ctx context.Context, run Runner, in StageInput) (string, error) {
	if err := ClearQuarantine(ctx, run, in.DMG); err != nil {
		return "", err
	}
	mnt, err := os.MkdirTemp(in.MountRoot, "mnt-")
	if err != nil {
		return "", installErr(ErrMount, "mount")
	}
	defer func() { _ = os.Remove(mnt) }() // only an empty folder: never a live mount
	if err := Mount(ctx, run, in.DMG, mnt); err != nil {
		return "", err
	}
	staged, err := StageFrom(ctx, run, filepath.Join(mnt, BundleName), in.Target, in.NewVersion)
	if derr := Detach(ctx, run, mnt); err == nil {
		err = derr
	}
	if err != nil {
		if staged != "" {
			_ = os.RemoveAll(filepath.Dir(staged))
		}
		return "", err
	}
	return staged, nil
}

// StageFrom verifies the mounted app, checks the space, copies it, strips the copy's quarantine
// and checks the copy. Stage and the mover share it. It returns the staged path even on error once
// the copy exists, so the caller can remove its folder.
func StageFrom(ctx context.Context, run Runner, mounted, target, version string) (string, error) {
	if err := VerifyApp(ctx, run, mounted); err != nil {
		return "", err
	}
	size, err := AppSize(mounted)
	if err != nil {
		// AppSize reads the copy source, which is already mounted on both paths, so this is a copy
		// failure; ErrMount's "couldn't open the download" would wrongly suggest a mount problem.
		return "", installErr(ErrCopy, "app-size")
	}
	if err := CheckSpace(filepath.Dir(target), 2*size); err != nil {
		return "", err
	}
	staged, err := CopyApp(ctx, run, mounted, target)
	if err != nil {
		return "", err
	}
	if err := ClearQuarantine(ctx, run, staged); err != nil {
		return staged, err
	}
	if err := VerifyApp(ctx, run, staged); err != nil {
		return staged, err
	}
	return staged, CheckStagedApp(ctx, run, staged, version)
}

// Swap exchanges the staged bundle and the installed one in a single renamex_np(RENAME_SWAP), so
// target holds a complete bundle at every instant; afterwards staged holds the OLD bundle. With
// no installed bundle there is nothing to exchange (RENAME_SWAP returns ENOENT), so it is a plain
// rename. Never ditto over a live bundle: in-place writes invalidate the code pages of the
// running old daemon.
func Swap(staged, target string) error {
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return os.Rename(staged, target)
	}
	return unix.RenamexNp(staged, target, unix.RENAME_SWAP)
}
