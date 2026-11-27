package daemon

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

// statsKeys flattens one level of nesting: "quality.step".
func statsKeys(m map[string]any) []string {
	var keys []string
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			for sk := range sub {
				keys = append(keys, k+"."+sk)
			}
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func getStats(t *testing.T, e *desktopEnv) (map[string]any, string) {
	t.Helper()
	var raw json.RawMessage
	if err := e.client.doJSON("GET", "/desktop/stats", nil, &raw); err != nil {
		t.Fatalf("GET /desktop/stats: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("stats is not a JSON object: %v", err)
	}
	return m, string(raw)
}

func TestDesktopStats_OffAnswers503(t *testing.T) {
	e := newDesktopEnv(t)
	var raw json.RawMessage
	err := e.client.doJSON("GET", "/desktop/stats", nil, &raw)
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("stats with the desktop off = %v; want a 503", err)
	}
}

func TestDesktopStats_ClosedKeySet(t *testing.T) {
	e := newDesktopEnv(t)
	e.mustOn(t)
	m, _ := getStats(t, e)
	want := []string{
		"audio", "goroutines", "own_viewers", "quality.fps", "quality.long_edge", "quality.no_feedback",
		"quality.slow", "quality.step", "quality.target_bps", "state", "still",
		"unlock.failures", "unlock.relock_armed", "unlock.state", "viewers",
	}
	if got := statsKeys(m); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("stats keys = %v\nwant %v (a new key needs a privacy review)", got, want)
	}
}

func TestDesktopStats_ViewerRaisesCountAndCarriesNoSecret(t *testing.T) {
	e := newDesktopEnv(t)
	resp := e.mustOn(t)
	v := connectViewer(t, resp.URL)
	v.waitIDR(t, 10*time.Second)
	var m map[string]any
	var body string
	eventually(t, 3*time.Second, "stats to read one viewer", func() bool {
		m, body = getStats(t, e)
		return m["viewers"] == float64(1)
	})
	if m["state"] != "streaming" || m["audio"] != "ok" {
		t.Errorf("stats state/audio = %v/%v; want streaming/ok", m["state"], m["audio"])
	}
	u, _ := url.Parse(resp.URL)
	for _, bad := range []string{u.Query().Get("cap"), "cap=", "http", "sdp", "100."} {
		if bad != "" && strings.Contains(body, bad) {
			t.Errorf("stats body contains %q: %s", bad, body)
		}
	}
	// Every string value is one of a few fixed words: nothing a viewer or the user typed can reach it.
	for k, v := range m {
		if s, ok := v.(string); ok && len(s) > 24 {
			t.Errorf("stats %s = %q is long enough to be an identifier", k, s)
		}
	}
}
