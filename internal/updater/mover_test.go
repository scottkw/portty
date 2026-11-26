package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// hdiutilInfoP6 is `hdiutil info -plist | plutil -convert json -o - -` from the VM while the
// quarantined v1.2.0 DMG was open the way Finder opens it (18-VM-CHECKS.md row P6), with the
// account name in the image path replaced by USER.
const hdiutilInfoP6 = `{"revision":"710","vendor":"Apple","framework":"710","images":[{"image-path":"/Users/USER/Downloads/portty-v1.2.0-darwin-universal.dmg","image-type":"read-only disk image","autodiskmount":true,"image-encrypted":false,"blocksize":512,"system-entities":[{"content-hint":"GUID_partition_scheme","dev-entry":"/dev/disk6"},{"content-hint":"7C3457EF-0000-11AA-AA11-00306543ECAC","dev-entry":"/dev/disk6s1"},{"content-hint":"EF57347C-0000-11AA-AA11-00306543ECAC","dev-entry":"/dev/disk7"},{"content-hint":"41504653-0000-11AA-AA11-00306543ECAC","dev-entry":"/dev/disk7s1","mount-point":"/Volumes/Portty"}],"diskimages2":true,"removable":true,"blockcount":103565,"hdid-pid":56647,"writeable":false,"owner-mode":288,"owner-uid":501},{"owner-uid":0,"blockcount":12288,"image-path":"/System/Library/AssetsV2/com_apple_MobileAsset_PKITrustStore/purpose_auto/d1b36c40ecf011c1a2dfeacc93215881ad4cf525.asset/AssetData/Restore/SECUREPKITRUSTSTOREASSETS_SECUREPKITRUSTSTORE_Cryptex.dmg","system-entities":[{"content-hint":"","dev-entry":"/dev/disk4"},{"content-hint":"EF57347C-0000-11AA-AA11-00306543ECAC","dev-entry":"/dev/disk5"},{"content-hint":"41504653-0000-11AA-AA11-00306543ECAC","dev-entry":"/dev/disk5s1","mount-point":"/System/Library/AssetsV2/com_apple_MobileAsset_PKITrustStore/purpose_auto/d1b36c40ecf011c1a2dfeacc93215881ad4cf525.asset/.AssetData"}],"writeable":false,"diskimages2":false,"blocksize":512,"removable":true,"autodiskmount":false}]}`

const p6Image = "/Users/USER/Downloads/portty-v1.2.0-darwin-universal.dmg"

func TestParseHdiutilInfo(t *testing.T) {
	imgs, err := ParseHdiutilInfo([]byte(hdiutilInfoP6))
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fatalf("images = %+v, want 2", imgs)
	}
	if imgs[0].ImagePath != p6Image || imgs[0].Writeable || !slices.Equal(imgs[0].MountPoints, []string{"/Volumes/Portty"}) {
		t.Errorf("image 0 = %+v", imgs[0])
	}
	if len(imgs[1].MountPoints) != 1 || !strings.HasSuffix(imgs[1].MountPoints[0], "/.AssetData") {
		t.Errorf("image 1 = %+v", imgs[1])
	}
	if _, err := ParseHdiutilInfo([]byte("hdiutil: WARNING: deprecated\n<?xml")); err == nil {
		t.Error("garbage parsed")
	}
}

// moverRun fakes hdiutil (through the stdout runner) and plutil (through run): plutil returns
// info as JSON after checking it was handed the file the stdout runner's output went to.
type moverRun struct {
	info     string
	hdiutil  int
	plutil   int
	hdiErr   error
	plistOut string // what hdiutil printed, checked against the file plutil reads
	tmpFile  string
}

func (m *moverRun) stdout(_ context.Context, name string, args ...string) ([]byte, error) {
	if name != "/usr/bin/hdiutil" || !slices.Equal(args, []string{"info", "-plist"}) {
		return nil, errors.New("unexpected stdout call " + name)
	}
	m.hdiutil++
	m.plistOut = "<plist>fake</plist>"
	return []byte(m.plistOut), m.hdiErr
}

func (m *moverRun) run(_ context.Context, name string, args ...string) ([]byte, error) {
	if name != "/usr/bin/plutil" || len(args) != 5 || args[0] != "-convert" || args[1] != "json" || args[3] != "-" {
		return nil, errors.New("unexpected run call " + name + " " + strings.Join(args, " "))
	}
	m.plutil++
	m.tmpFile = args[4]
	b, err := os.ReadFile(args[4])
	if err != nil || string(b) != m.plistOut {
		return nil, errors.New("plutil did not get hdiutil's output")
	}
	fi, _ := os.Stat(args[4])
	if fi.Mode().Perm() != 0o600 {
		return nil, errors.New("plist temp file is not 0600")
	}
	if di, _ := os.Stat(filepath.Dir(args[4])); di.Mode().Perm() != 0o700 {
		return nil, errors.New("plist temp dir is not 0700")
	}
	return []byte(m.info), nil
}

func noOrigin(string) (string, error) { return "", errors.New("origin must not be asked") }

func TestDetectDMGRun(t *testing.T) {
	ctx := context.Background()
	writeable := strings.Replace(hdiutilInfoP6, `"writeable":false,"owner-mode"`, `"writeable":true,"owner-mode"`, 1)
	for _, c := range []struct {
		name, bundle, info string
		origin             func(string) (string, error)
		want               DMGRun
		ok, wantErr        bool
		hdiutil            int
	}{
		{name: "dmg volume", bundle: "/Volumes/Portty/Portty.app", info: hdiutilInfoP6, origin: noOrigin,
			want: DMGRun{Bundle: "/Volumes/Portty/Portty.app", MountPoint: "/Volumes/Portty", ImagePath: p6Image}, ok: true, hdiutil: 1},
		{name: "writeable image (an external disk)", bundle: "/Volumes/Portty/Portty.app", info: writeable, origin: noOrigin, hdiutil: 1},
		{name: "installed", bundle: "/Applications/Portty.app", info: hdiutilInfoP6, origin: noOrigin},
		{name: "slash boundary", bundle: "/Volumes/Portty 1/Portty.app", info: hdiutilInfoP6, origin: noOrigin, hdiutil: 1},
		{name: "translocated from the dmg", bundle: "/private/var/folders/x/AppTranslocation/ABC/d/Portty.app", info: hdiutilInfoP6,
			origin: func(string) (string, error) { return "/Volumes/Portty/Portty.app", nil },
			want:   DMGRun{Bundle: "/Volumes/Portty/Portty.app", MountPoint: "/Volumes/Portty", ImagePath: p6Image}, ok: true, hdiutil: 1},
		{name: "translocated from elsewhere", bundle: "/private/var/folders/x/AppTranslocation/ABC/d/Portty.app", info: hdiutilInfoP6,
			origin: func(string) (string, error) { return "/Users/USER/Downloads/Portty.app", nil }},
		{name: "origin error", bundle: "/private/var/folders/x/AppTranslocation/ABC/d/Portty.app", info: hdiutilInfoP6,
			origin: func(string) (string, error) { return "", errors.New("statfs failed") }, wantErr: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := &moverRun{info: c.info}
			got, ok, err := DetectDMGRun(ctx, m.run, m.stdout, c.bundle, c.origin)
			if ok != c.ok || got != c.want || (err != nil) != c.wantErr {
				t.Fatalf("DetectDMGRun = %+v, %v, %v; want %+v, %v, err %v", got, ok, err, c.want, c.ok, c.wantErr)
			}
			if m.hdiutil != c.hdiutil {
				t.Errorf("hdiutil ran %d times, want %d", m.hdiutil, c.hdiutil)
			}
			if m.tmpFile != "" {
				if _, err := os.Lstat(filepath.Dir(m.tmpFile)); !errors.Is(err, os.ErrNotExist) {
					t.Error("the plist temp dir was left behind")
				}
			}
		})
	}
	t.Run("hdiutil fails", func(t *testing.T) {
		m := &moverRun{info: hdiutilInfoP6, hdiErr: errors.New("exit status 1")}
		if _, ok, err := DetectDMGRun(ctx, m.run, m.stdout, "/Volumes/Portty/Portty.app", noOrigin); ok || err == nil {
			t.Fatalf("ok %v, err %v; want false and the error", ok, err)
		}
		if m.plutil != 0 {
			t.Error("plutil ran after hdiutil failed")
		}
	})
}

func TestInstalledVersion(t *testing.T) {
	ctx := context.Background()
	var asked []string
	run := func(out string, err error) Runner {
		return func(_ context.Context, name string, args ...string) ([]byte, error) {
			asked = append(asked, name+" "+strings.Join(args, " "))
			return []byte(out), err
		}
	}
	if v, err := InstalledVersion(ctx, run("1.2.0\n", nil), "/Applications/Portty.app"); err != nil || v != "1.2.0" {
		t.Fatalf("InstalledVersion = %q, %v", v, err)
	}
	if want := "/usr/bin/plutil -extract CFBundleShortVersionString raw /Applications/Portty.app/Contents/Info.plist"; len(asked) != 1 || asked[0] != want {
		t.Errorf("ran %q, want only %q (never the installed app)", asked, want)
	}
	for _, c := range []struct {
		out string
		err error
	}{{"0.1.0-beta\n", nil}, {"", nil}, {"1.2.0\n", errors.New("exit status 1")}} {
		if v, err := InstalledVersion(ctx, run(c.out, c.err), "/Applications/Portty.app"); err == nil {
			t.Errorf("InstalledVersion(%q, %v) = %q, want an error", c.out, c.err, v)
		}
	}
}

func TestExistingCopy(t *testing.T) {
	system, home := t.TempDir(), t.TempDir()
	if got := ExistingCopy(system, home); got != "" {
		t.Errorf("no copy: %q", got)
	}
	userApp := siBundle(t, filepath.Join(home, "Applications"), "1.0.0")
	if got := ExistingCopy(system, home); got != userApp {
		t.Errorf("home copy: %q, want %q", got, userApp)
	}
	sysApp := siBundle(t, system, "1.0.0")
	if got := ExistingCopy(system, home); got != sysApp {
		t.Errorf("both: %q, want the system copy %q", got, sysApp)
	}
	link := t.TempDir()
	if err := os.Symlink(sysApp, filepath.Join(link, BundleName)); err != nil {
		t.Fatal(err)
	}
	if got := ExistingCopy(link, t.TempDir()); got != "" {
		t.Errorf("symlink counted as a copy: %q", got)
	}
}

func TestMoveTarget(t *testing.T) {
	system, home := t.TempDir(), t.TempDir()
	got, err := MoveTarget(system, home, func(d string) bool { return d == system })
	if err != nil || got != filepath.Join(system, BundleName) {
		t.Fatalf("writable: %q, %v", got, err)
	}
	got, err = MoveTarget(system, home, func(string) bool { return false })
	if want := filepath.Join(home, "Applications", BundleName); err != nil || got != want {
		t.Fatalf("not writable: %q, %v; want %q", got, err, want)
	}
	if fi, err := os.Stat(filepath.Join(home, "Applications")); err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o755 {
		t.Errorf("home Applications = %v, %v; want a 0755 dir", fi, err)
	}
	if _, err := MoveTarget(system, filepath.Join(home, "missing", "deeper"), func(string) bool { return false }); err != nil {
		t.Errorf("MoveTarget should create the home Applications folder: %v", err)
	}
}
