package updater

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These tests keep .github/workflows/release.yml in step with this package: the DMG it publishes
// must be the one DMGAssetName names, and it must pass Verify before it reaches a release.

func readWorkflow(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("ReadFile release.yml: %v", err)
	}
	return string(b)
}

var jobKeyRE = regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:\s*$`)

// job returns the text of one top-level job, from its key to the next job's key.
func job(t *testing.T, wf, name string) string {
	t.Helper()
	start := strings.Index(wf, "\n  "+name+":\n")
	if start < 0 {
		t.Fatalf("job %s not found", name)
	}
	rest := wf[start+1:]
	if loc := jobKeyRE.FindAllStringIndex(rest, 2); len(loc) == 2 {
		return rest[:loc[1][0]]
	}
	return rest
}

// step returns the text of the step named name, up to the next step.
func step(t *testing.T, j, name string) string {
	t.Helper()
	i := strings.Index(j, "- name: "+name+"\n")
	if i < 0 {
		t.Fatalf("step %q not found", name)
	}
	s := j[i+len("- name: "):]
	if k := strings.Index(s, "- name: "); k >= 0 {
		s = s[:k]
	}
	return s
}

func TestReleaseWorkflow_DMGName(t *testing.T) {
	wf := readWorkflow(t)
	if !strings.Contains(wf, "tags:\n      - 'v*'") {
		t.Error("release.yml no longer triggers on v* tags")
	}
	create := step(t, job(t, wf, "sign-macos"), "Create and sign DMG")
	if !strings.Contains(create, `VERSION="${GITHUB_REF_NAME}"`) {
		t.Errorf("VERSION is not set from GITHUB_REF_NAME:\n%s", create)
	}
	m := regexp.MustCompile(`DMG_NAME="([^"]+)"`).FindStringSubmatch(create)
	if m == nil {
		t.Fatalf("no DMG_NAME in:\n%s", create)
	}
	if got := strings.ReplaceAll(m[1], "${VERSION}", "v1.2.3"); got != DMGAssetName("1.2.3") {
		t.Errorf("tag v1.2.3 publishes %q, the updater looks for %q", got, DMGAssetName("1.2.3"))
	}
	// Verify requires the signing identifier to be the asset name without .dmg.
	id := regexp.MustCompile(`codesign --force --timestamp --identifier "([^"]+)"`).FindStringSubmatch(create)
	if id == nil {
		t.Fatalf("the DMG is not signed with an explicit --identifier:\n%s", create)
	}
	if got := strings.ReplaceAll(id[1], "${VERSION}", "v1.2.3") + ".dmg"; got != DMGAssetName("1.2.3") {
		t.Errorf("tag v1.2.3 signs identifier %q, Verify wants the asset name %q without .dmg", id[1], DMGAssetName("1.2.3"))
	}
}

func TestReleaseWorkflow_NotarizesAndStaplesDMG(t *testing.T) {
	j := job(t, readWorkflow(t), "sign-macos")
	verify := "- name: Verify DMG like the in-app updater\n"
	pos := 0
	for _, want := range []string{
		"- name: Create and sign DMG\n",
		`xcrun notarytool submit "$DMG" --keychain-profile "portty-notarize" --wait`,
		`xcrun stapler staple "$DMG"`,
		`xcrun stapler validate "$DMG"`,
		verify,
		"- name: Cleanup keychain\n",
		"- name: Upload signed DMG\n",
	} {
		i := strings.Index(j[pos:], want)
		if i < 0 {
			t.Fatalf("sign-macos lacks %q after offset %d (or it is out of order)", want, pos)
		}
		pos += i + len(want)
	}
	v := step(t, j, strings.TrimSuffix(strings.TrimPrefix(verify, "- name: "), "\n"))
	for _, want := range []string{
		`codesign --verify --strict "$DMG"`,
		"codesign -dvv",
		"'TeamIdentifier=" + TeamID + "'",
		`grep -qx "Identifier=portty-${VERSION}-darwin-universal" <<< "$DVV"`,
		"spctl -a -t open --context context:primary-signature",
		"': accepted'",
		"'source=Notarized Developer ID'",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("verify step lacks %q:\n%s", want, v)
		}
	}
}

func TestReleaseWorkflow_PrereleaseFlag(t *testing.T) {
	up := step(t, job(t, readWorkflow(t), "publish"), "Upload to GitHub Release")
	for _, want := range []string{
		"draft: ${{ contains(github.ref_name, '-rc') }}",
		"prerelease: ${{ contains(github.ref_name, '-') }}",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("publish step lacks %q (the updater skips drafts and prereleases)", want)
		}
	}
}

// WR-02: the release pipeline runs the brand gate itself (a v* tag need not have passed build.yml)
// and checks the built binary, before it is tarred, for the VM-only fake update feed.
func TestReleaseWorkflow_VMOnlyFakesGated(t *testing.T) {
	wf := readWorkflow(t)
	if !strings.Contains(job(t, wf, "validate"), "run: bash tests/check-brand.sh") {
		t.Error("the validate job does not run tests/check-brand.sh")
	}
	b := job(t, wf, "build-macos")
	gate := "- name: Release binary carries no VM-only fakes\n"
	i, j := strings.Index(b, "- name: Build macOS app\n"), strings.Index(b, gate)
	if i < 0 || j < i || j > strings.Index(b, "- name: Tar .app bundle") {
		t.Fatalf("build-macos lacks %q between the build and the tar step", gate)
	}
	if s := step(t, b, strings.TrimSuffix(strings.TrimPrefix(gate, "- name: "), "\n")); !strings.Contains(s, "grep -ac '127.0.0.1:8788' \"$BIN\"") {
		t.Errorf("the gate does not grep the binary for the fake feed address:\n%s", s)
	}
}
