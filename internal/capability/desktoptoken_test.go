package capability_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/capability"
)

func dtClaims() capability.DesktopClaims {
	return capability.DesktopClaims{
		Kind: capability.DesktopTokenKind, Mode: capability.DesktopModeAlways,
		GrantID: "g1", IAT: 1700000000, V: 1,
	}
}

// dtSignRaw signs arbitrary payload bytes under DesktopKey, as SignDesktop would.
func dtSignRaw(t *testing.T, payload, sessionKey []byte) string {
	t.Helper()
	dk, err := capability.DesktopKey(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, dk)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestDesktopToken_RoundTripAndSeparation(t *testing.T) {
	key := ptKey(0)
	tok, err := capability.SignDesktop(dtClaims(), key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := capability.VerifyDesktop(tok, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != dtClaims() {
		t.Errorf("round trip: got %+v, want %+v", got, dtClaims())
	}

	// A desktop token is neither a session capability nor a port token.
	if _, err := capability.Verify(tok, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("Verify of a desktop token: %v, want ErrInvalidSignature", err)
	}
	if _, err := capability.VerifyPort(tok, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("VerifyPort of a desktop token: %v, want ErrInvalidSignature", err)
	}

	// And neither of those is a desktop token.
	session, _ := capability.Sign(capability.Claims{SID: "s1", Perms: "read,write", GrantID: "g1", V: 1}, key)
	port, _ := capability.SignPort(ptClaims(), key)
	for name, other := range map[string]string{"session capability": session, "port token": port} {
		if _, err := capability.VerifyDesktop(other, key); !errors.Is(err, capability.ErrInvalidSignature) {
			t.Errorf("VerifyDesktop of a %s: %v, want ErrInvalidSignature", name, err)
		}
	}

	// A token signed under the session key itself, with desktop claims, still
	// fails: HKDF separation, not the Kind field, is the barrier.
	payload, _ := json.Marshal(dtClaims())
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	forged := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := capability.VerifyDesktop(forged, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("VerifyDesktop of a session-key-signed desktop payload: %v, want ErrInvalidSignature", err)
	}

	// Another key (Regenerate Signing Key) kills the token.
	if _, err := capability.VerifyDesktop(tok, ptKey(1)); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("rotated key: %v, want ErrInvalidSignature", err)
	}

	// Tampering with the payload or the signature fails.
	dot := strings.IndexByte(tok, '.')
	other := dtClaims()
	other.GrantID = "g2"
	p2, _ := json.Marshal(other)
	if _, err := capability.VerifyDesktop(base64.RawURLEncoding.EncodeToString(p2)+tok[dot:], key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("changed grant: %v, want ErrInvalidSignature", err)
	}
	bad := []byte(tok)
	if bad[len(bad)-1] == 'A' {
		bad[len(bad)-1] = 'B'
	} else {
		bad[len(bad)-1] = 'A'
	}
	if _, err := capability.VerifyDesktop(string(bad), key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("bad signature: %v, want ErrInvalidSignature", err)
	}

	// An empty key is an error everywhere, never a token under a zero key.
	if _, err := capability.SignDesktop(dtClaims(), nil); err == nil {
		t.Error("SignDesktop accepted an empty key")
	}
	if _, err := capability.VerifyDesktop(tok, nil); err == nil {
		t.Error("VerifyDesktop accepted an empty key")
	}
}

func TestDesktopToken_ClaimsRejects(t *testing.T) {
	key := ptKey(0)

	kind, mode, ver, grant := dtClaims(), dtClaims(), dtClaims(), dtClaims()
	kind.Kind, mode.Mode, ver.V, grant.GrantID = "port", "guest", 2, ""
	for name, c := range map[string]capability.DesktopClaims{"kind": kind, "mode": mode, "version": ver, "empty grant": grant} {
		p, _ := json.Marshal(c)
		if _, err := capability.VerifyDesktop(dtSignRaw(t, p, key), key); !errors.Is(err, capability.ErrNotDesktopToken) {
			t.Errorf("%s: %v, want ErrNotDesktopToken", name, err)
		}
	}

	// The marshalled key order is the signed format of every saved link.
	tok, _ := capability.SignDesktop(dtClaims(), key)
	raw, err := base64.RawURLEncoding.DecodeString(tok[:strings.IndexByte(tok, '.')])
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"kind":"desktop","mode":"always","grant_id":"g1","iat":1700000000,"v":1}`
	if string(raw) != want {
		t.Errorf("payload %s, want %s", raw, want)
	}

	if _, err := capability.VerifyDesktop(dtSignRaw(t, []byte("not json"), key), key); !errors.Is(err, capability.ErrMalformedClaims) {
		t.Errorf("non-JSON payload: %v, want ErrMalformedClaims", err)
	}
	for _, s := range []string{"", "onesegment", "!!!.!!!", tok[:bytes.IndexByte([]byte(tok), '.')], tok[:bytes.IndexByte([]byte(tok), '.')+1] + "!!!"} {
		if _, err := capability.VerifyDesktop(s, key); !errors.Is(err, capability.ErrMalformedToken) {
			t.Errorf("VerifyDesktop(%q): %v, want ErrMalformedToken", s, err)
		}
	}
}

func TestDesktopKey_Derived(t *testing.T) {
	a, err := capability.DesktopKey(ptKey(0))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := capability.DesktopKey(ptKey(0))
	port, _ := capability.PortKey(ptKey(0))
	switch {
	case len(a) != 32:
		t.Errorf("len = %d, want 32", len(a))
	case !bytes.Equal(a, again):
		t.Error("not deterministic")
	case bytes.Equal(a, ptKey(0)):
		t.Error("equals the session key")
	case bytes.Equal(a, port):
		t.Error("equals the port key")
	}
}
