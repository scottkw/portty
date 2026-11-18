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
	run := &fakeRun{team: "TeamIdentifier=" + TeamID, spctl: goodSpctl}
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
		"404":        func(*ghFake) any { return nil },
		"draft":      func(g *ghFake) any { r := g.rel("9.9.9"); r["draft"] = true; return r },
		"prerelease": func(g *ghFake) any { r := g.rel("9.9.9"); r["prerelease"] = true; return r },
		"tag latest": func(g *ghFake) any { r := g.rel("9.9.9"); r["tag_name"] = "latest"; return r },
		"no digest": func(g *ghFake) any {
			r := g.rel("9.9.9")
			delete(r["assets"].([]map[string]any)[0], "digest")
			return r
		},
		"same version": func(g *ghFake) any { return g.rel("1.0.0") },
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
