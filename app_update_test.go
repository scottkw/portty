package main

// UPD-01..UPD-04: the update poller cadence, update:status events, the once-per-version
// notification, the DownloadUpdate binding, the Help menu alerts and the tray item.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/updater"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var updateDMG = []byte(strings.Repeat("portty-dmg!", 4096))

// updateFake is a fake GitHub serving version (404 when ""), or status when non-zero, with an
// ETag (304 on a match), and the DMG at /dl/.
type updateFake struct {
	*httptest.Server
	mu      sync.Mutex
	version string
	status  int
	noAsset bool // the release has no DMG yet
	checks  int
	assets  int
	onAsset func() // runs inside the asset handler before it answers
}

func (f *updateFake) set(version string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.version, f.status = version, status
}

func (f *updateFake) counts() (checks, assets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks, f.assets
}

func (f *updateFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	version, status, onAsset, noAsset := f.version, f.status, f.onAsset, f.noAsset
	if strings.HasPrefix(r.URL.Path, "/dl/") {
		f.assets++
	} else {
		f.checks++
	}
	f.mu.Unlock()
	if strings.HasPrefix(r.URL.Path, "/dl/") {
		if onAsset != nil {
			onAsset()
		}
		_, _ = w.Write(updateDMG)
		return
	}
	if status != 0 {
		w.WriteHeader(status)
		return
	}
	if version == "" {
		http.NotFound(w, r)
		return
	}
	etag := fmt.Sprintf(`"%s-%v"`, version, noAsset)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	sum := sha256.Sum256(updateDMG)
	name := updater.DMGAssetName(version)
	assets := []map[string]any{{
		"name": name, "size": len(updateDMG), "state": "uploaded", "digest": "sha256:" + hex.EncodeToString(sum[:]),
		"browser_download_url": f.URL + "/dl/" + name,
	}}
	if noAsset {
		assets = nil
	}
	w.Header().Set("ETag", etag)
	_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v" + version, "assets": assets})
}

// updateHarness is an App on release version 1.0.0 whose update service talks to an updateFake,
// with every update seam recorded. Answer is what the dialog seam returns.
type updateHarness struct {
	app *App
	gh  *updateFake

	mu      sync.Mutex
	events  []updater.Status
	names   []string // every emitted event name, update:status or not
	notes   []string
	tray    []string
	dialogs []string // "Title|Message|Buttons|Default|Cancel"
	shows   int
	answer  string
	now     time.Time

	waiting chan time.Duration
	ticks   chan time.Time
	waits   []time.Duration
}

func setVersion(t *testing.T, v string) {
	t.Helper()
	old := Version
	Version = v
	t.Cleanup(func() { Version = old })
}

func newUpdateHarness(t *testing.T) *updateHarness {
	t.Helper()
	setVersion(t, "v1.0.0")
	// os.UserConfigDir and os.UserCacheDir read $HOME on darwin: update_notified and the updates
	// folder land in a temp dir, never in the developer's real Portty files.
	t.Setenv("HOME", t.TempDir())
	gh := &updateFake{}
	gh.Server = httptest.NewServer(http.HandlerFunc(gh.serve))
	t.Cleanup(gh.Close)
	oldAPI, oldAllow := updateAPI, updateAllow
	host := strings.TrimPrefix(gh.URL, "http://")
	updateAPI, updateAllow = gh.URL, func(u *url.URL) bool { return u.Scheme == "http" && u.Host == host }
	t.Cleanup(func() { updateAPI, updateAllow = oldAPI, oldAllow })

	h := &updateHarness{gh: gh, now: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC),
		waiting: make(chan time.Duration, 1), ticks: make(chan time.Time)}
	h.app = &App{
		emitEventFunc: func(name string, data any) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.names = append(h.names, name)
			if name != "update:status" {
				return
			}
			h.events = append(h.events, data.(updater.Status))
		},
		sendNotificationFunc: func(id, title, body string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.notes = append(h.notes, id+"|"+title+"|"+body)
		},
		updateAfter: func(d time.Duration) <-chan time.Time { h.waiting <- d; return h.ticks },
		updateNow: func() time.Time {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.now
		},
		messageDialogFunc: func(o runtime.MessageDialogOptions) (string, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.dialogs = append(h.dialogs, strings.Join([]string{o.Title, o.Message, strings.Join(o.Buttons, " "), o.DefaultButton, o.CancelButton}, "|"))
			return h.answer, nil
		},
		setTrayUpdateFunc: func(title string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.tray = append(h.tray, title)
		},
		showWindowFunc: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.shows++
		},
	}
	h.app.updates = h.app.newUpdateService()
	h.app.updates.Run = fakeUpdateRun
	return h
}

// fakeUpdateRun answers codesign, spctl, xattr and open like a good notarized DMG.
func fakeUpdateRun(_ context.Context, name string, args ...string) ([]byte, error) {
	switch {
	case strings.HasSuffix(name, "codesign") && len(args) > 0 && args[0] == "-dvv":
		return []byte("Identifier=portty-v9.9.9-darwin-universal\nTeamIdentifier=" + updater.TeamID + "\n"), nil
	case strings.HasSuffix(name, "spctl"):
		return []byte("x.dmg: accepted\nsource=Notarized Developer ID\n"), nil
	}
	return nil, nil
}

func (h *updateHarness) setNow(t time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = t
}

func (h *updateHarness) states(state string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, e := range h.events {
		if e.State == state {
			n++
		}
	}
	return n
}

func (h *updateHarness) notifications() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.notes...)
}

// start runs the update loop with the fake timer and waits until it asks for its first wait.
// It returns a channel closed when the loop returns.
func (h *updateHarness) start(t *testing.T, ctx context.Context) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		h.app.runUpdateLoop(ctx)
		close(done)
	}()
	h.next(t)
	return done
}

func (h *updateHarness) next(t *testing.T) {
	t.Helper()
	select {
	case d := <-h.waiting:
		h.waits = append(h.waits, d)
	case <-time.After(10 * time.Second):
		t.Fatal("the update loop is not waiting")
	}
}

// tick fires the pending wait and returns once the loop waits again, so that iteration's check
// has finished.
func (h *updateHarness) tick(t *testing.T) {
	t.Helper()
	h.ticks <- time.Time{}
	h.next(t)
}

func notifiedFile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(notifiedVersionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func TestUpdatePoller_FirstCheckAfterDelay(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.app.startUpdatePoller(ctx)
	h.next(t)
	if h.waits[0] != 10*time.Second {
		t.Errorf("first wait %v, want 10s", h.waits[0])
	}
	time.Sleep(50 * time.Millisecond)
	if c, _ := h.gh.counts(); c != 0 {
		t.Fatalf("%d requests before the first delay fired", c)
	}
	h.tick(t)
	if c, _ := h.gh.counts(); c != 1 {
		t.Errorf("%d requests after the first delay, want 1", c)
	}
	if n := h.states("available"); n != 1 {
		t.Errorf("%d update:status available events, want 1", n)
	}
	if got, want := strings.Join(h.notifications(), ";"), "portty.update-available|Portty|Version 9.9.9 is available."; got != want {
		t.Errorf("notifications %q, want %q", got, want)
	}
	if got := notifiedFile(t); got != "9.9.9" {
		t.Errorf("update_notified = %q, want 9.9.9", got)
	}
	if fi, err := os.Stat(notifiedVersionPath()); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("update_notified mode: %v %v", fi, err)
	}
	if st := h.app.GetUpdateStatus(); st.State != "available" || st.Latest != "9.9.9" || st.Current != "1.0.0" {
		t.Errorf("GetUpdateStatus = %+v", st)
	}
}

func TestUpdatePoller_DailyCadenceAndNoRepeat(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.start(t, ctx)
	t0 := h.now
	h.tick(t) // the launch check
	for _, d := range []time.Duration{time.Hour, 12 * time.Hour, 23*time.Hour + 59*time.Minute} {
		h.setNow(t0.Add(d))
		h.tick(t)
		if c, _ := h.gh.counts(); c != 1 {
			t.Fatalf("%d requests %v after the last check, want 1", c, d)
		}
	}
	h.setNow(t0.Add(24 * time.Hour))
	h.tick(t)
	if c, _ := h.gh.counts(); c != 2 {
		t.Fatalf("%d requests 24h after the last check, want 2", c)
	}
	if n := len(h.notifications()); n != 1 {
		t.Errorf("%d notifications after the 304 check, want 1", n)
	}
	h.gh.set("9.9.10", 0)
	h.setNow(t0.Add(47 * time.Hour))
	h.tick(t)
	if c, _ := h.gh.counts(); c != 2 {
		t.Fatalf("a check ran 23h after the last one")
	}
	h.setNow(t0.Add(48 * time.Hour))
	h.tick(t)
	got := h.notifications()
	if len(got) != 2 || got[1] != "portty.update-available|Portty|Version 9.9.10 is available." {
		t.Errorf("notifications %q", got)
	}
	if notifiedFile(t) != "9.9.10" {
		t.Errorf("update_notified = %q", notifiedFile(t))
	}
	for i, d := range h.waits[1:] {
		if d != time.Hour {
			t.Errorf("wait %d = %v, want the hourly tick", i+1, d)
		}
	}
}

func TestUpdatePoller_FailureRetriedHourlyAndQuiet(t *testing.T) {
	for _, status := range []int{500, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			h := newUpdateHarness(t)
			h.gh.set("", status)
			ctx, cancel := context.WithCancel(context.Background())
			done := h.start(t, ctx)
			h.tick(t)
			h.tick(t) // the very next hourly tick retries
			if c, _ := h.gh.counts(); c != 2 {
				t.Errorf("%d requests, want 2 (failure retried on the next tick)", c)
			}
			if h.states("available") != 0 || len(h.notifications()) != 0 {
				t.Errorf("a failed check produced available or a notification")
			}
			h.gh.set("9.9.9", 0)
			h.tick(t)
			if h.states("available") != 1 {
				t.Errorf("the loop stopped after failures")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("the loop did not stop when ctx ended")
			}
		})
	}
}

func TestUpdatePoller_DevBuildNeverChecks(t *testing.T) {
	for _, v := range []string{"dev", "v1.2.3-beta.1", ""} {
		h := newUpdateHarness(t)
		setVersion(t, v)
		fired := make(chan time.Time)
		close(fired)
		called := make(chan struct{}, 10)
		h.app.updateAfter = func(time.Duration) <-chan time.Time { called <- struct{}{}; return fired }
		ctx, cancel := context.WithCancel(context.Background())
		h.app.startUpdatePoller(ctx)
		time.Sleep(100 * time.Millisecond)
		cancel()
		if c, _ := h.gh.counts(); c != 0 || len(called) != 0 {
			t.Errorf("Version %q: %d requests, %d timers", v, c, len(called))
		}
	}
}

func TestDownloadUpdate_BindingRunsDownload(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := h.app.DownloadUpdate()
	if st.State != "revealed" || st.Latest != "9.9.9" || st.Error != "" {
		t.Errorf("DownloadUpdate = %+v", st)
	}
	for _, s := range []string{"downloading", "verifying", "revealed"} {
		if h.states(s) == 0 {
			t.Errorf("no update:status %s event", s)
		}
	}
	if _, assets := h.gh.counts(); assets != 1 {
		t.Errorf("%d asset requests, want 1", assets)
	}

	if st := (&App{}).DownloadUpdate(); st.State != "idle" {
		t.Errorf("nil updates: DownloadUpdate = %+v", st)
	}
}

func (h *updateHarness) dialogList() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.dialogs...)
}

func (h *updateHarness) showCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.shows
}

func (h *updateHarness) trayList() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.tray...)
}

// oneDialog checks that exactly one alert was shown and returns it.
func (h *updateHarness) oneDialog(t *testing.T) string {
	t.Helper()
	d := h.dialogList()
	if len(d) != 1 {
		t.Fatalf("dialogs %q, want one", d)
	}
	return d[0]
}

const availableAlert = "Version 9.9.9 is available.|You have version 1.0.0.|Update Later|Update|Later"

func TestManualUpdateCheck_UpToDate(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("1.0.0", 0)
	h.app.checkForUpdatesManually()
	if got, want := h.oneDialog(t), "Portty is up to date.|Version 1.0.0 is the newest version.|OK|OK|"; got != want {
		t.Errorf("alert %q, want %q", got, want)
	}
}

func TestManualUpdateCheck_AvailableLater(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	h.answer = "Later"
	h.app.checkForUpdatesManually()
	if got := h.oneDialog(t); got != availableAlert {
		t.Errorf("alert %q, want %q", got, availableAlert)
	}
	if _, assets := h.gh.counts(); assets != 0 || h.showCount() != 0 {
		t.Errorf("Later: %d asset requests, %d window shows", assets, h.showCount())
	}
	if n := len(h.notifications()); n != 0 {
		t.Errorf("a manual check sent %d notifications", n)
	}
	if notifiedFile(t) != "9.9.9" {
		t.Errorf("update_notified = %q, want 9.9.9", notifiedFile(t))
	}
}

func TestManualUpdateCheck_AvailableUpdate(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	h.answer = "Update"
	h.app.checkForUpdatesManually()
	if got := h.oneDialog(t); got != availableAlert {
		t.Errorf("alert %q", got)
	}
	if h.showCount() != 1 {
		t.Errorf("%d window shows, want 1", h.showCount())
	}
	if st := h.app.GetUpdateStatus(); st.State != "revealed" {
		t.Errorf("after Update: %+v", st)
	}
}

func TestManualUpdateCheck_Error(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int // 0: the server is closed
		text   string
	}{
		{"closed", 0, "Portty couldn't reach GitHub. Check your internet connection and try again."},
		{"403", 403, "GitHub is limiting update checks from this network. Try again in an hour."},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newUpdateHarness(t)
			if c.status == 0 {
				h.gh.Close()
			} else {
				h.gh.set("", c.status)
			}
			h.app.checkForUpdatesManually()
			if got, want := h.oneDialog(t), "Couldn't check for updates.|"+c.text+"|OK|OK|"; got != want {
				t.Errorf("alert %q, want %q", got, want)
			}
			if st := h.app.GetUpdateStatus(); st.State == "available" {
				t.Errorf("state %+v", st)
			}
		})
	}
}

func TestManualUpdateCheck_DevBuild(t *testing.T) {
	h := newUpdateHarness(t)
	setVersion(t, "dev")
	h.gh.set("9.9.9", 0)
	h.app.checkForUpdatesManually()
	want := "This build doesn't check for updates.|Only release builds of Portty check GitHub for new versions.|OK|OK|"
	if got := h.oneDialog(t); got != want {
		t.Errorf("alert %q, want %q", got, want)
	}
	if c, _ := h.gh.counts(); c != 0 {
		t.Errorf("%d requests from a dev build", c)
	}
}

// blockDownload makes h available and starts a download that waits in the asset handler until
// the returned func is called; that func waits for the download to finish.
func (h *updateHarness) blockDownload(t *testing.T) func() {
	t.Helper()
	h.gh.set("9.9.9", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	h.gh.mu.Lock()
	h.gh.onAsset = func() { entered <- struct{}{}; <-release }
	h.gh.mu.Unlock()
	go func() { h.app.DownloadUpdate(); close(done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the download never reached the asset")
	}
	var once sync.Once
	finish := func() { once.Do(func() { close(release); <-done }) }
	t.Cleanup(finish)
	return finish
}

// holdAt makes h available and runs a Download whose install is held at state: "downloading" in
// the asset handler, "installing" in the first hdiutil attach, "relaunching" in StartInstaller.
// The returned func releases the hold and waits for the Download to finish; a held installer then
// fails, which falls back to the reveal flow.
func (h *updateHarness) holdAt(t *testing.T, state string) func() {
	t.Helper()
	if state == "downloading" {
		return h.blockDownload(t)
	}
	h.gh.set("9.9.9", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	hold := func() { entered <- struct{}{}; <-release }
	h.app.updates.Bundle = filepath.Join(t.TempDir(), "Portty.app")
	h.app.updates.StartInstaller = func(updater.Job) error {
		if state == "relaunching" {
			hold()
		}
		return errors.New("installer held")
	}
	h.app.updates.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch {
		case strings.HasSuffix(name, "hdiutil") && args[0] == "attach":
			if err := os.Mkdir(filepath.Join(args[len(args)-2], updater.BundleName), 0o700); err != nil {
				return nil, err
			}
			if state == "installing" {
				hold()
			}
		case strings.HasSuffix(name, "hdiutil") && args[0] == "detach":
			return nil, os.RemoveAll(args[len(args)-1])
		case strings.HasSuffix(name, "ditto"):
			return nil, os.MkdirAll(args[len(args)-1], 0o700)
		}
		return fakeUpdateRun(ctx, name, args...)
	}
	go func() { h.app.DownloadUpdate(); close(done) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatalf("the install never reached %s", state)
	}
	if got := h.app.GetUpdateStatus().State; got != state {
		t.Fatalf("held status %q, want %q", got, state)
	}
	var once sync.Once
	finish := func() { once.Do(func() { close(release); <-done }) }
	t.Cleanup(finish)
	return finish
}

func TestManualUpdateCheck_WhileUpdating(t *testing.T) {
	for _, state := range []string{"downloading", "installing", "relaunching"} {
		t.Run(state, func(t *testing.T) {
			h := newUpdateHarness(t)
			finish := h.holdAt(t, state)
			h.app.checkForUpdatesManually()
			want := "Portty is updating to version 9.9.9.|It will restart by itself when the update is installed.|OK|OK|"
			if got := h.oneDialog(t); got != want {
				t.Errorf("alert %q, want %q", got, want)
			}
			finish()
		})
	}
}

func TestManualUpdateCheck_WhileVerifying(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	h.app.updates.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if strings.HasSuffix(name, "codesign") {
			entered <- struct{}{}
			<-release
		}
		return fakeUpdateRun(ctx, name, args...)
	}
	go func() { h.app.DownloadUpdate(); close(done) }()
	<-entered
	h.app.checkForUpdatesManually()
	want := "Portty is updating to version 9.9.9.|It will restart by itself when the update is installed.|OK|OK|"
	if got := h.oneDialog(t); got != want {
		t.Errorf("alert %q, want %q", got, want)
	}
	close(release)
	<-done
}

func TestTrayUpdate_FollowsStatus(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("9.9.9", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.trayList(); len(got) != 1 || got[0] != "Update Available…" {
		t.Fatalf("tray after available %q (checking must not touch it)", got)
	}
	h.gh.set("1.0.0", 0)
	if _, err := h.app.updates.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := h.trayList(); len(got) != 2 || got[1] != "" {
		t.Errorf("tray after current %q", got)
	}
}

// A failed or rolled-back install ends revealed with an Error: the tray stops offering that version
// (the banner shows the failure), while the other states keep the item.
func TestTrayUpdate_HiddenAfterFailedInstall(t *testing.T) {
	for _, c := range []struct {
		name string
		st   updater.Status
		want string
	}{
		{"revealed with error", updater.Status{State: "revealed", Latest: "9.9.9", Error: "Portty couldn't install the update, so the download is shown in Finder."}, ""},
		{"revealed", updater.Status{State: "revealed", Latest: "9.9.9"}, "Update Available…"},
		{"available", updater.Status{State: "available", Latest: "9.9.9"}, "Update Available…"},
		{"available with download error", updater.Status{State: "available", Latest: "9.9.9", Error: "Portty couldn't download version 9.9.9. Try again later."}, "Update Available…"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newUpdateHarness(t)
			h.app.onUpdateStatus(c.st)
			if got := h.trayList(); len(got) != 1 || got[0] != c.want {
				t.Errorf("tray %q, want [%q]", got, c.want)
			}
		})
	}
}

func TestTrayUpdate_OfferUpdate(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		h := newUpdateHarness(t)
		h.gh.set("9.9.9", 0)
		if _, err := h.app.updates.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		h.answer = "Later"
		h.app.offerUpdate()
		if h.showCount() != 1 {
			t.Errorf("%d window shows, want 1", h.showCount())
		}
		if got := h.oneDialog(t); got != availableAlert {
			t.Errorf("alert %q", got)
		}
	})
	t.Run("downloading", func(t *testing.T) {
		h := newUpdateHarness(t)
		finish := h.blockDownload(t)
		h.app.offerUpdate()
		if h.showCount() != 1 || len(h.dialogList()) != 0 {
			t.Errorf("%d shows, dialogs %q; want one show, no alert", h.showCount(), h.dialogList())
		}
		finish()
	})
	for _, state := range []string{"installing", "relaunching"} {
		t.Run(state, func(t *testing.T) {
			h := newUpdateHarness(t)
			finish := h.holdAt(t, state)
			h.app.offerUpdate()
			if h.showCount() != 1 || len(h.dialogList()) != 0 {
				t.Errorf("%d shows, dialogs %q; want one show, no alert", h.showCount(), h.dialogList())
			}
			if _, assets := h.gh.counts(); assets != 1 {
				t.Errorf("%d asset requests, want 1 (no second download)", assets)
			}
			finish()
		})
	}
	t.Run("revealed", func(t *testing.T) { // IN-04: Update shows the kept DMG again
		h := newUpdateHarness(t)
		h.gh.set("9.9.9", 0)
		if _, err := h.app.updates.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		if st := h.app.DownloadUpdate(); st.State != "revealed" {
			t.Fatalf("DownloadUpdate = %+v", st)
		}
		h.answer = "Update"
		h.app.offerUpdate()
		if got := h.oneDialog(t); got != availableAlert {
			t.Errorf("alert %q", got)
		}
		if _, assets := h.gh.counts(); assets != 1 || h.app.GetUpdateStatus().State != "revealed" {
			t.Errorf("%d asset requests (want 1), status %+v", assets, h.app.GetUpdateStatus())
		}
	})
	t.Run("current", func(t *testing.T) {
		h := newUpdateHarness(t)
		h.gh.set("1.0.0", 0)
		if _, err := h.app.updates.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		h.app.offerUpdate()
		if h.showCount() != 0 || len(h.dialogList()) != 0 {
			t.Errorf("current: %d shows, dialogs %q", h.showCount(), h.dialogList())
		}
	})
}

// WR-03: the harness never touches the real user's update_notified or updates folder.
func TestUpdateHarness_IsolatesUserFiles(t *testing.T) {
	realHome := os.Getenv("HOME")
	h := newUpdateHarness(t)
	for _, p := range []string{notifiedVersionPath(), h.app.updates.Dir} {
		if lib := filepath.Join(realHome, "Library") + string(os.PathSeparator); realHome != "" && strings.HasPrefix(p, lib) {
			t.Errorf("%s is in the real %s", p, lib)
		}
	}
}

const notReadyText = "Version 9.9.9 isn't ready to download yet. Try again later."

// WR-04: a newer release without its DMG yet is an incomplete check: the poller retries it on the
// next hourly tick, quietly, and offers the update once the DMG is there.
func TestUpdatePoller_NotReadyRetriedHourly(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.mu.Lock()
	h.gh.version, h.gh.noAsset = "9.9.9", true
	h.gh.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.start(t, ctx)
	h.tick(t)
	h.tick(t)
	if c, _ := h.gh.counts(); c != 2 {
		t.Errorf("%d requests, want 2 (retried on the next tick)", c)
	}
	if h.states("available") != 0 || len(h.notifications()) != 0 || len(h.dialogList()) != 0 {
		t.Errorf("a not-ready release produced available, a notification or an alert")
	}
	h.gh.mu.Lock()
	h.gh.noAsset = false
	h.gh.mu.Unlock()
	h.tick(t)
	if h.states("available") != 1 || len(h.notifications()) != 1 {
		t.Errorf("the DMG arrived but no update was offered")
	}
}

// WR-04: the manual check says the newer version isn't ready, never "up to date".
func TestManualUpdateCheck_NotReady(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.mu.Lock()
	h.gh.version, h.gh.noAsset = "9.9.9", true
	h.gh.mu.Unlock()
	h.app.checkForUpdatesManually()
	if got, want := h.oneDialog(t), "Couldn't check for updates.|"+notReadyText+"|OK|OK|"; got != want {
		t.Errorf("alert %q, want %q", got, want)
	}
}

// IN-01: a check completed while the clock was ahead; once the clock is set back, the next tick
// checks instead of waiting for the wall clock to pass the old future time.
func TestUpdatePoller_ClockSetBack(t *testing.T) {
	h := newUpdateHarness(t)
	h.gh.set("1.0.0", 0)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.start(t, ctx)
	t0 := h.now
	h.tick(t) // the launch check, at t0
	h.setNow(t0.Add(-30 * 24 * time.Hour))
	h.tick(t)
	if c, _ := h.gh.counts(); c != 2 {
		t.Fatalf("%d requests after the clock went back, want 2", c)
	}
	h.setNow(t0.Add(-30*24*time.Hour + time.Hour))
	h.tick(t)
	if c, _ := h.gh.counts(); c != 2 {
		t.Errorf("%d requests an hour later, want 2 (the daily cadence resumes)", c)
	}
}

// eventCount is how many times the event name was emitted.
func (h *updateHarness) eventCount(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, got := range h.names {
		if got == name {
			n++
		}
	}
	return n
}

// A click on the update notification brings Portty forward and asks for the banner; it must not
// start the update (CONTEXT decision 2026-11-22).
func TestNotificationClick_ShowsWindowAndBanner(t *testing.T) {
	h := newUpdateHarness(t)
	h.app.handleNotificationClick("portty.update-available")
	if n := h.showCount(); n != 1 {
		t.Errorf("shows = %d, want 1", n)
	}
	if n := h.eventCount("update:show"); n != 1 {
		t.Errorf("update:show emitted %d times, want 1", n)
	}
	if _, assets := h.gh.counts(); assets != 0 {
		t.Errorf("%d asset downloads after a click, want 0", assets)
	}
	if n := h.states("downloading"); n != 0 {
		t.Errorf("%d downloading status events after a click, want 0", n)
	}
}

func TestNotificationClick_IgnoresOtherIDs(t *testing.T) {
	h := newUpdateHarness(t)
	h.app.handleNotificationClick("portty.quit-gui-only")
	h.app.handleNotificationClick("")
	if n := h.showCount(); n != 0 {
		t.Errorf("shows = %d, want 0", n)
	}
	if n := h.eventCount("update:show"); n != 0 {
		t.Errorf("update:show emitted %d times, want 0", n)
	}
}
