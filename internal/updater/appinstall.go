package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The installed app's shape, shared by the staging pipeline, the installer and the job checks.
const (
	BundleName       = "Portty.app"
	StagingPrefix    = ".Portty-update-"
	BundleExecutable = "Contents/MacOS/Portty"
)

// ErrInstall wraps every staging failure. Messages carry the step name only, never a path.
var ErrInstall = errors.New("install failed")

func installErr(step string) error { return fmt.Errorf("%w: %s", ErrInstall, step) }

// StageInput is what Stage needs: the verified DMG, the installed bundle (the staging folder is
// made beside it), the folder for the mount point and the version being installed.
type StageInput struct{ DMG, Target, MountRoot, NewVersion string }

// Mount attaches dmg read-only at mountDir (which must exist) with no Finder window, no checksum
// pass and no prompt (exec gives stdin /dev/null), and requires a real Portty.app directory in it.
func Mount(ctx context.Context, run Runner, dmg, mountDir string) error {
	if _, err := run(ctx, "/usr/bin/hdiutil", "attach", "-nobrowse", "-readonly", "-noverify", "-noautoopen", "-mountpoint", mountDir, dmg); err != nil {
		return installErr("mount")
	}
	if fi, err := os.Lstat(filepath.Join(mountDir, BundleName)); err != nil || !fi.IsDir() {
		_ = Detach(ctx, run, mountDir)
		return installErr("mount")
	}
	return nil
}

// Detach ejects the image at mountDir and removes the folder.
func Detach(ctx context.Context, run Runner, mountDir string) error {
	if _, err := run(ctx, "/usr/bin/hdiutil", "detach", mountDir); err != nil {
		return installErr("detach")
	}
	if err := os.Remove(mountDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return installErr("detach")
	}
	return nil
}

// CopyApp copies srcApp into a new 0700 staging folder beside target and returns the staged app.
// ditto, never cp or a tree walk, keeps symlinks, xattrs and the signature seal.
func CopyApp(ctx context.Context, run Runner, srcApp, target string) (string, error) {
	dir, err := os.MkdirTemp(filepath.Dir(target), StagingPrefix)
	if err != nil {
		return "", installErr("copy")
	}
	staged := filepath.Join(dir, BundleName)
	if _, err := run(ctx, "/usr/bin/ditto", srcApp, staged); err != nil {
		_ = os.RemoveAll(dir)
		return "", installErr("copy")
	}
	return staged, nil
}

// VerifyApp is the one place an app bundle's signature is checked: a strict deep codesign verify
// and the release Team ID.
func VerifyApp(ctx context.Context, run Runner, app string) error {
	if _, err := run(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app); err != nil {
		return installErr("app-codesign")
	}
	out, err := run(ctx, "/usr/bin/codesign", "-dvv", app)
	if err != nil || !hasLine(out, "TeamIdentifier="+TeamID) {
		return installErr("app-team")
	}
	return nil
}

// Stage turns a verified DMG into a staged copy of the app beside in.Target and returns its path.
// The DMG's quarantine is cleared first so the mounted volume raises no system dialog, and the
// staged copy's only after the mounted app passed VerifyApp (UPD-09). The image is detached on
// every path after a good mount; on any failure the staging folder is removed too. The caller
// writes the DMG's quarantine back before it falls back to Reveal.
func Stage(ctx context.Context, run Runner, in StageInput) (string, error) {
	if err := ClearQuarantine(ctx, run, in.DMG); err != nil {
		return "", err
	}
	mnt, err := os.MkdirTemp(in.MountRoot, "mnt-")
	if err != nil {
		return "", installErr("mount")
	}
	defer func() { _ = os.Remove(mnt) }() // only an empty folder: never a live mount
	if err := Mount(ctx, run, in.DMG, mnt); err != nil {
		return "", err
	}
	staged, err := stageFrom(ctx, run, filepath.Join(mnt, BundleName), in.Target)
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

// stageFrom verifies the mounted app, copies it, strips the copy's quarantine and verifies the
// copy. It returns the staged path even on error so Stage can remove it.
func stageFrom(ctx context.Context, run Runner, mounted, target string) (string, error) {
	if err := VerifyApp(ctx, run, mounted); err != nil {
		return "", err
	}
	staged, err := CopyApp(ctx, run, mounted, target)
	if err != nil {
		return "", err
	}
	if err := ClearQuarantine(ctx, run, staged); err != nil {
		return staged, err
	}
	return staged, VerifyApp(ctx, run, staged)
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
