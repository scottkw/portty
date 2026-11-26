package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/updater"
)

// rbNote is one notification the App sent.
type rbNote struct{ id, title, body string }

// rbApp is an App whose notifications are recorded and whose events go nowhere.
func rbApp(t *testing.T) (*App, *[]rbNote) {
	t.Helper()
	var notes []rbNote
	a := &App{
		emitEventFunc:        func(string, any) {},
		sendNotificationFunc: func(id, title, body string) { notes = append(notes, rbNote{id, title, body}) },
	}
	t.Cleanup(func() { _ = os.Remove(installResultPath()) })
	return a, &notes
}

func rbWriteResult(t *testing.T, r updater.Result) {
	t.Helper()
	r.V = 1
	if err := updater.WriteResult(installResultPath(), r); err != nil {
		t.Fatal(err)
	}
}

// rbSwapEnsure records every EnsureOptions the default ensure seam passes.
func rbSwapEnsure(t *testing.T) *[]daemon.EnsureOptions {
	t.Helper()
	var got []daemon.EnsureOptions
	orig := ensureDaemon
	ensureDaemon = func(_ context.Context, _ string, opts daemon.EnsureOptions) error {
		got = append(got, opts)
		return nil
	}
	t.Cleanup(func() { ensureDaemon = orig })
	return &got
}

func TestConsumeInstallResult_KeepDaemonPolicyOnce(t *testing.T) {
	opts := rbSwapEnsure(t)
	a, _ := rbApp(t)
	rbWriteResult(t, updater.Result{Outcome: updater.OutcomeRolledBack, OldVersion: "1.8.0", NewVersion: "1.8.1", Reason: "gui-exited", KeepDaemon: true})
	a.consumeInstallResult()
	for range 2 {
		if err := a.ensure()("sock"); err != nil {
			t.Fatal(err)
		}
	}
	if len(*opts) != 2 {
		t.Fatalf("ensures = %d", len(*opts))
	}
	first, second := (*opts)[0], (*opts)[1]
	if !first.NoHandoff || first.Restart == nil || first.Restart(question(2)) || first.Restart(question(-1)) {
		t.Errorf("first ensure after a rollback: NoHandoff %v; want NoHandoff and a policy that says no", first.NoHandoff)
	}
	if a.GetDaemonRestartPrompt() != 0 {
		t.Error("the first ensure after a rollback raised the restart question")
	}
	if second.NoHandoff || second.Restart == nil || second.Restart(question(2)) || a.GetDaemonRestartPrompt() != 2 {
		t.Errorf("second ensure: NoHandoff %v, prompt %d; want today's restartConsent", second.NoHandoff, a.GetDaemonRestartPrompt())
	}
}

func TestConsumeInstallResult_TellsUser(t *testing.T) {
	for _, c := range []struct {
		outcome, body string
	}{
		{updater.OutcomeRolledBack, "The update to Portty 1.8.1 didn't take. Portty 1.8.0 is still installed."},
		{updater.OutcomeFailed, "Portty couldn't install version 1.8.1."},
	} {
		t.Run(c.outcome, func(t *testing.T) {
			a, notes := rbApp(t)
			dir := t.TempDir()
			dmg := filepath.Join(dir, updater.DMGAssetName("1.8.1"))
			if err := os.WriteFile(dmg, []byte("dmg"), 0o600); err != nil {
				t.Fatal(err)
			}
			var calls []string
			a.updates = &updater.Service{Current: "1.8.0", Dir: dir, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls = append(calls, name+" "+strings.Join(args, " "))
				return nil, nil
			}}
			rbWriteResult(t, updater.Result{Outcome: c.outcome, OldVersion: "1.8.0", NewVersion: "1.8.1", Reason: "health", DMG: dmg})
			a.consumeInstallResult()

			if st := a.updates.Status(); st.State != "revealed" || st.Latest != "1.8.1" || st.Error == "" {
				t.Errorf("status = %+v; want the banner's revealed result", st)
			}
			if len(*notes) != 1 || (*notes)[0] != (rbNote{updateResultNotificationID, "Portty", c.body}) {
				t.Errorf("notifications = %+v", *notes)
			}
			if strings.Contains((*notes)[0].body, "/") {
				t.Errorf("notification body holds a slash: %q", (*notes)[0].body)
			}
			if _, err := os.Lstat(installResultPath()); !os.IsNotExist(err) {
				t.Error("result file not consumed")
			}
		})
	}
}

func TestConsumeInstallResult_UpdatedAndBadFilesChangeNothing(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"updated": func(t *testing.T) {
			rbWriteResult(t, updater.Result{Outcome: updater.OutcomeUpdated, OldVersion: "1.8.0", NewVersion: "1.8.1"})
		},
		"missing": func(*testing.T) {},
		"malformed": func(t *testing.T) {
			if err := os.WriteFile(installResultPath(), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"not ours (mode)": func(t *testing.T) {
			if err := os.WriteFile(installResultPath(), []byte(`{"v":1,"outcome":"rolled-back","keepDaemon":true}`), 0o644); err != nil {
				t.Fatal(err)
			}
			_ = os.Chmod(installResultPath(), 0o644)
		},
		"symlink": func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "r.json")
			if err := os.WriteFile(src, []byte(`{"v":1,"outcome":"rolled-back","keepDaemon":true}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(src, installResultPath()); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			a, notes := rbApp(t)
			a.updates = &updater.Service{Current: "1.8.0", Dir: t.TempDir()}
			set(t)
			a.consumeInstallResult()
			if a.keepDaemonOnce.Load() || len(*notes) != 0 || a.updates.Status().State != "idle" {
				t.Errorf("keepDaemon %v, notes %v, status %+v; want nothing changed", a.keepDaemonOnce.Load(), *notes, a.updates.Status())
			}
			if _, err := os.Lstat(installResultPath()); !os.IsNotExist(err) {
				t.Error("result file left in place")
			}
		})
	}
}
