package webserver_test

// RB-03 cap-free contract for GET /api/sessions/meta: the payload carries no
// credential material at all, only the exact key set id, name, status, url.

import (
	"encoding/json"
	"github.com/scottkw/portty/internal/webserver"
	"net/http"
	"testing"
)

// TestSessionsMeta_ExactKeySet: every item in GET /api/sessions/meta has
// exactly the keys id, name, status and url. Credentials are never part of the
// discovery payload; the owner shares a session by sending its link.
func TestSessionsMeta_ExactKeySet(t *testing.T) {
	ws, client := testServer(t)
	ws.SetWhoIsClientForTest(webserver.SameUserWhoIsForTest())
	ws.SetSigningKey(ssExtTestKey)
	ws.SetSessionResolver(func(id string) (string, string, string, bool) {
		if id == "sess-rb03" {
			return "RB03 Session", "running", "host.ts.net", true
		}
		return "", "", "", false
	})
	ws.EnableSession("sess-rb03")

	resp, err := client.Get(ws.BaseURL() + "/api/sessions/meta")
	if err != nil {
		t.Fatalf("GET /api/sessions/meta: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(items), items)
	}

	item := items[0]

	want := map[string]bool{"id": true, "name": true, "status": true, "url": true}
	if len(item) != len(want) {
		t.Errorf("meta item has %d keys, want exactly %d: %v", len(item), len(want), item)
	}
	for key := range item {
		if !want[key] {
			t.Errorf("RB-03 violation: meta item carries unexpected key %q", key)
		}
	}
	for key := range want {
		if _, ok := item[key]; !ok {
			t.Errorf("meta item missing cap-free field %q", key)
		}
	}
}
