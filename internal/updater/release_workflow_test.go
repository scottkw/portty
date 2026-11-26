package updater

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These tests keep .github/workflows/release.yml in step with this package: the DMG it publishes
// must be the one DMGAssetName names, and it must pass Verify before it reaches a release. They
// also pin build.yml and the bundle scripts' identity constants (REL-01).

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", rel, err)
	}
	return string(b)
}

func readWorkflow(t *testing.T) string {
	t.Helper()
	return readRepoFile(t, ".github/workflows/release.yml")
}

// before reports whether a occurs in s and b occurs after it.
func before(s, a, b string) bool {
	i := strings.Index(s, a)
	return i >= 0 && strings.Contains(s[i+len(a):], b)
}

const (
	cgoCFlags   = `CGO_CFLAGS: "-mmacosx-version-min=14.0"`
	cgoLDFlags  = `CGO_LDFLAGS: "-mmacosx-version-min=14.0"`
	releaseGate = "Release bundle is arm64-only with minos 14.0 (REL-01)"
	buildGate   = "Bundle is arm64-only with minos 14.0 (REL-01)"
	// v1.2.0's designated requirement (18.1-RESEARCH Gap 4). The office Mac's TCC grants are keyed
	// to it, so every release must keep it.
	v12DesignatedRequirement = `designated => identifier "app.portty.Portty" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] /* exists */ and certificate leaf[field.1.2.840.113635.100.6.1.13] /* exists */ and certificate leaf[subject.OU] = ` + TeamID
)

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

func TestReleaseWorkflow_Arm64Build(t *testing.T) {
	wf := readWorkflow(t)
	b := job(t, wf, "build-macos")
	build := step(t, b, "Build macOS app")
	for _, want := range []string{"wails build -platform darwin/arm64 -tags wailsassets", cgoCFlags, cgoLDFlags} {
		if !strings.Contains(build, want) {
			t.Errorf("Build macOS app lacks %q:\n%s", want, build)
		}
	}
	if strings.Contains(wf, "darwin/universal") {
		t.Error("release.yml still names the universal platform")
	}
	gate := step(t, b, releaseGate)
	if !strings.Contains(gate, "run: bash scripts/verify-bundle.sh build/bin/Portty.app") {
		t.Errorf("the unsigned bundle gate does not run verify-bundle.sh:\n%s", gate)
	}
	if !before(b, "- name: Build macOS app\n", "- name: "+releaseGate) || !before(b, "- name: "+releaseGate, "- name: Tar .app bundle") {
		t.Error("the bundle gate is not between the build and the tar step")
	}
	for _, j := range []string{b, job(t, wf, "sign-macos")} {
		if !strings.Contains(j, "name: portty-darwin-arm64-unsigned\n") {
			t.Error("the unsigned artifact is not portty-darwin-arm64-unsigned on both the upload and the download side")
		}
	}
}

func TestReleaseWorkflow_InsideOutSigning(t *testing.T) {
	wf := readWorkflow(t)
	if !strings.Contains(job(t, wf, "validate"), "run: bash tests/verify-bundle.test.sh") {
		t.Error("the validate job does not run tests/verify-bundle.test.sh")
	}
	if strings.Contains(wf, "--deep") {
		t.Error("release.yml signs or verifies with --deep")
	}
	j := job(t, wf, "sign-macos")
	sign := step(t, j, "Sign .app with hardened runtime")
	if !before(sign, "bash scripts/sign-bundle.sh build/bin/Portty.app", "bash scripts/verify-bundle.sh build/bin/Portty.app --signed") {
		t.Errorf("the sign step must run sign-bundle.sh and then the --signed gate:\n%s", sign)
	}
	if !strings.Contains(sign, `SIGN_IDENTITY="$MACOS_CERTIFICATE_NAME"`) {
		t.Errorf("the sign step does not pass the certificate name as SIGN_IDENTITY:\n%s", sign)
	}
	notarize := step(t, j, "Notarize .app")
	if !before(notarize, "xcrun stapler staple build/bin/Portty.app", "bash scripts/verify-bundle.sh build/bin/Portty.app --notarized") {
		t.Errorf("the --notarized gate must follow the staple:\n%s", notarize)
	}
	if !before(j, "- name: Sign .app with hardened runtime\n", "- name: Notarize .app\n") ||
		!before(j, "- name: Notarize .app\n", "- name: Create and sign DMG\n") {
		t.Error("sign-macos steps are not ordered sign, notarize, DMG")
	}
}

func TestBuildWorkflow_Arm64MinOS14(t *testing.T) {
	wf := readRepoFile(t, ".github/workflows/build.yml")
	b := job(t, wf, "build")
	for _, want := range []string{"platform: darwin/arm64", "build (portty, darwin/arm64, macos-latest)", "name: portty-darwin-arm64\n"} {
		if !strings.Contains(wf, want) {
			t.Errorf("build.yml lacks %q", want)
		}
	}
	if strings.Contains(wf, "darwin/universal") {
		t.Error("build.yml still names the universal platform")
	}
	if s := step(t, b, "Build with Wails"); !strings.Contains(s, cgoCFlags) || !strings.Contains(s, cgoLDFlags) {
		t.Errorf("Build with Wails lacks the macOS 14 CGO variables:\n%s", s)
	}
	if s := step(t, b, buildGate); !strings.Contains(s, "run: bash scripts/verify-bundle.sh build/bin/Portty.app") {
		t.Errorf("the bundle gate does not run verify-bundle.sh:\n%s", s)
	}
	if !before(b, "- name: Build with Wails\n", "- name: "+buildGate) || !before(b, "- name: "+buildGate, "- name: Upload macOS artifact\n") {
		t.Error("the bundle gate is not between the build and the upload")
	}
	if s := step(t, b, "Run bundle gate tests (REL-01)"); !strings.Contains(s, "run: bash tests/verify-bundle.test.sh") {
		t.Errorf("the gate tests step does not run tests/verify-bundle.test.sh:\n%s", s)
	}
}

func TestVerifyBundle_PinsTeamAndDR(t *testing.T) {
	v := readRepoFile(t, "scripts/verify-bundle.sh")
	for _, want := range []string{
		`TEAM_ID="` + TeamID + `"`,
		`BUNDLE_ID="app.portty.Portty"`,
		"DESIGNATED_REQUIREMENT='" + v12DesignatedRequirement + "'",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("verify-bundle.sh lacks %q", want)
		}
	}
	sign := readRepoFile(t, "scripts/sign-bundle.sh")
	if !strings.Contains(sign, `BUNDLE_ID="app.portty.Portty"`) {
		t.Error(`sign-bundle.sh lacks BUNDLE_ID="app.portty.Portty"`)
	}
	for _, line := range strings.Split(sign, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "--deep") {
			t.Errorf("sign-bundle.sh signs with --deep: %s", line)
		}
	}
}
