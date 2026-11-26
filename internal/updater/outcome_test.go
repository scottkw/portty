package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rbResultFix is a Service (newSvc) with its cache folder made and a kept DMG in it.
func rbResultFix(t *testing.T) (svcFix, string) {
	t.Helper()
	f := newSvc(t)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	dmg := filepath.Join(f.dir, DMGAssetName("9.9.9"))
	if err := os.WriteFile(dmg, []byte("dmg"), 0o600); err != nil {
		t.Fatal(err)
	}
	return f, dmg
}

func rbCalls(f svcFix, sub string) int {
	f.run.mu.Lock()
	defer f.run.mu.Unlock()
	n := 0
	for _, c := range f.run.calls {
		if strings.Contains(c, sub) {
			n++
		}
	}
	return n
}

func TestShowResult_RevealsKeptDMG(t *testing.T) {
	for _, c := range []struct {
		outcome, text string
	}{
		{OutcomeRolledBack, "Portty 9.9.9 didn't start, so Portty put version 1.0.0 back. The download is shown in Finder."},
		{OutcomeFailed, "Portty couldn't install version 9.9.9. The download is shown in Finder."},
	} {
		t.Run(c.outcome, func(t *testing.T) {
			f, dmg := rbResultFix(t)
			r := Result{V: 1, Outcome: c.outcome, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: dmg}
			if !f.svc.ShowResult(context.Background(), r) {
				t.Fatal("ShowResult = false with the DMG present")
			}
			if want := (Status{State: "revealed", Current: "1.0.0", Latest: "9.9.9", Error: c.text}); f.svc.Status() != want {
				t.Errorf("status = %+v, want %+v", f.svc.Status(), want)
			}
			if strings.ContainsAny(c.text, "/") || strings.Contains(c.text, "://") {
				t.Errorf("text holds a slash: %q", c.text)
			}
			if rbCalls(f, "xattr -w com.apple.quarantine") != 1 || rbCalls(f, "open -R "+dmg) != 1 {
				t.Errorf("calls = %v; want the quarantine written back and one reveal", f.run.calls)
			}
			// Update again from the banner shows the kept DMG again instead of fetching it.
			if st, err := f.svc.Download(); err != nil || st.State != "revealed" || rbCalls(f, "open -R "+dmg) != 2 {
				t.Errorf("Download after ShowResult = %+v, %v; calls %v", st, err, f.run.calls)
			}
		})
	}
}

func TestShowResult_NoUsableDMG(t *testing.T) {
	for name, dmg := range map[string]func(t *testing.T, dir string) string{
		"missing": func(_ *testing.T, dir string) string { return filepath.Join(dir, DMGAssetName("9.9.9")+"x") },
		"outside": func(t *testing.T, _ string) string {
			p := filepath.Join(t.TempDir(), DMGAssetName("9.9.9"))
			_ = os.WriteFile(p, []byte("dmg"), 0o600)
			return p
		},
		"name": func(t *testing.T, dir string) string {
			p := filepath.Join(dir, "other.dmg")
			_ = os.WriteFile(p, []byte("dmg"), 0o600)
			return p
		},
		"symlink": func(t *testing.T, dir string) string {
			p := filepath.Join(dir, "portty-v9.9.9-darwin-arm64.dmg")
			_ = os.Symlink(DMGAssetName("9.9.9"), p)
			return p
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := rbResultFix(t)
			r := Result{V: 1, Outcome: OutcomeRolledBack, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: dmg(t, f.dir)}
			if f.svc.ShowResult(context.Background(), r) {
				t.Error("ShowResult = true")
			}
			if st := f.svc.Status(); st.State != "idle" || rbCalls(f, "open -R") != 0 {
				t.Errorf("status = %+v, calls %v; want unchanged and no reveal", st, f.run.calls)
			}
		})
	}
}

func TestShowResult_RecordsSkipWithoutDMG(t *testing.T) {
	f, dmg := rbResultFix(t)
	if err := os.Remove(dmg); err != nil {
		t.Fatal(err)
	}
	r := Result{V: 1, Outcome: OutcomeFailed, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: dmg}
	if f.svc.ShowResult(context.Background(), r) {
		t.Error("ShowResult = true without the DMG")
	}
	for _, s := range []*Service{f.svc, {Dir: f.dir}} { // the same Service and a fresh one after a restart
		if v, err := s.skippedVersion(); v != "9.9.9" || err != nil {
			t.Errorf("skippedVersion = %q, %v; want 9.9.9", v, err)
		}
	}
	if fi, err := os.Stat(filepath.Join(f.dir, skipFile)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("skip record: %v %v", fi, err)
	}
}

func TestShowResult_UpdatedRecordsNothing(t *testing.T) {
	f, dmg := rbResultFix(t)
	if f.svc.ShowResult(context.Background(), Result{V: 1, Outcome: OutcomeUpdated, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: dmg}) {
		t.Error("ShowResult = true for an updated result")
	}
	if v, err := f.svc.skippedVersion(); v != "" || err != nil {
		t.Errorf("skippedVersion = %q, %v; want no record", v, err)
	}
	if rbCalls(f, "open -R") != 0 {
		t.Errorf("calls = %v", f.run.calls)
	}
}

func TestSkippedVersion_UnreadableIsError(t *testing.T) {
	f, _ := rbResultFix(t)
	if v, err := f.svc.skippedVersion(); v != "" || err != nil {
		t.Fatalf("no record: %q, %v; want empty and no error", v, err)
	}
	rec := filepath.Join(f.dir, skipFile)
	check := func(name string) {
		t.Helper()
		v, err := f.svc.skippedVersion()
		if v != "" || err == nil {
			t.Errorf("%s: skippedVersion = %q, %v; want an error", name, v, err)
			return
		}
		if strings.Contains(err.Error(), "/") {
			t.Errorf("%s: error holds a path: %v", name, err)
		}
	}
	if err := os.Mkdir(rec, 0o700); err != nil {
		t.Fatal(err)
	}
	check("directory")
	_ = os.Remove(rec)
	for _, junk := range []string{"garbage\n", "../../etc\n", "\n"} {
		if err := os.WriteFile(rec, []byte(junk), 0o600); err != nil {
			t.Fatal(err)
		}
		check("content " + strings.TrimSpace(junk))
	}
	if os.Getuid() != 0 {
		if err := os.WriteFile(rec, []byte("9.9.9\n"), 0o000); err != nil {
			t.Fatal(err)
		}
		_ = os.Chmod(rec, 0o000)
		check("unreadable")
	}
	if _, err := (&Service{}).skippedVersion(); err == nil {
		t.Error("no cache folder: want an error, not an empty answer")
	}
}
