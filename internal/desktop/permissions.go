package desktop

// permissions.go: how the daemon learns the live Screen Recording, Accessibility
// and pointer-event permissions. A long-running process keeps the first answer
// the OS gave it (research C1), so the daemon never calls the preflight itself:
// it asks a freshly spawned `portty daemon perm-probe` child, and caches the
// answer for one second so several callers share one spawn.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

const (
	probeCacheTTL = time.Second
	probeTimeout  = 3 * time.Second
	probeMaxOut   = 1024
)

// errRefusedInTest is what every real child spawn and store read returns under
// `go test`, so no test reads TCC or the approval store.
var errRefusedInTest = errors.New("desktop: refused in tests")

// probeRunner runs the child and returns its stdout.
type probeRunner func(ctx context.Context, exe string, args ...string) ([]byte, error)

// Prober reads permissions through the perm-probe child.
type Prober struct {
	run probeRunner
	now func() time.Time

	mu     sync.Mutex // held across a spawn: concurrent callers wait, then hit the cache
	last   screencap.Permissions
	lastAt time.Time
	have   bool
}

// NewProber returns a Prober that spawns this executable.
func NewProber() *Prober { return &Prober{run: runChild, now: time.Now} }

// NewProberWithRunner is NewProber with the child spawn replaced, so a caller
// in another package (the daemon's endpoint tests) can drive a real Prober, its
// cache and its shared spawn, without ever running a child.
func NewProberWithRunner(run func(ctx context.Context, exe string, args ...string) ([]byte, error)) *Prober {
	return &Prober{run: run, now: time.Now}
}

var defaultProber = NewProber()

// DefaultProber is the one shared instance, so the onboarding endpoint and the
// state loop share spawns.
func DefaultProber() *Prober { return defaultProber }

func runChild(ctx context.Context, exe string, args ...string) ([]byte, error) {
	if testing.Testing() {
		return nil, errRefusedInTest
	}
	return exec.CommandContext(ctx, exe, args...).Output()
}

// Read returns the live permissions: the cached value when under a second old,
// otherwise a fresh child's. Any failure is an error, never a default false.
func (p *Prober) Read(ctx context.Context) (screencap.Permissions, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now().Round(0)
	if age := now.Sub(p.lastAt); p.have && age >= 0 && age < probeCacheTTL {
		return p.last, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return screencap.Permissions{}, fmt.Errorf("perm-probe: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out, err := p.run(ctx, exe, "daemon", "perm-probe")
	if err != nil {
		return screencap.Permissions{}, fmt.Errorf("perm-probe: %w", err)
	}
	perms, err := decodeProbe(out)
	if err != nil {
		return screencap.Permissions{}, fmt.Errorf("perm-probe: %w", err)
	}
	p.last, p.lastAt, p.have = perms, now, true
	return perms, nil
}

// decodeProbe accepts exactly {"sr":bool,"ax":bool,"pe":bool}: an unknown field,
// a missing key or trailing data is an error (T-19-32).
func decodeProbe(out []byte) (screencap.Permissions, error) {
	if len(out) > probeMaxOut {
		return screencap.Permissions{}, errors.New("output too long")
	}
	var raw struct {
		SR *bool `json:"sr"`
		AX *bool `json:"ax"`
		PE *bool `json:"pe"`
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return screencap.Permissions{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return screencap.Permissions{}, errors.New("trailing data")
	}
	if raw.SR == nil || raw.AX == nil || raw.PE == nil {
		return screencap.Permissions{}, errors.New("missing key")
	}
	return screencap.Permissions{SR: *raw.SR, AX: *raw.AX, PE: *raw.PE}, nil
}

// approvalStoreRel is replayd's approval store (R19). Only a process with Full
// Disk Access can read it, which Portty never asks for, so a read error means
// "signal unavailable", never a lapse.
const approvalStoreRel = "Library/Group Containers/group.com.apple.replayd/ScreenCaptureApprovals.plist"

// readApprovalStore returns this app bundle's kScreenCaptureApprovalLastAlerted
// date through `plutil -p`.
func readApprovalStore(ctx context.Context) (time.Time, error) {
	if testing.Testing() {
		return time.Time{}, errRefusedInTest
	}
	exe, err := os.Executable()
	if err != nil {
		return time.Time{}, err
	}
	bundle, ok := bundlePath(exe)
	if !ok {
		return time.Time{}, errors.New("not running from an app bundle")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return time.Time{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-p", filepath.Join(home, approvalStoreRel)).Output()
	if err != nil {
		return time.Time{}, err
	}
	return parseLastAlerted(string(out), bundle)
}

// bundlePath returns the .app directory (no trailing slash) containing exe.
func bundlePath(exe string) (string, bool) {
	i := strings.Index(exe, ".app/")
	if i < 0 {
		return "", false
	}
	return exe[:i+len(".app")], true
}

const lastAlertedKey = `"kScreenCaptureApprovalLastAlerted" => `

// parseLastAlerted reads `plutil -p` output: entries keyed by the bundle's file
// URL (trailing slash), each a dict holding the LastAlerted date. shortcut: the
// text format is parsed line by line; an unexpected shape is an error and the
// signal falls back to Portty's own record.
func parseLastAlerted(out, bundle string) (time.Time, error) {
	inEntry := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"file://`) && strings.HasSuffix(line, "=> {") {
			key := strings.Trim(strings.TrimSuffix(line, "=> {"), `" `)
			u, err := url.Parse(key)
			inEntry = err == nil && strings.TrimSuffix(u.Path, "/") == bundle
			continue
		}
		if inEntry && strings.HasPrefix(line, lastAlertedKey) {
			return time.Parse("2006-01-02 15:04:05 -0700", strings.TrimPrefix(line, lastAlertedKey))
		}
	}
	return time.Time{}, errors.New("no approval entry for this app")
}
