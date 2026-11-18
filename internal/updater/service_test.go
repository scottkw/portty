package updater

import (
	"context"
	"crypto/sha256"
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
	"sync/atomic"
	"testing"
	"time"
)

// ghFake is a fake GitHub: the latest-release route answers body (404 when nil, status when
// set, 304 when If-None-Match matches its ETag) and /dl/ serves assetBody.
type ghFake struct {
	*httptest.Server
	mu          sync.Mutex
	body        any
	status      int
	noneMatch   []string // If-None-Match of each latest-release request
	onLatest    func()   // runs inside the latest-release handler before it answers
	onAsset     func()   // runs inside the asset handler before it answers
	assetBody   []byte
	assetStatus int
	assets      atomic.Int32
}

func newGH(t *testing.T) *ghFake {
	t.Helper()
	g := &ghFake{assetBody: payload}
	g.Server = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.Close)
	return g
}

func (g *ghFake) set(body any, status int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.body, g.status = body, status
}

func (g *ghFake) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	body, status, onLatest, onAsset, asset, assetStatus := g.body, g.status, g.onLatest, g.onAsset, g.assetBody, g.assetStatus
	if r.URL.Path == "/repos/"+Repo+"/releases/latest" {
		g.noneMatch = append(g.noneMatch, r.Header.Get("If-None-Match"))
	}
	g.mu.Unlock()
	switch {
	case r.URL.Path == "/repos/"+Repo+"/releases/latest":
		if onLatest != nil {
			onLatest()
		}
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		if body == nil {
			http.NotFound(w, r)
			return
		}
		b, _ := json.Marshal(body)
		etag := fmt.Sprintf(`"%x"`, sha256.Sum256(b))
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write(b)
	case strings.HasPrefix(r.URL.Path, "/dl/"):
		g.assets.Add(1)
		if onAsset != nil {
			onAsset()
		}
		if assetStatus != 0 {
			w.WriteHeader(assetStatus)
			return
		}
		_, _ = w.Write(asset)
	default:
		http.NotFound(w, r)
	}
}

func (g *ghFake) checks() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.noneMatch...)
}

// rel is a published release of v whose DMG is the fake's /dl/ route with payload's digest.
func (g *ghFake) rel(v string) map[string]any {
	name := DMGAssetName(v)
	return map[string]any{"tag_name": "v" + v, "draft": false, "prerelease": false, "assets": []map[string]any{{
		"name": name, "size": len(payload), "state": "uploaded", "digest": "sha256:" + sum(payload),
		"browser_download_url": g.URL + "/dl/" + name,
	}}}
}

type emits struct {
	mu  sync.Mutex
	all []Status
}

func (e *emits) emit(s Status) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.all = append(e.all, s)
}

func (e *emits) list() []Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Status(nil), e.all...)
}

func (e *emits) states() string {
	var s []string
	for _, st := range e.list() {
		s = append(s, st.State)
	}
	return strings.Join(s, ",")
}

func (e *emits) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.all = nil
}

type svcFix struct {
	svc *Service
	gh  *ghFake
	ev  *emits
	run *fakeRun
	dir string
}

// newSvc is a Service on running version 1.0.0 against a fresh fake GitHub.
func newSvc(t *testing.T) svcFix {
	t.Helper()
	g := newGH(t)
	host := strings.TrimPrefix(g.URL, "http://")
	allow := func(u *url.URL) bool { return u.Scheme == "http" && u.Host == host }
	ev := &emits{}
	run := &fakeRun{team: goodDVV, spctl: goodSpctl}
	dir := filepath.Join(t.TempDir(), "updates")
	svc := &Service{
		Checker: &Checker{API: g.URL, Repo: Repo, HTTP: NewCheckClient(g.URL), Allow: allow},
		Current: "1.0.0",
		Emit:    ev.emit,
		Dir:     dir,
		Client:  NewDownloadClient(allow),
		Run:     run.run,
		Now:     func() time.Time { return time.Unix(0x6720_0000, 0) },
	}
	return svcFix{svc, g, ev, run, dir}
}

const (
	netText  = "Portty couldn't reach GitHub. Check your internet connection and try again."
	rateText = "GitHub is limiting update checks from this network. Try again in an hour."
)

func TestService_CheckAvailable(t *testing.T) {
	f := newSvc(t)
	f.gh.set(f.gh.rel("9.9.9"), 0)
	st, err := f.svc.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := (Status{State: "available", Current: "1.0.0", Latest: "9.9.9"}); st != want {
		t.Errorf("Check = %+v, want %+v", st, want)
	}
	if got := f.ev.states(); got != "checking,available" {
		t.Errorf("emitted %s", got)
	}
	if got := f.svc.Status(); got != st {
		t.Errorf("Status = %+v", got)
	}
}

func TestService_CheckQuietOutcomes(t *testing.T) {
	for name, body := range map[string]func(g *ghFake) any{
		"404":          func(*ghFake) any { return nil },
		"draft":        func(g *ghFake) any { r := g.rel("9.9.9"); r["draft"] = true; return r },
		"prerelease":   func(g *ghFake) any { r := g.rel("9.9.9"); r["prerelease"] = true; return r },
		"tag latest":   func(g *ghFake) any { r := g.rel("9.9.9"); r["tag_name"] = "latest"; return r },
		"same version": func(g *ghFake) any { return g.rel("1.0.0") },
		"same version, no DMG": func(g *ghFake) any {
			r := g.rel("1.0.0")
			r["assets"] = []map[string]any{}
			return r
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSvc(t)
			f.gh.set(body(f.gh), 0)
			st, err := f.svc.Check(context.Background())
			if err != nil || st.State != "current" || st.Latest != "" || st.Error != "" {
				t.Errorf("Check = %+v, %v; want current", st, err)
			}
			if strings.Contains(f.ev.states(), "available") {
				t.Errorf("emitted %s", f.ev.states())
			}
		})
	}
}

func TestService_CheckFailures(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int // 0: the server is closed
		text   string
	}{
		{"closed", 0, netText},
		{"500", 500, netText},
		{"403", 403, rateText},
		{"429", 429, rateText},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newSvc(t)
			f.gh.set(f.gh.rel("9.9.9"), 0)
			if _, err := f.svc.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
			// A fresh Checker drops the ETag cache, so the failing answer is really read.
			f.svc.Checker = &Checker{API: f.gh.URL, Repo: Repo, HTTP: NewCheckClient(f.gh.URL), Allow: f.svc.Checker.Allow}
			if c.status == 0 {
				f.gh.Close()
			} else {
				f.gh.set(nil, c.status)
			}
			f.ev.reset()
			st, err := f.svc.Check(context.Background())
			var e *Error
			if !errors.As(err, &e) || e.Kind != "check" || e.Text != c.text || err.Error() != c.text {
				t.Errorf("err = %#v, want check error %q", err, c.text)
			}
			if st.State != "available" || st.Latest != "9.9.9" {
				t.Errorf("state after failure = %+v, want the previous available", st)
			}
			if got := f.ev.states(); got != "checking,available" {
				t.Errorf("emitted %s", got)
			}

			// From idle, a failure never yields available.
			g := newSvc(t)
			if c.status == 0 {
				g.gh.Close()
			} else {
				g.gh.set(nil, c.status)
			}
			if st, err := g.svc.Check(context.Background()); err == nil || st.State != "idle" {
				t.Errorf("from idle: %+v, %v", st, err)
			}
		})
	}
}

// WR-04: a newer release whose DMG is missing, still uploading or invalid is an incomplete check
// (kind check, previous status kept), not "current", so the poller retries it on the next tick.
func TestService_CheckNewerNotInstallable(t *testing.T) {
	const text = "Version 9.9.9 isn't ready to download yet. Try again later."
	for name, edit := range map[string]func(r map[string]any){
		"no DMG":    func(r map[string]any) { r["assets"] = []map[string]any{} },
		"uploading": func(r map[string]any) { r["assets"].([]map[string]any)[0]["state"] = "starter" },
		"no digest": func(r map[string]any) { delete(r["assets"].([]map[string]any)[0], "digest") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newSvc(t)
			r := f.gh.rel("9.9.9")
			edit(r)
			f.gh.set(r, 0)
			st, err := f.svc.Check(context.Background())
			var e *Error
			if !errors.As(err, &e) || e.Kind != "check" || e.Text != text {
				t.Errorf("err = %#v, want check error %q", err, text)
			}
			if st.State != "idle" || f.ev.states() != "checking,idle" {
				t.Errorf("Check = %+v, emitted %s; want the previous idle", st, f.ev.states())
			}
		})
	}
}

func TestService_CheckSerialized(t *testing.T) {
	f := newSvc(t)
	f.gh.set(f.gh.rel("9.9.9"), 0)
	entered, release := make(chan struct{}, 2), make(chan struct{})
	var first atomic.Bool
	f.gh.onLatest = func() {
		if first.CompareAndSwap(false, true) {
			entered <- struct{}{}
			<-release
		}
	}
	done := make(chan Status, 2)
	go func() { st, _ := f.svc.Check(context.Background()); done <- st }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first Check never reached the server")
	}
	go func() { st, _ := f.svc.Check(context.Background()); done <- st }()
	select {
	case st := <-done:
		t.Fatalf("a Check returned %+v while the first was still in flight", st)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for range 2 {
		select {
		case st := <-done:
			if st.State != "available" {
				t.Errorf("Check = %+v", st)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a Check never returned")
		}
	}
	got := f.gh.checks()
	if len(got) != 2 || got[0] != "" || got[1] == "" {
		t.Errorf("If-None-Match per request = %q; want two requests, the second conditional", got)
	}
}

// available checks the fake's v9.9.9 so the service is ready to Download, then clears the emits.
func (f svcFix) available(t *testing.T) {
	t.Helper()
	f.gh.set(f.gh.rel("9.9.9"), 0)
	if st, err := f.svc.Check(context.Background()); err != nil || st.State != "available" {
		t.Fatalf("Check = %+v, %v", st, err)
	}
	f.ev.reset()
}

const (
	checksumText  = "The download didn't match the release's checksum, so Portty deleted it."
	signatureText = "The download didn't pass the signature check, so Portty deleted it."
	revealText    = "Portty couldn't show the download in Finder."
	downloadText  = "Portty couldn't download version 9.9.9. Try again later."
)

func TestService_DownloadReveals(t *testing.T) {
	f := newSvc(t)
	f.available(t)
	stale := filepath.Join(f.dir, DMGAssetName("1.0.0"))
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Download()
	if err != nil {
		t.Fatal(err)
	}
	if want := (Status{State: "revealed", Current: "1.0.0", Latest: "9.9.9"}); st != want {
		t.Errorf("Download = %+v, want %+v", st, want)
	}
	path := filepath.Join(f.dir, DMGAssetName("9.9.9"))
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("downloaded DMG: %v %v", fi, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale DMG not cleaned: %v", err)
	}
	ev := f.ev.list()
	if len(ev) < 4 {
		t.Fatalf("emitted %s", f.ev.states())
	}
	if ev[0].State != "downloading" || ev[0].Total != int64(len(payload)) || ev[0].Latest != "9.9.9" {
		t.Errorf("first emit %+v", ev[0])
	}
	if p := ev[len(ev)-3]; p.State != "downloading" || p.Done != int64(len(payload)) || p.Total != int64(len(payload)) {
		t.Errorf("last progress %+v", p)
	}
	if v, r := ev[len(ev)-2], ev[len(ev)-1]; v.State != "verifying" || r != st {
		t.Errorf("tail %+v, %+v", v, r)
	}
	f.run.mu.Lock()
	calls := append([]string(nil), f.run.calls...)
	f.run.mu.Unlock()
	if len(calls) != 5 || !strings.HasPrefix(calls[0], "/usr/bin/codesign --verify") || calls[4] != "/usr/bin/open -R "+path {
		t.Errorf("runner calls %q", calls)
	}
	// IN-02: verify and reveal run on their own 2-minute deadline, not the download's 10 minutes.
	f.run.mu.Lock()
	left := append([]time.Duration(nil), f.run.left...)
	f.run.mu.Unlock()
	for i, d := range left {
		if d <= 0 || d > 2*time.Minute {
			t.Errorf("call %d (%s) ran with %v left, want its own deadline of at most 2m", i, calls[i], d)
		}
	}
}

// downloadFailure drives one failing Download from available.
func downloadFailure(t *testing.T, mode string) (svcFix, Status, error) {
	t.Helper()
	f := newSvc(t)
	f.available(t)
	f.gh.mu.Lock()
	switch mode {
	case "500":
		f.gh.assetStatus = 500
	case "digest":
		bad := append([]byte(nil), payload...)
		bad[0] ^= 1
		f.gh.assetBody = bad
	case "codesign":
		f.run.fail = "codesign"
	case "reveal":
		f.run.fail = "open"
	}
	f.gh.mu.Unlock()
	st, err := f.svc.Download()
	return f, st, err
}

func TestService_DownloadFailures(t *testing.T) {
	for _, c := range []struct {
		mode, kind, text string
		kept             bool
	}{
		{"500", "download", downloadText, false},
		{"digest", "verify", checksumText, false},
		{"codesign", "verify", signatureText, false},
		{"reveal", "reveal", revealText, true},
	} {
		t.Run(c.mode, func(t *testing.T) {
			f, st, err := downloadFailure(t, c.mode)
			var e *Error
			if !errors.As(err, &e) || e.Kind != c.kind || e.Text != c.text {
				t.Errorf("err = %#v, want %s %q", err, c.kind, c.text)
			}
			if want := (Status{State: "available", Current: "1.0.0", Latest: "9.9.9", Error: c.text}); st != want || f.svc.Status() != want {
				t.Errorf("Download = %+v, Status = %+v; want %+v", st, f.svc.Status(), want)
			}
			if ev := f.ev.list(); len(ev) == 0 || ev[len(ev)-1].Error != c.text {
				t.Errorf("last emit %+v", ev)
			}
			_, statErr := os.Stat(filepath.Join(f.dir, DMGAssetName("9.9.9")))
			if kept := statErr == nil; kept != c.kept {
				t.Errorf("DMG kept %v, want %v", kept, c.kept)
			}
			if c.kept {
				return
			}
			entries, _ := os.ReadDir(f.dir)
			for _, e := range entries {
				t.Errorf("left in Dir: %s", e.Name())
			}
		})
	}
	t.Run("retry clears the error", func(t *testing.T) {
		f, _, _ := downloadFailure(t, "reveal")
		f.run.mu.Lock()
		f.run.fail = ""
		f.run.mu.Unlock()
		if st, err := f.svc.Download(); err != nil || st.State != "revealed" || st.Error != "" {
			t.Errorf("retry = %+v, %v", st, err)
		}
	})
	t.Run("no Dir", func(t *testing.T) {
		f := newSvc(t)
		f.available(t)
		f.svc.Dir = ""
		var e *Error
		if st, err := f.svc.Download(); !errors.As(err, &e) || e.Kind != "download" || st.State != "available" {
			t.Errorf("Download with no Dir = %+v, %v", st, err)
		}
		if f.gh.assets.Load() != 0 {
			t.Error("an asset request was sent with no Dir")
		}
	})
}

func TestService_DownloadNotAvailable(t *testing.T) {
	f := newSvc(t)
	if st, err := f.svc.Download(); err != nil || st != (Status{State: "idle", Current: "1.0.0"}) {
		t.Errorf("from idle: %+v, %v", st, err)
	}
	f.gh.set(f.gh.rel("1.0.0"), 0)
	if _, err := f.svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.ev.reset()
	if st, err := f.svc.Download(); err != nil || st != (Status{State: "current", Current: "1.0.0"}) {
		t.Errorf("from current: %+v, %v", st, err)
	}
	if f.gh.assets.Load() != 0 || len(f.ev.list()) != 0 {
		t.Errorf("%d asset requests, emitted %s", f.gh.assets.Load(), f.ev.states())
	}
}

func TestService_DownloadOneAtATime(t *testing.T) {
	f := newSvc(t)
	f.available(t)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	f.gh.mu.Lock()
	f.gh.onAsset = func() { entered <- struct{}{}; <-release }
	f.gh.mu.Unlock()
	done := make(chan Status, 1)
	go func() { st, _ := f.svc.Download(); done <- st }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the first Download never reached the asset")
	}
	if st, err := f.svc.Download(); err != nil || st.State != "downloading" {
		t.Errorf("second Download = %+v, %v", st, err)
	}
	if st, err := f.svc.Check(context.Background()); err != nil || st.State != "downloading" {
		t.Errorf("Check during download = %+v, %v", st, err)
	}
	close(release)
	if st := <-done; st.State != "revealed" {
		t.Errorf("first Download = %+v", st)
	}
	if n := f.gh.assets.Load(); n != 1 {
		t.Errorf("%d asset requests, want 1", n)
	}
	if n := len(f.gh.checks()); n != 1 {
		t.Errorf("%d release requests, want 1 (the Check during the download sent none)", n)
	}
}

func TestService_ErrorTextsHaveNoURLOrPath(t *testing.T) {
	var texts []string
	for _, mode := range []string{"500", "digest", "codesign", "reveal"} {
		f, st, err := downloadFailure(t, mode)
		if err == nil {
			t.Fatalf("%s: no error", mode)
		}
		texts = append(texts, err.Error(), st.Error)
		for _, s := range []string{err.Error(), st.Error} {
			if strings.Contains(s, f.dir) || strings.Contains(s, f.gh.URL) {
				t.Errorf("%s: %q holds the Dir or the server URL", mode, s)
			}
		}
	}
	for _, status := range []int{500, 403} {
		f := newSvc(t)
		f.gh.set(nil, status)
		_, err := f.svc.Check(context.Background())
		texts = append(texts, err.Error())
	}
	for _, s := range texts {
		if s == "" || strings.Contains(s, "://") || strings.Contains(s, string(filepath.Separator)) {
			t.Errorf("error text %q is empty or holds a URL or a path", s)
		}
	}
}
