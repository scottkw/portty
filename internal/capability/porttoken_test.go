package capability_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/scottkw/portty/internal/capability"
)

func ptKey(seed byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return key
}

func ptClaims() capability.PortClaims {
	return capability.PortClaims{SID: "s1", Port: 8000, IAT: 1700000000, GrantID: "g1", Kind: capability.PortTokenKind, V: 1}
}

// ptSignRaw signs arbitrary payload bytes under PortKey, as SignPort would.
func ptSignRaw(t *testing.T, payload []byte, key []byte) string {
	t.Helper()
	pk, err := capability.PortKey(key)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, pk)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestPortToken_RoundTrip(t *testing.T) {
	key := ptKey(0)
	tok, err := capability.SignPort(ptClaims(), key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := capability.VerifyPort(tok, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != ptClaims() {
		t.Errorf("got %+v, want %+v", got, ptClaims())
	}
}

func TestPortToken_NotASessionCapability(t *testing.T) {
	key := ptKey(0)
	tok, _ := capability.SignPort(ptClaims(), key)
	if _, err := capability.Verify(tok, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("Verify of a port token: %v, want ErrInvalidSignature", err)
	}
}

func TestPortToken_SessionCapNotAPortToken(t *testing.T) {
	key := ptKey(0)
	tok, _ := capability.Sign(capability.Claims{SID: "s1", Perms: "read,write", GrantID: "g1", V: 1}, key)
	if _, err := capability.VerifyPort(tok, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("VerifyPort of a session capability: %v, want ErrInvalidSignature", err)
	}
}

func TestPortToken_Tamper(t *testing.T) {
	key := ptKey(0)
	tok, _ := capability.SignPort(ptClaims(), key)
	sig := tok[bytes.IndexByte([]byte(tok), '.')+1:]

	other := ptClaims()
	other.Port = 8001
	payload, _ := json.Marshal(other)
	if _, err := capability.VerifyPort(base64.RawURLEncoding.EncodeToString(payload)+"."+sig, key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("changed port: %v, want ErrInvalidSignature", err)
	}
	// Flipping the signature's last character.
	bad := []byte(tok)
	if bad[len(bad)-1] == 'A' {
		bad[len(bad)-1] = 'B'
	} else {
		bad[len(bad)-1] = 'A'
	}
	if _, err := capability.VerifyPort(string(bad), key); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("bad signature: %v, want ErrInvalidSignature", err)
	}

	kind, ver := ptClaims(), ptClaims()
	kind.Kind, ver.V = "cap", 2
	for name, c := range map[string]capability.PortClaims{"kind": kind, "version": ver} {
		p, _ := json.Marshal(c)
		if _, err := capability.VerifyPort(ptSignRaw(t, p, key), key); !errors.Is(err, capability.ErrWrongKind) {
			t.Errorf("%s: %v, want ErrWrongKind", name, err)
		}
	}

	if _, err := capability.VerifyPort(ptSignRaw(t, []byte("not json"), key), key); !errors.Is(err, capability.ErrMalformedClaims) {
		t.Errorf("non-JSON payload: %v, want ErrMalformedClaims", err)
	}
	for _, s := range []string{"", "onesegment", "!!!.!!!", tok[:bytes.IndexByte([]byte(tok), '.')]} {
		if _, err := capability.VerifyPort(s, key); !errors.Is(err, capability.ErrMalformedToken) {
			t.Errorf("VerifyPort(%q): %v, want ErrMalformedToken", s, err)
		}
	}
}

func TestPortToken_KeyRotation(t *testing.T) {
	tok, _ := capability.SignPort(ptClaims(), ptKey(0))
	if _, err := capability.VerifyPort(tok, ptKey(1)); !errors.Is(err, capability.ErrInvalidSignature) {
		t.Errorf("rotated key: %v, want ErrInvalidSignature", err)
	}
}

func TestPortKey_Derived(t *testing.T) {
	a, err := capability.PortKey(ptKey(0))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := capability.PortKey(ptKey(0))
	b, _ := capability.PortKey(ptKey(1))
	switch {
	case len(a) != 32:
		t.Errorf("len = %d, want 32", len(a))
	case !bytes.Equal(a, again):
		t.Error("not deterministic")
	case bytes.Equal(a, ptKey(0)):
		t.Error("equals the session key")
	case bytes.Equal(a, b):
		t.Error("two session keys give one port key")
	}
	if _, err := capability.PortKey(nil); err == nil {
		t.Error("empty session key accepted")
	}
}

func FuzzVerifyPort(f *testing.F) {
	key := ptKey(0)
	good, _ := capability.SignPort(ptClaims(), key)
	session, _ := capability.Sign(capability.Claims{SID: "s1", Perms: "read", V: 1}, key)
	for _, s := range []string{good, session, "", "."} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, token string) {
		c, err := capability.VerifyPort(token, key)
		if err == nil && (c.Kind != capability.PortTokenKind || c.V != 1) {
			t.Errorf("accepted %+v", c)
		}
	})
}
