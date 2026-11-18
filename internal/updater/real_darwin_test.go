//go:build darwin

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Env-gated checks on real release files with the real tools; they skip in the normal suite.

// copyTemp copies src into a fresh temp dir, optionally flipping the middle byte.
func copyTemp(t *testing.T, src string, flip bool) string {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if flip {
		b[len(b)/2] ^= 0xFF
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

// signingIdentifier reads the Identifier= line of codesign -dvv for path.
func signingIdentifier(t *testing.T, path string) string {
	t.Helper()
	out, err := ExecRunner(context.Background(), "/usr/bin/codesign", "-dvv", path)
	if err != nil {
		t.Fatalf("codesign -dvv: %v\n%s", err, out)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if id, ok := strings.CutPrefix(l, "Identifier="); ok {
			return id
		}
	}
	t.Fatalf("no Identifier= line:\n%s", out)
	return ""
}

// UPDATE_REAL_DMG={path}: the real tools accept a notarized Team S2K7P43927 DMG under the asset
// name its signing identifier names, and reject a byte-flipped copy. Under a Portty asset name
// for another version the same DMG is refused at the identifier step (WR-01): GIFsmith v1.0.1's
// DMG (Identifier=Gifsmith-1.0.1-arm64) must not pass as a Portty release. A real Portty release
// DMG kept under its asset name must carry that name as its identifier.
// UPDATE_REAL_BAD_DMG={path[:path...]}: each is rejected; the failed step is logged.
// UPDATE_REAL_REVEAL=1 also reveals the verified copy in Finder (only the user sets this).
func TestRealDMG(t *testing.T) {
	src := os.Getenv("UPDATE_REAL_DMG")
	if src == "" {
		t.Skip("UPDATE_REAL_DMG not set")
	}
	ctx := context.Background()
	good := copyTemp(t, src, false)
	id := signingIdentifier(t, good)
	if err := Verify(ctx, ExecRunner, good, TeamID, id+".dmg", time.Now()); err != nil {
		t.Fatalf("Verify(real DMG as %s.dmg) = %v", id, err)
	}
	if base := filepath.Base(src); strings.HasPrefix(base, "portty-v") && base != id+".dmg" {
		t.Errorf("Portty release DMG %s is signed with identifier %q", base, id)
	}
	other := DMGAssetName("0.0.1")
	if other == id+".dmg" {
		other = DMGAssetName("0.0.2")
	}
	if err := Verify(ctx, ExecRunner, good, TeamID, other, time.Now()); !errors.Is(err, ErrVerify) || !strings.HasSuffix(err.Error(), ": identifier") {
		t.Errorf("Verify(real DMG as %s) = %v, want ErrVerify at step identifier", other, err)
	} else {
		t.Logf("as %s: %v", other, err)
	}
	if err := Verify(ctx, ExecRunner, copyTemp(t, src, true), TeamID, id+".dmg", time.Now()); !errors.Is(err, ErrVerify) {
		t.Errorf("Verify(byte-flipped DMG) = %v, want ErrVerify", err)
	} else {
		t.Logf("byte-flipped: %v", err)
	}
	if bad := os.Getenv("UPDATE_REAL_BAD_DMG"); bad != "" {
		for _, p := range strings.Split(bad, ":") {
			err := Verify(ctx, ExecRunner, copyTemp(t, p, false), TeamID, filepath.Base(p), time.Now())
			if !errors.Is(err, ErrVerify) {
				t.Errorf("Verify(%s) = %v, want ErrVerify", p, err)
				continue
			}
			t.Logf("%s: %v", filepath.Base(p), err)
		}
	}
	if os.Getenv("UPDATE_REAL_REVEAL") == "1" {
		if err := Reveal(ctx, ExecRunner, good); err != nil {
			t.Errorf("Reveal: %v", err)
		}
	}
}

// UPDATE_RELEASE_JSON={file} (the REST JSON of one release) and UPDATE_DMG_PATH={dmg}: served as the
// latest release with draft forced false, the checker offers exactly that DMG and its digest.
func TestReleaseJSON(t *testing.T) {
	file, dmg := os.Getenv("UPDATE_RELEASE_JSON"), os.Getenv("UPDATE_DMG_PATH")
	if file == "" || dmg == "" {
		t.Skip("UPDATE_RELEASE_JSON or UPDATE_DMG_PATH not set")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var rel map[string]any
	if err := json.Unmarshal(raw, &rel); err != nil {
		t.Fatal(err)
	}
	rel["draft"] = false // a draft release differs from the published one only in this flag
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()
	got, found, err := (&Checker{API: srv.URL, Repo: Repo, HTTP: srv.Client(), Allow: GitHubAllow}).Latest(context.Background())
	if err != nil || !found {
		t.Fatalf("Latest = %v, %v", found, err)
	}
	tag, _ := rel["tag_name"].(string)
	if got.Version != strings.TrimPrefix(tag, "v") || got.DMG.Name != DMGAssetName(got.Version) {
		t.Errorf("release %+v for tag %q", got, tag)
	}
	b, err := os.ReadFile(dmg)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if got.DMG.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("asset digest %s, local DMG %x", got.DMG.SHA256, sum)
	}
}
