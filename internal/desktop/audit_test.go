package desktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var testGuest = NewIdentity(1234, "colleague@example.com", "Pat Colleague", "pat-laptop", "nPAT1CNTRL", "192.0.2.7")

func TestAudit_TracerOneLine(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatalf("OpenAudit: %v", err)
	}
	a.now = func() time.Time { return time.Date(2026, 11, 25, 22, 41, 7, 0, time.UTC) }

	a.Record("admit_request", "share-9f3c", "guest", testGuest, "")

	path := filepath.Join(dir, "desktop-audit.jsonl")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 0600", st.Mode().Perm())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"ts":"2026-11-25T22:41:07Z","event":"admit_request","share_id":"share-9f3c","mode":"guest","user_id":1234,"login_name":"colleague@example.com","display_name":"Pat Colleague","node_name":"pat-laptop","node_stable_id":"nPAT1CNTRL","tailnet_ip":"192.0.2.7"}` + "\n"
	if string(got) != want {
		t.Fatalf("line mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestAudit_OpenNarrowsExistingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-audit.jsonl")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 0600", st.Mode().Perm())
	}
}

func TestAudit_ReasonKeyAndConstants(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	reasons := []string{
		ReasonViewerClosed, ReasonRevoked, ReasonShareStopped, ReasonGuiGone, ReasonDaemonExit,
		ReasonKeyRegenerated, ReasonTimeout, ReasonReplaced, ReasonMediaIdentityMismatch, ReasonDenied,
	}
	wantReasons := []string{
		"viewer_closed", "revoked", "share_stopped", "gui_gone", "daemon_exit",
		"key_regenerated", "timeout", "replaced", "media_identity_mismatch", "denied",
	}
	for i, r := range reasons {
		if r != wantReasons[i] {
			t.Fatalf("reason constant %d = %q, want %q", i, r, wantReasons[i])
		}
		a.Record(AuditLeave, "s", AuditModeGuest, testGuest, r)
	}
	a.Record(AuditJoin, "", AuditModeOwn, testGuest, "")
	_ = a.Close()

	events := []string{
		AuditShareStart, AuditShareStop, AuditAdmitRequest, AuditAdmitted, AuditDenied, AuditJoin,
		AuditLeave, AuditRevoked, AuditControlGranted, AuditControlRevoked, AuditClipboardGranted, AuditClipboardRevoked,
	}
	wantEvents := []string{
		"share_start", "share_stop", "admit_request", "admitted", "denied", "join",
		"leave", "revoked", "control_granted", "control_revoked", "clipboard_granted", "clipboard_revoked",
	}
	for i, e := range events {
		if e != wantEvents[i] {
			t.Fatalf("event constant %d = %q, want %q", i, e, wantEvents[i])
		}
	}
	if AuditModeGuest != "guest" || AuditModeOwn != "own" {
		t.Fatal("mode constants wrong")
	}

	lines := readLines(t, filepath.Join(dir, "desktop-audit.jsonl"))
	if len(lines) != len(reasons)+1 {
		t.Fatalf("got %d lines, want %d", len(lines), len(reasons)+1)
	}
	for i, l := range lines[:len(reasons)] {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		if m["reason"] != wantReasons[i] {
			t.Fatalf("line %d reason = %v, want %q", i, m["reason"], wantReasons[i])
		}
	}
	if strings.Contains(lines[len(reasons)], `"reason"`) {
		t.Fatalf("empty reason must omit the key: %s", lines[len(reasons)])
	}
}

func TestAudit_RotatesAtCap(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	a.now = func() time.Time { return time.Date(2026, 11, 25, 22, 41, 7, 0, time.UTC) }
	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	lineLen := fileSize(t, a.path)
	a.max = lineLen*2 + lineLen/2 // room for two lines, not three

	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "") // 2 lines
	if _, err := os.Stat(a.path + ".1"); err == nil {
		t.Fatal("rotated before the cap was passed")
	}
	a.Record(AuditLeave, "s", AuditModeGuest, testGuest, ReasonViewerClosed) // would pass the cap
	if n := len(readLines(t, a.path+".1")); n != 2 {
		t.Fatalf(".1 has %d lines, want 2", n)
	}
	cur := readLines(t, a.path)
	if len(cur) != 1 || !strings.Contains(cur[0], `"event":"leave"`) {
		t.Fatalf("current file = %v, want only the leave line", cur)
	}
	if st, _ := os.Stat(a.path); st.Mode().Perm() != 0600 {
		t.Fatalf("rotated file mode = %o, want 0600", st.Mode().Perm())
	}

	// A second rotation replaces the old .1.
	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	old := readLines(t, a.path+".1")
	// current held [leave, join] when the next join rotated it.
	if len(old) != 2 || !strings.Contains(old[0], `"event":"leave"`) {
		t.Fatalf(".1 after second rotation = %v", old)
	}
}

func TestAudit_RestartSeedsSizeFromStat(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	_ = a.Close()
	size := fileSize(t, a.path)

	// Restart below the cap: no rotation.
	b, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.size != size {
		t.Fatalf("seeded size = %d, want %d", b.size, size)
	}
	b.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	if _, err := os.Stat(b.path + ".1"); err == nil {
		t.Fatal("a restart below the cap rotated")
	}
	_ = b.Close()

	// Restart with the file already at the cap: the seeded size rotates it.
	c, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	c.max = c.size
	c.Record(AuditLeave, "s", AuditModeGuest, testGuest, "")
	if n := len(readLines(t, c.path+".1")); n != 2 {
		t.Fatalf(".1 has %d lines, want 2 (the seeded file)", n)
	}
}

func TestAudit_OpenMissingDirErrors(t *testing.T) {
	if _, err := OpenAudit(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("OpenAudit on a missing directory must fail")
	}
}

type logCount struct {
	mu sync.Mutex
	n  int
}

func (l *logCount) log(string, error) { l.mu.Lock(); l.n++; l.mu.Unlock() }
func (l *logCount) count() int        { l.mu.Lock(); defer l.mu.Unlock(); return l.n }

func TestAudit_WriteFailureNeverBlocksAndLogsOncePerMinute(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	var lc logCount
	a.logf = lc.log
	clock := time.Date(2026, 11, 25, 22, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return clock }
	_ = a.f.Close() // every write now fails

	done := make(chan struct{})
	go func() {
		a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
		a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Record blocked on a failing write")
	}
	if lc.count() != 1 {
		t.Fatalf("logged %d times within a minute, want 1", lc.count())
	}
	clock = clock.Add(61 * time.Second)
	a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	if lc.count() != 2 {
		t.Fatalf("logged %d times after a minute, want 2", lc.count())
	}
}

func TestAudit_RotateFailureNeverBlocks(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	var lc logCount
	a.logf = lc.log
	a.max = 1 // every record wants to rotate
	if err := os.Remove(a.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0700) }()

	start := time.Now()
	for range 5 {
		a.Record(AuditJoin, "s", AuditModeGuest, testGuest, "")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("Record blocked on a failing rotation")
	}
	if lc.count() != 1 {
		t.Fatalf("logged %d times within a minute, want 1", lc.count())
	}
}

func TestAudit_ConcurrentRecordsStayWholeLines(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for range 200 {
				a.Record(AuditControlGranted, "share", AuditModeGuest, NewIdentity(int64(g), "user@example.com", "Name", "node", "nSTABLE", "192.0.2.1"), "")
			}
		})
	}
	wg.Wait()
	_ = a.Close()

	lines := readLines(t, filepath.Join(dir, "desktop-audit.jsonl"))
	if len(lines) != 1600 {
		t.Fatalf("got %d lines, want 1600", len(lines))
	}
	for i, l := range lines {
		if !json.Valid([]byte(l)) {
			t.Fatalf("line %d is not valid JSON: %q", i, l)
		}
	}
}

func TestAudit_CloseTwiceReturnsErrClosed(t *testing.T) {
	a, err := OpenAudit(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err == nil || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("second Close = %v, want os.ErrClosed", err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}
