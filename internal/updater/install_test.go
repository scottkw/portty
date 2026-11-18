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
	fail  string // "codesign", "dvv", "spctl", "xattr", "open"
	team  string // -dvv output
	spctl string // spctl output
}

func (f *fakeRun) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
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
	f := &fakeRun{team: "TeamIdentifier=" + TeamID, spctl: path + ": accepted\nsource=Notarized Developer ID\n"}
	if err := Verify(ctx, f.run, path, TeamID, now); err != nil {
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
