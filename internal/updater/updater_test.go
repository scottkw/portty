package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// fakeDetect returns a DetectFunc that returns the configured version/found/error
// and tracks the number of times it is called.
func fakeDetect(version string, found bool, err error) (DetectFunc, *atomic.Int32) {
	var calls atomic.Int32
	fn := func(ctx context.Context, slug string) (string, bool, error) {
		calls.Add(1)
		return version, found, err
	}
	return fn, &calls
}

// writeTimestamp writes a last_check JSON file to configDir with the given time.
func writeTimestamp(t *testing.T, configDir string, ts time.Time) {
	t.Helper()
	data, err := json.Marshal(lastCheckFile{LastCheck: ts.UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("writeTimestamp: marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "update_check.json"), data, 0600); err != nil {
		t.Fatalf("writeTimestamp: write: %v", err)
	}
}

const testSlug = "scottkw/portty"

// TestCheck_DevVersionSkip ensures Check returns nil immediately when version is "dev"
// and does NOT call detectFunc.
func TestCheck_DevVersionSkip(t *testing.T) {
	dir := t.TempDir()
	det, calls := fakeDetect("2.0.0", true, nil)

	info, err := Check(context.Background(), dir, testSlug, "dev", det, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil UpdateInfo, got %+v", info)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected detectFunc not called, got %d calls", calls.Load())
	}
}

// TestCheck_NewerVersionFound verifies Check returns UpdateInfo with correct fields
// when the latest version is greater than current.
func TestCheck_NewerVersionFound(t *testing.T) {
	dir := t.TempDir()
	det, calls := fakeDetect("2.0.0", true, nil)

	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil UpdateInfo")
	}
	if info.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, "1.0.0")
	}
	if info.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, "2.0.0")
	}
	wantURL := fmt.Sprintf("https://github.com/%s/releases/tag/v%s", testSlug, "2.0.0")
	if info.ReleaseURL != wantURL {
		t.Errorf("ReleaseURL = %q, want %q", info.ReleaseURL, wantURL)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 detectFunc call, got %d", calls.Load())
	}
}

// TestCheck_AlreadyLatest ensures Check returns nil when current version equals latest.
func TestCheck_AlreadyLatest(t *testing.T) {
	dir := t.TempDir()
	det, _ := fakeDetect("2.0.0", true, nil)

	info, err := Check(context.Background(), dir, testSlug, "2.0.0", det, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil UpdateInfo for already-latest, got %+v", info)
	}
}

// TestCheck_DetectError ensures Check returns nil (error swallowed) when detectFunc errors.
func TestCheck_DetectError(t *testing.T) {
	dir := t.TempDir()
	det, _ := fakeDetect("", false, fmt.Errorf("network error"))

	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("expected nil error (swallowed), got %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil UpdateInfo on detect error, got %+v", info)
	}
}

// TestCheck_NotFound ensures Check returns nil when detectFunc reports not found.
func TestCheck_NotFound(t *testing.T) {
	dir := t.TempDir()
	det, _ := fakeDetect("", false, nil)

	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil UpdateInfo when not found, got %+v", info)
	}
}

// TestCheck_RateLimit verifies that a second Check within 1 hour does not call detectFunc.
func TestCheck_RateLimit(t *testing.T) {
	dir := t.TempDir()
	det, calls := fakeDetect("2.0.0", true, nil)

	// First check — should call detectFunc.
	_, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("first check error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call after first check, got %d", calls.Load())
	}

	// Second check within 1 hour — should be rate-limited, detectFunc NOT called.
	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("second check error: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil UpdateInfo when rate-limited, got %+v", info)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected detectFunc not called on second check, got %d total calls", calls.Load())
	}
}

// TestCheck_RateLimitExpired verifies that Check calls detectFunc when last_check is >1 hour ago.
func TestCheck_RateLimitExpired(t *testing.T) {
	dir := t.TempDir()
	det, calls := fakeDetect("2.0.0", true, nil)

	// Write a timestamp 2 hours in the past.
	writeTimestamp(t, dir, time.Now().Add(-2*time.Hour))

	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected UpdateInfo when rate limit expired")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 detectFunc call after expired rate limit, got %d", calls.Load())
	}
}

// TestCheck_RateLimitBypass verifies that force=true always calls detectFunc.
func TestCheck_RateLimitBypass(t *testing.T) {
	dir := t.TempDir()
	det, calls := fakeDetect("2.0.0", true, nil)

	// Write a fresh timestamp (within the 1-hour window).
	writeTimestamp(t, dir, time.Now())

	// With force=true, should bypass rate limit and call detectFunc.
	info, err := Check(context.Background(), dir, testSlug, "1.0.0", det, true)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected UpdateInfo when force=true bypasses rate limit")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 detectFunc call with force=true, got %d", calls.Load())
	}
}

// fakeSource is an offline selfupdate.Source returning canned releases or a
// canned error. DownloadReleaseAsset is never called by detection.
type fakeSource struct {
	releases []selfupdate.SourceRelease
	err      error
}

func (f fakeSource) ListReleases(context.Context, selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return f.releases, f.err
}

func (f fakeSource) DownloadReleaseAsset(context.Context, *selfupdate.Release, int64) (io.ReadCloser, error) {
	return nil, errors.New("fakeSource: download not supported")
}

type fakeRelease struct {
	tag        string
	draft      bool
	prerelease bool
	assets     []string
}

func (r fakeRelease) GetID() int64              { return 1 }
func (r fakeRelease) GetTagName() string        { return r.tag }
func (r fakeRelease) GetDraft() bool            { return r.draft }
func (r fakeRelease) GetPrerelease() bool       { return r.prerelease }
func (r fakeRelease) GetPublishedAt() time.Time { return time.Time{} }
func (r fakeRelease) GetReleaseNotes() string   { return "" }
func (r fakeRelease) GetName() string           { return r.tag }
func (r fakeRelease) GetURL() string {
	return "https://github.com/" + testSlug + "/releases/tag/" + r.tag
}
func (r fakeRelease) GetAssets() []selfupdate.SourceAsset {
	out := make([]selfupdate.SourceAsset, 0, len(r.assets))
	for i, name := range r.assets {
		out = append(out, fakeAsset{id: int64(i + 1), name: name, tag: r.tag})
	}
	return out
}

type fakeAsset struct {
	id   int64
	name string
	tag  string
}

func (a fakeAsset) GetID() int64    { return a.id }
func (a fakeAsset) GetName() string { return a.name }
func (a fakeAsset) GetSize() int    { return 1 }
func (a fakeAsset) GetBrowserDownloadURL() string {
	return "https://github.com/" + testSlug + "/releases/download/" + a.tag + "/" + a.name
}

func detectFrom(t *testing.T, src selfupdate.Source) (string, bool, error) {
	t.Helper()
	return detectLatestFrom(context.Background(), src, testSlug)
}

// TestDetectLatestFrom_FindsDMGRelease: the release DMG name matches the filter.
func TestDetectLatestFrom_FindsDMGRelease(t *testing.T) {
	src := fakeSource{releases: []selfupdate.SourceRelease{
		fakeRelease{tag: "v1.3.0", assets: []string{"checksums.txt", "portty-v1.3.0-darwin-universal.dmg"}},
	}}
	v, found, err := detectFrom(t, src)
	if err != nil || !found || v != "1.3.0" {
		t.Fatalf("got (%q, %v, %v), want (\"1.3.0\", true, nil)", v, found, err)
	}
}

// TestDetectLatestFrom_SkipsDraftAndPrerelease: newer drafts and prereleases are ignored.
func TestDetectLatestFrom_SkipsDraftAndPrerelease(t *testing.T) {
	src := fakeSource{releases: []selfupdate.SourceRelease{
		fakeRelease{tag: "v1.4.0", draft: true, assets: []string{"portty-v1.4.0-darwin-universal.dmg"}},
		fakeRelease{tag: "v1.3.0", prerelease: true, assets: []string{"portty-v1.3.0-darwin-universal.dmg"}},
		fakeRelease{tag: "v1.2.0", assets: []string{"portty-v1.2.0-darwin-universal.dmg"}},
	}}
	v, found, err := detectFrom(t, src)
	if err != nil || !found || v != "1.2.0" {
		t.Fatalf("got (%q, %v, %v), want (\"1.2.0\", true, nil)", v, found, err)
	}
}

// TestDetectLatestFrom_NoMatchingAsset: a release without the DMG is not an update.
func TestDetectLatestFrom_NoMatchingAsset(t *testing.T) {
	src := fakeSource{releases: []selfupdate.SourceRelease{
		fakeRelease{tag: "v1.3.0", assets: []string{"checksums.txt"}},
	}}
	_, found, err := detectFrom(t, src)
	if err != nil || found {
		t.Fatalf("got (found=%v, err=%v), want (false, nil)", found, err)
	}
}

// TestDetectLatestFrom_NoReleases: the GitHub source maps a 404 (no releases
// yet, or a private repo) to (nil, nil); that must read as nothing found.
func TestDetectLatestFrom_NoReleases(t *testing.T) {
	_, found, err := detectFrom(t, fakeSource{})
	if err != nil || found {
		t.Fatalf("got (found=%v, err=%v), want (false, nil)", found, err)
	}
}

// TestDetectLatestFrom_SourceError: a network or rate-limit error is reported
// by detection and swallowed by Check, so no UI appears.
func TestDetectLatestFrom_SourceError(t *testing.T) {
	src := fakeSource{err: errors.New("rate limited")}
	_, found, err := detectFrom(t, src)
	if found || err == nil {
		t.Fatalf("got (found=%v, err=%v), want (false, non-nil)", found, err)
	}

	detect := func(ctx context.Context, slug string) (string, bool, error) {
		return detectLatestFrom(ctx, src, slug)
	}
	info, err := Check(context.Background(), t.TempDir(), testSlug, "1.0.0", detect, true)
	if info != nil || err != nil {
		t.Fatalf("Check got (%+v, %v), want (nil, nil)", info, err)
	}
}
