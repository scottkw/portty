package updater

// Mover (UPD-08): telling that Portty runs from a mounted disk image, finding the DMG behind the
// volume, and where a first copy goes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StdoutRunner is a Runner that returns stdout only. hdiutil prints a deprecation warning on
// stderr (macOS 27), which would corrupt the plist a combined runner returns.
func StdoutRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// DiskImage is one attached image from `hdiutil info -plist`.
type DiskImage struct {
	ImagePath   string
	Writeable   bool
	MountPoints []string
}

// ParseHdiutilInfo reads the JSON plutil makes of `hdiutil info -plist` (18-VM-CHECKS.md P6).
func ParseHdiutilInfo(b []byte) ([]DiskImage, error) {
	var info struct {
		Images []struct {
			ImagePath string `json:"image-path"`
			Writeable bool   `json:"writeable"`
			Entities  []struct {
				MountPoint string `json:"mount-point"`
			} `json:"system-entities"`
		} `json:"images"`
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, fmt.Errorf("hdiutil info: %w", err)
	}
	imgs := make([]DiskImage, 0, len(info.Images))
	for _, im := range info.Images {
		d := DiskImage{ImagePath: im.ImagePath, Writeable: im.Writeable}
		for _, e := range im.Entities {
			if e.MountPoint != "" {
				d.MountPoints = append(d.MountPoints, e.MountPoint)
			}
		}
		imgs = append(imgs, d)
	}
	return imgs, nil
}

// DMGRun is a Portty running from a mounted disk image: the bundle on the volume, the volume's
// mount point and the image file behind it.
type DMGRun struct{ Bundle, MountPoint, ImagePath string }

// DetectDMGRun reports whether bundle is on a read-only disk-image volume. A bundle outside
// /Volumes is not, and hdiutil is never run for it. A Gatekeeper-translocated bundle is judged by
// its origin (statfs's mount-from name); an origin that cannot be read is an error and no run, so
// the caller starts normally and logs it. A writeable image (an external disk) is not a DMG run.
func DetectDMGRun(ctx context.Context, run, stdout Runner, bundle string, origin func(string) (string, error)) (DMGRun, bool, error) {
	if strings.Contains(bundle, "/AppTranslocation/") {
		o, err := origin(bundle)
		if err != nil {
			return DMGRun{}, false, err
		}
		// shortcut: P5 saw no translocation for a DMG launch, so the mount-from shape is untested;
		// a folder origin gets the bundle name added.
		if filepath.Base(o) != BundleName {
			o = filepath.Join(o, BundleName)
		}
		bundle = o
	}
	if !strings.HasPrefix(bundle, "/Volumes/") {
		return DMGRun{}, false, nil
	}
	plist, err := stdout(ctx, "/usr/bin/hdiutil", "info", "-plist")
	if err != nil {
		return DMGRun{}, false, fmt.Errorf("hdiutil info: %w", err)
	}
	dir, err := os.MkdirTemp("", "portty-hdiutil-")
	if err != nil {
		return DMGRun{}, false, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	file := filepath.Join(dir, "info.plist")
	if err := os.WriteFile(file, plist, 0o600); err != nil {
		return DMGRun{}, false, err
	}
	out, err := run(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", file)
	if err != nil {
		return DMGRun{}, false, fmt.Errorf("plutil: %w", err)
	}
	imgs, err := ParseHdiutilInfo(out)
	if err != nil {
		return DMGRun{}, false, err
	}
	for _, im := range imgs {
		for _, mp := range im.MountPoints {
			if !im.Writeable && strings.HasPrefix(bundle, mp+"/") {
				return DMGRun{Bundle: bundle, MountPoint: mp, ImagePath: im.ImagePath}, true, nil
			}
		}
	}
	return DMGRun{}, false, nil
}

// InstalledVersion reads app's CFBundleShortVersionString from its Info.plist. It never runs the
// installed app (Pitfall 7: an old Portty started with --version opens its GUI).
func InstalledVersion(ctx context.Context, run Runner, app string) (string, error) {
	out, err := run(ctx, "/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return "", errors.New("installed version unreadable")
	}
	v := strings.TrimSpace(string(out))
	if _, err := ParseVersion(v); err != nil {
		return "", err
	}
	return v, nil
}

// ExistingCopy is the installed Portty.app in the system Applications folder, else in home's
// Applications folder, else "". A symlink is not a copy.
func ExistingCopy(system, home string) string {
	for _, p := range []string{filepath.Join(system, BundleName), filepath.Join(home, "Applications", BundleName)} {
		if isDir(p) {
			return p
		}
	}
	return ""
}

// MoveTarget is where a first copy goes: the system Applications folder when writable, else
// home's Applications folder (made 0755 if missing).
func MoveTarget(system, home string, writable func(string) bool) (string, error) {
	if writable(system) {
		return filepath.Join(system, BundleName), nil
	}
	dir := filepath.Join(home, "Applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, BundleName), nil
}
