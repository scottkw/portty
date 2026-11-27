package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAudit_TracerOneLine(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenAudit(dir)
	if err != nil {
		t.Fatalf("OpenAudit: %v", err)
	}
	a.now = func() time.Time { return time.Date(2026, 11, 25, 22, 41, 7, 0, time.UTC) }

	id := NewIdentity(1234, "colleague@example.com", "Pat Colleague", "pat-laptop", "nPAT1CNTRL", "192.0.2.7")
	a.Record("admit_request", "share-9f3c", "guest", id, "")

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
