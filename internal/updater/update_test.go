package updater

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1}, {"1.0.1", "1.1.0", -1}, {"1.1.0", "2.0.0", -1},
		{"1.10.0", "1.9.0", 1}, {"v1.2.3", "1.2.3", 0}, {"2.0.0", "1.99.99", 1},
	} {
		got, err := Compare(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("Compare(%q, %q) = %d, %v; want %d", c.a, c.b, got, err, c.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	if v, err := ParseVersion("v1.10.3"); err != nil || v != [3]int{1, 10, 3} {
		t.Errorf("ParseVersion(v1.10.3) = %v, %v", v, err)
	}
	for _, s := range []string{"", "v", "1.2", "1.2.3.4", "1.2.3-beta.1", "1.2.x", " 1.2.3", "01.2.3"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) accepted", s)
		}
	}
}

func TestDMGAssetName(t *testing.T) {
	if got := DMGAssetName("1.1.0"); got != "portty-v1.1.0-darwin-universal.dmg" {
		t.Errorf("DMGAssetName = %q", got)
	}
}

const digest = "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func asset(name string) map[string]any {
	return map[string]any{
		"name": name, "size": 1234, "state": "uploaded", "digest": digest,
		"browser_download_url": "https://github.com/scottkw/portty/releases/download/v1.1.0/" + name,
	}
}

func release(tag string, assets ...map[string]any) map[string]any {
	if assets == nil {
		assets = []map[string]any{asset(DMGAssetName(strings.TrimPrefix(tag, "v")))}
	}
	return map[string]any{"tag_name": tag, "draft": false, "prerelease": false, "assets": assets}
}

// serve answers the latest-release route with body (ETag "e1", 304 on a match) and records requests.
func serve(t *testing.T, status int, body any) (*Checker, func() []*http.Request) {
	t.Helper()
	var mu sync.Mutex
	var reqs []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r)
		mu.Unlock()
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"e1"`)
		w.WriteHeader(status)
		if s, ok := body.(string); ok {
			_, _ = w.Write([]byte(s))
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return &Checker{API: srv.URL, Repo: Repo, HTTP: srv.Client(), Allow: GitHubAllow}, func() []*http.Request {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(reqs)
	}
}

func TestLatestFound(t *testing.T) {
	c, reqs := serve(t, 200, release("v1.1.0"))
	rel, found, err := c.Latest(context.Background())
	if err != nil || !found {
		t.Fatalf("Latest = %v, %v", found, err)
	}
	want := Release{Version: "1.1.0", DMG: Asset{Name: "portty-v1.1.0-darwin-universal.dmg", Size: 1234, SHA256: digest[len("sha256:"):],
		URL: "https://github.com/scottkw/portty/releases/download/v1.1.0/portty-v1.1.0-darwin-universal.dmg"}}
	if rel != want {
		t.Errorf("rel = %+v\nwant %+v", rel, want)
	}
	r := reqs()[0]
	if r.URL.Path != "/repos/scottkw/portty/releases/latest" || r.URL.RawQuery != "" || r.Method != http.MethodGet {
		t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
	}
	if r.Header.Get("User-Agent") != "Portty" || r.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers %v", r.Header)
	}
	for k := range r.Header {
		switch k {
		case "Accept", "User-Agent", "If-None-Match", "Accept-Encoding":
		default:
			t.Errorf("unexpected header %s: %v", k, r.Header[k])
		}
	}
	// The second check sends the ETag and reuses the cached release on 304.
	rel2, found, err := c.Latest(context.Background())
	if err != nil || !found || rel2 != want {
		t.Fatalf("after 304: %+v, %v, %v", rel2, found, err)
	}
	if got := reqs()[1].Header.Get("If-None-Match"); got != `"e1"` {
		t.Errorf("If-None-Match = %q", got)
	}
}

func TestLatestNotFoundAndErrors(t *testing.T) {
	c, _ := serve(t, 404, `{"message":"Not Found"}`)
	if _, found, err := c.Latest(context.Background()); found || err != nil {
		t.Errorf("404: found %v err %v; want false, nil", found, err)
	}
	c, _ = serve(t, 503, "")
	if _, _, err := c.Latest(context.Background()); err == nil {
		t.Error("503: want an error")
	}
	c, _ = serve(t, 200, `{"tag_name":"v1.1.0","body":"`+strings.Repeat("x", 1<<20)+`"}`)
	if _, _, err := c.Latest(context.Background()); err == nil {
		t.Error("body over 1 MiB: want an error")
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c = &Checker{API: srv.URL, Repo: Repo, HTTP: http.DefaultClient, Allow: GitHubAllow}
	if _, _, err := c.Latest(context.Background()); err == nil {
		t.Error("closed listener: want an error")
	}
}

func TestLatestNotOffered(t *testing.T) {
	draft := release("v1.1.0")
	draft["draft"] = true
	pre := release("v1.1.0")
	pre["prerelease"] = true
	for name, body := range map[string]any{"draft": draft, "prerelease": pre, "v1.1": release("v1.1"), "latest": release("latest")} {
		c, _ := serve(t, 200, body)
		if _, found, err := c.Latest(context.Background()); found || err != nil {
			t.Errorf("%s: found %v err %v; want false, nil", name, found, err)
		}
	}
}

func TestLatestNotInstallable(t *testing.T) {
	name := DMGAssetName("1.1.0")
	mod := func(k string, v any) map[string]any {
		a := asset(name)
		a[k] = v
		return a
	}
	cases := map[string][]map[string]any{
		"missing":     {asset("portty-v1.1.0-darwin-arm64.dmg")},
		"traversal":   {asset("../" + name)},
		"zip":         {asset(name + ".zip")},
		"starter":     {mod("state", "starter")},
		"size 0":      {mod("size", 0)},
		"too large":   {mod("size", MaxDMGBytes+1)},
		"no digest":   {mod("digest", "")},
		"sha1":        {mod("digest", "sha1:0123456789abcdef0123456789abcdef01234567")},
		"bad hex":     {mod("digest", "sha256:XYZ")},
		"other host":  {mod("browser_download_url", "https://evil.example/"+name)},
		"plain http":  {mod("browser_download_url", "http://github.com/scottkw/portty/releases/download/v1.1.0/"+name)},
		"other repo":  {mod("browser_download_url", "https://github.com/other/repo/releases/download/v1.1.0/"+name)},
		"other owner": {mod("browser_download_url", "https://github.com/someone/portty/releases/download/v1.1.0/"+name)},
		"upper hex":   {mod("digest", strings.ToUpper(digest))},
		"empty asset": {},
	}
	for n, assets := range cases {
		c, _ := serve(t, 200, map[string]any{"tag_name": "v1.1.0", "assets": assets})
		rel, found, err := c.Latest(context.Background())
		if !errors.Is(err, ErrNotInstallable) || found || rel.Version != "1.1.0" {
			t.Errorf("%s: %+v, %v, err = %v; want version 1.1.0 with ErrNotInstallable", n, rel, found, err)
		}
	}
}

func TestGitHubAllow(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://github.com/scottkw/portty/releases/download/v1.1.0/portty-v1.1.0-darwin-universal.dmg": true,
		"https://objects.githubusercontent.com/x":                                                       true,
		"https://release-assets.githubusercontent.com/x":                                                true,
		"https://github.com/other/repo/releases/download/v1.1.0/x.dmg":                                  false,
		"https://github.com/someone/portty/releases/download/v1.1.0/x.dmg":                              false,
		"http://github.com/scottkw/portty/releases/download/v1.1.0/x.dmg":                               false,
		"https://github.com.evil.example/scottkw/portty/releases/download/v1.1.0/x.dmg":                 false,
		"https://evil.githubusercontent.com/x":                                                          false,
		"https://github.com/scottkw/portty/releases/download/../../other/repo/x.dmg":                    false,
		"https://user@github.com/scottkw/portty/releases/download/v1.1.0/x.dmg":                         false,
		"http://objects.githubusercontent.com/x":                                                        false,
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := GitHubAllow(u); got != want {
			t.Errorf("GitHubAllow(%s) = %v, want %v", raw, got, want)
		}
	}
}

// One request at a time: Latest serializes on its cache, so concurrent calls never race.
func TestLatestConcurrent(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("ETag", `"e1"`)
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_ = json.NewEncoder(w).Encode(release("v1.1.0"))
	}))
	defer srv.Close()
	c := &Checker{API: srv.URL, Repo: Repo, HTTP: srv.Client(), Allow: GitHubAllow}
	done := make(chan error)
	for range 4 {
		go func() { _, _, err := c.Latest(context.Background()); done <- err }()
	}
	for range 4 {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
}

func TestLatestETag(t *testing.T) {
	// No ETag on the first request; a 304 with nothing cached is an error, not a stale answer.
	var inm []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inm = append(inm, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	c := &Checker{API: srv.URL, Repo: Repo, HTTP: srv.Client(), Allow: GitHubAllow}
	if _, found, err := c.Latest(context.Background()); err == nil || found {
		t.Errorf("304 without a cache: found %v err %v; want an error", found, err)
	}
	if !slices.Equal(inm, []string{""}) {
		t.Errorf("If-None-Match sent %q; want none on the first call", inm)
	}
}

func TestLatestRateLimited(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		c, _ := serve(t, status, `{"message":"API rate limit exceeded"}`)
		if _, found, err := c.Latest(context.Background()); found || !errors.Is(err, ErrRateLimited) {
			t.Errorf("%d: found %v err %v; want ErrRateLimited", status, found, err)
		}
	}
	c, _ := serve(t, http.StatusInternalServerError, "")
	if _, _, err := c.Latest(context.Background()); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("500: err %v; want a non-rate-limit error", err)
	}
}

func TestNewCheckClient(t *testing.T) {
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer other.Close()
	var api *httptest.Server
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, api.URL+"/ok", http.StatusFound)
		case "/off":
			http.Redirect(w, r, other.URL+"/ok", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, api.URL+"/loop", http.StatusFound)
		case "/ok":
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer api.Close()
	c := NewCheckClient(api.URL)
	if c.Timeout != 30*time.Second {
		t.Errorf("timeout %v", c.Timeout)
	}
	get := func(path string) error {
		resp, err := c.Get(api.URL + path)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}
	if err := get("/same"); err != nil {
		t.Errorf("same-host redirect: %v", err)
	}
	if err := get("/off"); err == nil {
		t.Error("redirect to another host: want an error")
	}
	if err := get("/loop"); err == nil {
		t.Error("redirect loop: want an error")
	}
	if hits.Load() != 0 {
		t.Errorf("the other host saw %d requests", hits.Load())
	}
	// https API: a redirect to http on the same host is refused before any request is sent.
	sec := NewCheckClient("https://api.example")
	req, _ := http.NewRequest(http.MethodGet, "http://api.example/x", nil)
	if err := sec.CheckRedirect(req, []*http.Request{{}}); err == nil {
		t.Error("https to http redirect: want an error")
	}
}
