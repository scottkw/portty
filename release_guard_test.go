package main

// MOB-05 / T-14-11: the WhoIs fake (internal/webserver/webservertest) must not
// compile into any release-like build. The Playwright fixture's graph is the
// positive control that proves the search can find it.

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

const (
	webserverPkg = "github.com/scottkw/portty/internal/webserver"
	fakeWhoIsPkg = webserverPkg + "/webservertest"
)

// goListDeps returns `go list -deps` output. Any failure to run, or a
// non-zero exit, is an error: a broken go list must never read as "fake absent".
func goListDeps(tags, pkg string) (string, error) {
	args := []string{"list", "-deps"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, pkg)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", &goListError{args: args, err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type goListError struct {
	args   []string
	err    error
	stderr string
}

func (e *goListError) Error() string {
	return "go " + strings.Join(e.args, " ") + ": " + e.err.Error() + ": " + e.stderr
}

func TestNoFakeWhoIsInRelease(t *testing.T) {
	for _, tags := range []string{"", "wailsassets", "vmwhois"} {
		out, err := goListDeps(tags, ".")
		if err != nil {
			t.Fatalf("tags %q: %v", tags, err)
		}
		if !strings.Contains(out, webserverPkg+"\n") {
			t.Fatalf("tags %q: graph lacks %s (the check would be vacuous)", tags, webserverPkg)
		}
		if strings.Contains(out, fakeWhoIsPkg) {
			t.Errorf("tags %q: release graph contains %s", tags, fakeWhoIsPkg)
		}
	}
	out, err := goListDeps("playwrightfixture", "./cmd/playwright-fixture")
	if err != nil {
		t.Fatalf("fixture graph: %v", err)
	}
	if !strings.Contains(out, fakeWhoIsPkg) {
		t.Errorf("positive control: the fixture graph lacks %s", fakeWhoIsPkg)
	}
}

func TestGoListDeps_ReportsFailure(t *testing.T) {
	if out, err := goListDeps("", "./no/such/pkg"); err == nil {
		t.Fatalf("goListDeps on a missing package returned no error (output %q)", out)
	}
}
