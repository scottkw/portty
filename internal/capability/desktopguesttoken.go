package capability

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DesktopGuestTokenKind is the Kind of every desktop guest token.
const DesktopGuestTokenKind = "desktop-guest"

// desktopGuestKeyInfo is the HKDF info string that separates the guest key from
// the session, port and desktop keys.
const desktopGuestKeyInfo = "portty/desktop-guest-token/v1"

// ErrNotDesktopGuestToken: the signature verified but the claims are not a v1
// desktop guest token.
var ErrNotDesktopGuestToken = errors.New("capability: not a desktop guest token")

// DesktopGuestClaims is the guest link's payload. Field declaration order is
// load-bearing for the same reason as Claims: the HMAC signs the marshalled
// bytes, and every link already sent carries them. The token names the share
// only, never a person: who joins comes from WhoIs on the connection. There is
// no expiry: the link lives as long as that share is running and the signing
// key is unchanged.
type DesktopGuestClaims struct {
	Kind    string `json:"kind"`     // always DesktopGuestTokenKind
	ShareID string `json:"share_id"` // 32 lowercase hex characters, one per share
	IAT     int64  `json:"iat"`      // issued-at UNIX timestamp
	V       int    `json:"v"`        // claim schema version; always 1
}

// DesktopGuestKey derives the key guest tokens are signed under. As with
// DesktopKey, separation by derived key, not the Kind field, is what stops a
// guest token authenticating on another route family and the reverse.
func DesktopGuestKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 {
		return nil, errors.New("capability: empty session key")
	}
	return hkdf.Key(sha256.New, sessionKey, nil, desktopGuestKeyInfo, 32)
}

// SignDesktopGuest sets Kind and V itself, serialises c as JSON and returns
// "b64Payload.b64Sig" with an HMAC-SHA256 under DesktopGuestKey(sessionKey).
func SignDesktopGuest(c DesktopGuestClaims, sessionKey []byte) (string, error) {
	key, err := DesktopGuestKey(sessionKey)
	if err != nil {
		return "", err
	}
	c.Kind, c.V = DesktopGuestTokenKind, 1
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyDesktopGuest authenticates a guest token against
// DesktopGuestKey(sessionKey). Failures are Verify's (ErrMalformedToken,
// ErrInvalidSignature, ErrMalformedClaims) plus ErrNotDesktopGuestToken when the
// signed claims are not Kind "desktop-guest" with V 1; a share ID that is not
// exactly 32 lowercase hex characters is ErrMalformedClaims.
func VerifyDesktopGuest(token string, sessionKey []byte) (DesktopGuestClaims, error) {
	key, err := DesktopGuestKey(sessionKey)
	if err != nil {
		return DesktopGuestClaims{}, err
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return DesktopGuestClaims{}, fmt.Errorf("%w: segment count", ErrMalformedToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return DesktopGuestClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return DesktopGuestClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return DesktopGuestClaims{}, ErrInvalidSignature
	}
	var c DesktopGuestClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return DesktopGuestClaims{}, fmt.Errorf("%w: %v", ErrMalformedClaims, err)
	}
	if c.Kind != DesktopGuestTokenKind || c.V != 1 {
		return DesktopGuestClaims{}, ErrNotDesktopGuestToken
	}
	if !isShareID(c.ShareID) {
		return DesktopGuestClaims{}, fmt.Errorf("%w: share_id", ErrMalformedClaims)
	}
	return c, nil
}

// isShareID reports whether s is exactly 32 lowercase hex characters.
func isShareID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
