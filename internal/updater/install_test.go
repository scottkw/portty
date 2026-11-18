package updater

import (
	"bytes"
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
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var payload = bytes.Repeat([]byte("portty!!"), 64<<10) // 512 KiB

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// assetServer serves body on every path and counts requests.
func assetServer(t *testing.T, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func anyAllow(*url.URL) bool { return true }

func noFiles(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("files left: %v", names)
	}
}

// fakeRun answers like codesign/spctl/xattr/open on a good notarized DMG; fail names a step to break.
type fakeRun struct {
	mu    sync.Mutex
	calls []string
	fail  string          // "codesign", "dvv", "spctl", "xattr", "open"
	team  string          // -dvv output lines (goodDVV: the identifier of DMGAssetName("9.9.9") and Team S2K7P43927)
	spctl string          // spctl output
	left  []time.Duration // time left on each call's context deadline (0: none)
}

func (f *fakeRun) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	var left time.Duration
	if d, ok := ctx.Deadline(); ok {
		left = time.Until(d)
	}
	f.left = append(f.left, left)
	step := filepath.Base(name)
	if slices.Contains(args, "-dvv") {
		step = "dvv"
	}
	if step == f.fail {
		return []byte("failed"), errors.New("exit status 1")
	}
	switch step {
	case "dvv":
		return []byte("Executable=/x\n" + f.team + "\nTimestamp=now\n"), nil
	case "spctl":
		return []byte(f.spctl), nil
	}
	return nil, nil
}

const goodDVV = "Identifier=portty-v9.9.9-darwin-universal\nTeamIdentifier=" + TeamID

const goodSpctl = "/tmp/x.dmg: accepted\nsource=Notarized Developer ID\norigin=Developer ID Application: Ken Scott (S2K7P43927)\n"

// TestFlow_CheckDownloadVerifyReveal drives the whole client path: a fake GitHub release becomes
// a downloaded, digest-checked, verified, quarantined and revealed DMG.
func TestFlow_CheckDownloadVerifyReveal(t *testing.T) {
	name := DMGAssetName("9.9.9")
	if name != "portty-v9.9.9-darwin-universal.dmg" {
		t.Fatalf("DMGAssetName = %q", name)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/scottkw/portty/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v9.9.9", "draft": false, "prerelease": false,
				"assets": []map[string]any{{
					"name": name, "size": len(payload), "state": "uploaded", "digest": "sha256:" + sum(payload),
					"browser_download_url": srv.URL + "/dl/" + name,
				}},
			})
		case "/dl/" + name:
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	allow := func(u *url.URL) bool { return u.Host == host }

	ctx := context.Background()
	c := &Checker{API: srv.URL, Repo: Repo, HTTP: NewCheckClient(srv.URL), Allow: allow}
	rel, found, err := c.Latest(ctx)
	if err != nil || !found || rel.Version != "9.9.9" || rel.DMG.Name != name {
		t.Fatalf("Latest = %+v, %v, %v", rel, found, err)
	}
	dir := filepath.Join(t.TempDir(), "updates")
	path, err := Download(ctx, NewDownloadClient(allow), rel.DMG, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("downloaded file: %v %v", fi, err)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Error(".part left behind")
	}

	now := time.Unix(0x6720_0000, 0)
	f := &fakeRun{team: goodDVV, spctl: path + ": accepted\nsource=Notarized Developer ID\n"}
	if err := Verify(ctx, f.run, path, TeamID, name, now); err != nil {
		t.Fatal(err)
	}
	if err := Reveal(ctx, f.run, path); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/usr/bin/codesign --verify --strict " + path,
		"/usr/bin/codesign -dvv " + path,
		"/usr/sbin/spctl -a -t open --context context:primary-signature -vv " + path,
		fmt.Sprintf("/usr/bin/xattr -w com.apple.quarantine 0081;%x;Portty; %s", now.Unix(), path),
		"/usr/bin/open -R " + path,
	}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls\n%q\nwant\n%q", f.calls, want)
	}
}

func TestReveal(t *testing.T) {
	path := filepath.Join(t.TempDir(), DMGAssetName("1.0.0"))
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeRun{fail: "open"}
	if err := Reveal(context.Background(), f.run, path); !errors.Is(err, ErrReveal) {
		t.Errorf("err = %v, want ErrReveal", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a failed reveal removed the file: %v", err)
	}
	if want := []string{"/usr/bin/open -R " + path}; !slices.Equal(f.calls, want) {
		t.Errorf("calls %q, want %q", f.calls, want)
	}
}

const testDMG = "portty-v1.1.0-darwin-universal.dmg"

func TestDownload(t *testing.T) {
	srv, _ := assetServer(t, payload)
	dir := filepath.Join(t.TempDir(), "updates")
	var mu sync.Mutex
	var calls [][2]int64
	a := Asset{Name: testDMG, URL: srv.URL + "/x.dmg", Size: int64(len(payload)), SHA256: sum(payload)}
	path, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, func(done, total int64) {
		mu.Lock()
		calls = append(calls, [2]int64{done, total})
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, a.Name) {
		t.Errorf("path = %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("contents differ (%v)", err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v", fi.Mode().Perm())
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Error(".part left behind")
	}
	if len(calls) == 0 || calls[len(calls)-1] != [2]int64{a.Size, a.Size} {
		t.Fatalf("progress %v", calls)
	}
	if !slices.IsSortedFunc(calls, func(x, y [2]int64) int { return int(x[0] - y[0]) }) {
		t.Errorf("progress decreased: %v", calls)
	}
}

func TestDownloadDigestMismatch(t *testing.T) {
	srv, _ := assetServer(t, payload)
	dir := t.TempDir()
	a := Asset{Name: testDMG, URL: srv.URL, Size: int64(len(payload)), SHA256: sum(payload[1:])}
	if _, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, nil); !errors.Is(err, ErrDigest) {
		t.Fatalf("err = %v, want ErrDigest", err)
	}
	noFiles(t, dir)
}

func TestDownloadSize(t *testing.T) {
	srv, n := assetServer(t, payload)
	dir := t.TempDir()
	a := Asset{Name: testDMG, URL: srv.URL, Size: MaxDMGBytes + 1, SHA256: sum(payload)}
	if _, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, nil); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
	if n.Load() != 0 {
		t.Error("a request was sent for an oversized asset")
	}
	for _, size := range []int64{int64(len(payload)) - 1, int64(len(payload)) + 1} {
		a := Asset{Name: testDMG, URL: srv.URL, Size: size, SHA256: sum(payload)}
		if _, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, nil); err == nil {
			t.Errorf("size %d of %d: want an error", size, len(payload))
		}
		noFiles(t, dir)
	}
}

func TestDownloadBadName(t *testing.T) {
	srv, n := assetServer(t, payload)
	dir := t.TempDir()
	a := Asset{Name: "../" + testDMG, URL: srv.URL, Size: int64(len(payload)), SHA256: sum(payload)}
	if _, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, nil); err == nil || n.Load() != 0 {
		t.Errorf("err = %v, requests %d", err, n.Load())
	}
}

func TestDownloadShortBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload[:1000])
	}))
	defer srv.Close()
	dir := t.TempDir()
	a := Asset{Name: testDMG, URL: srv.URL, Size: int64(len(payload)), SHA256: sum(payload)}
	if _, err := Download(context.Background(), NewDownloadClient(anyAllow), a, dir, nil); err == nil || errors.Is(err, ErrDigest) {
		t.Errorf("err = %v, want a transfer error", err)
	}
	noFiles(t, dir)
}

func TestDownloadRedirects(t *testing.T) {
	other, n := assetServer(t, payload)
	redirect := func(to string) *httptest.Server {
		srv := httptest.NewServer(http.RedirectHandler(to, http.StatusFound))
		t.Cleanup(srv.Close)
		return srv
	}
	a := Asset{Name: testDMG, Size: int64(len(payload)), SHA256: sum(payload)}
	dir := t.TempDir()
	// The first hop is allowed; the redirect target's host is not.
	first := redirect(other.URL + "/x.dmg")
	firstHost := strings.TrimPrefix(first.URL, "http://")
	a.URL = first.URL
	allowFirst := func(u *url.URL) bool { return u.Host == firstHost }
	if _, err := Download(context.Background(), NewDownloadClient(allowFirst), a, dir, nil); err == nil {
		t.Error("redirect to a refused host: want an error")
	}
	// GitHubAllow refuses an https→http redirect.
	a.URL = redirect("http://objects.githubusercontent.com/x").URL
	if _, err := Download(context.Background(), NewDownloadClient(GitHubAllow), a, dir, nil); err == nil {
		t.Error("redirect to http: want an error")
	}
	if n.Load() != 0 {
		t.Errorf("the refused host saw %d requests", n.Load())
	}
	noFiles(t, dir)
}

func TestVerify(t *testing.T) {
	now := time.Unix(0x6720_0000, 0)
	f := &fakeRun{team: goodDVV, spctl: goodSpctl}
	if err := Verify(context.Background(), f.run, "/c/x.dmg", TeamID, DMGAssetName("9.9.9"), now); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/usr/bin/codesign --verify --strict /c/x.dmg",
		"/usr/bin/codesign -dvv /c/x.dmg",
		"/usr/sbin/spctl -a -t open --context context:primary-signature -vv /c/x.dmg",
		fmt.Sprintf("/usr/bin/xattr -w com.apple.quarantine 0081;%x;Portty; /c/x.dmg", now.Unix()),
	}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls\n%q\nwant\n%q", f.calls, want)
	}
}

func TestVerifyFailures(t *testing.T) {
	good := fakeRun{team: goodDVV, spctl: goodSpctl}
	// The identifier cases (WR-01) stand for a genuine older Portty DMG, or another product of the
	// same team, served under DMGAssetName("9.9.9").
	cases := map[string]struct {
		f    *fakeRun
		step string
	}{
		"codesign fails":    {&fakeRun{fail: "codesign", team: good.team, spctl: good.spctl}, "codesign"},
		"no team line":      {&fakeRun{team: "Authority=Developer ID Application", spctl: good.spctl}, "team"},
		"other team":        {&fakeRun{team: "TeamIdentifier=ABCDE12345", spctl: good.spctl}, "team"},
		"team not set":      {&fakeRun{team: "TeamIdentifier=not set", spctl: good.spctl}, "team"},
		"team prefix":       {&fakeRun{team: "TeamIdentifier=S2K7P439270", spctl: good.spctl}, "team"},
		"no identifier":     {&fakeRun{team: "TeamIdentifier=" + TeamID, spctl: good.spctl}, "identifier"},
		"older version":     {&fakeRun{team: "Identifier=portty-v1.0.0-darwin-universal\nTeamIdentifier=" + TeamID, spctl: good.spctl}, "identifier"},
		"other product":     {&fakeRun{team: "Identifier=Gifsmith-1.0.1-arm64\nTeamIdentifier=" + TeamID, spctl: good.spctl}, "identifier"},
		"identifier prefix": {&fakeRun{team: "Identifier=portty-v9.9.9-darwin-universalx\nTeamIdentifier=" + TeamID, spctl: good.spctl}, "identifier"},
		"spctl fails":       {&fakeRun{fail: "spctl", team: good.team, spctl: good.spctl}, "spctl"},
		"spctl rejected":    {&fakeRun{team: good.team, spctl: "/tmp/x.dmg: rejected\nsource=Notarized Developer ID\n"}, "spctl"},
		"unnotarized":       {&fakeRun{team: good.team, spctl: "/tmp/x.dmg: rejected\nsource=Unnotarized Developer ID\n"}, "spctl"},
		"not notarized":     {&fakeRun{team: good.team, spctl: "/tmp/x.dmg: accepted\nsource=Developer ID\n"}, "spctl"},
		"notarized prefix":  {&fakeRun{team: good.team, spctl: "/tmp/x.dmg: accepted\nsource=Notarized Developer IDx\n"}, "spctl"},
		"quarantine fails":  {&fakeRun{fail: "xattr", team: good.team, spctl: good.spctl}, "quarantine"},
		"dvv command fails": {&fakeRun{fail: "dvv", team: good.team, spctl: good.spctl}, "team"},
	}
	for name, c := range cases {
		err := Verify(context.Background(), c.f.run, "/c/x.dmg", TeamID, DMGAssetName("9.9.9"), time.Now())
		if !errors.Is(err, ErrVerify) || !strings.HasSuffix(err.Error(), ": "+c.step) {
			t.Errorf("%s: err = %v, want ErrVerify at step %s", name, err, c.step)
		}
	}
}

func TestClean(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{testDMG, "portty-v1.2.0-darwin-universal.dmg.part", "keep.txt", "portty-v1.1.0-darwin-arm64.dmg"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "portty-v9-darwin-universal.dmg"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Clean(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if want := []string{"keep.txt", "portty-v1.1.0-darwin-arm64.dmg", "portty-v9-darwin-universal.dmg"}; !slices.Equal(left, want) {
		t.Errorf("left %v, want %v", left, want)
	}
	if err := Clean(filepath.Join(dir, "missing")); err != nil {
		t.Errorf("missing dir: %v", err)
	}
}
