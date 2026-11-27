package desktop

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestGuestViewer_Golden: every line of signal-guest-v1.jsonl round-trips
// through the Go message types, and every close line names the code and reason
// the Go constants carry, so the guest page's table and the daemon cannot drift.
func TestGuestViewer_Golden(t *testing.T) {
	closes := map[string]int{
		"close-denied":        int(CloseGuestDenied),
		"close-admit-timeout": int(CloseGuestTimeout),
		"close-revoked":       int(CloseGuestRevoked),
		"close-share-ended":   int(CloseShareEnded),
		"close-token-invalid": int(CloseTokenRejected),
		"close-replaced":      int(CloseGuestReplaced),
		"close-full":          int(CloseFull),
	}
	reasons := map[string]string{
		"close-denied":        guestReasonDenied,
		"close-admit-timeout": guestReasonTimeout,
		"close-revoked":       guestReasonRevoked,
		"close-share-ended":   guestReasonShareEnded,
		"close-token-invalid": guestReasonTokenInvalid,
		"close-replaced":      guestReasonReplaced,
		"close-full":          guestReasonFull,
	}
	seen := map[string]bool{}
	roundTrip := func(name string, wire json.RawMessage, v any) {
		t.Helper()
		if err := json.Unmarshal(wire, v); err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		back, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var want, got bytes.Buffer
		if err := json.Compact(&want, wire); err != nil {
			t.Fatal(err)
		}
		got.Write(back)
		if want.String() != got.String() {
			t.Errorf("%s: wire %s re-encodes as %s", name, want.String(), got.String())
		}
	}
	for _, l := range readGolden(t, "testdata/signal-guest-v1.jsonl") {
		seen[l.Name] = true
		var head struct {
			Type  string `json:"type"`
			Close int    `json:"close"`
		}
		if err := json.Unmarshal(l.Wire, &head); err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		switch {
		case head.Type == typeAdmission:
			var m msgAdmission
			roundTrip(l.Name, l.Wire, &m)
			if m.State != admissionWaiting && m.State != admissionAdmitted {
				t.Errorf("%s: unknown admission state %q", l.Name, m.State)
			}
		case head.Type == typeRights:
			var m msgRights
			roundTrip(l.Name, l.Wire, &m)
			for _, k := range []string{"control", "clipboard", "surface"} {
				if _, ok := m.Rights[k]; !ok {
					t.Errorf("%s: rights lacks %q", l.Name, k)
				}
			}
		case head.Type == typeLease:
			var m msgLease
			roundTrip(l.Name, l.Wire, &m)
			switch m.State {
			case leaseYou, leaseOwner, leasePhysical, leaseFree:
				if m.Name != "" {
					t.Errorf("%s: state %q must carry no name", l.Name, m.State)
				}
			case leaseOther:
				if m.Name == "" {
					t.Errorf("%s: a guest holder's lease line must carry its name", l.Name)
				}
			default:
				t.Errorf("%s: unknown lease state %q", l.Name, m.State)
			}
		case head.Type == typeHello:
			var m msgHello
			roundTrip(l.Name, l.Wire, &m)
			if m.V != ProtocolVersion {
				t.Errorf("%s: v %d, want %d", l.Name, m.V, ProtocolVersion)
			}
			if m.Rights["control"] || m.Rights["clipboard"] || m.Rights["surface"] {
				t.Errorf("%s: a guest hello must carry every right false", l.Name)
			}
		case head.Close != 0:
			var m struct {
				Close  int    `json:"close"`
				Reason string `json:"reason"`
			}
			roundTrip(l.Name, l.Wire, &m)
			if m.Close != closes[l.Name] || m.Reason != reasons[l.Name] {
				t.Errorf("%s: wire says %d %q, Go constants say %d %q", l.Name, m.Close, m.Reason, closes[l.Name], reasons[l.Name])
			}
		default:
			t.Errorf("%s: neither a known message nor a close line", l.Name)
		}
	}
	for name := range closes {
		if !seen[name] {
			t.Errorf("golden file lacks %s", name)
		}
	}
	for _, name := range []string{"admission-waiting", "admission-admitted", "rights-view-only", "rights-control-and-clipboard",
		"lease-you", "lease-other", "lease-owner", "lease-physical", "lease-free"} {
		if !seen[name] {
			t.Errorf("golden file lacks %s", name)
		}
	}
}
