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

// DesktopTokenKind is the Kind of every desktop token.
const DesktopTokenKind = "desktop"

// DesktopModeAlways is the Mode of an always-available desktop link. Phase 22
// guest links will add their own mode under the same route family.
const DesktopModeAlways = "always"

// desktopKeyInfo is the HKDF info string that separates the desktop key from
// the session key and the port key.
const desktopKeyInfo = "portty/desktop-token/v1"

// ErrNotDesktopToken: the signature verified but the claims are not a v1
// always-available desktop token.
var ErrNotDesktopToken = errors.New("capability: not a desktop token")

// DesktopClaims is the desktop token payload. Field declaration order is
// load-bearing for the same reason as Claims: the HMAC signs the marshalled
// bytes, and every saved link carries them. There is no expiry: the token
// lives as long as always-available is on with this grant ID and the signing
// key (turning it off revokes the grant, regenerating the key kills the token).
type DesktopClaims struct {
	Kind    string `json:"kind"`     // always DesktopTokenKind
	Mode    string `json:"mode"`     // DesktopModeAlways
	GrantID string `json:"grant_id"` // names the always-available grant
	IAT     int64  `json:"iat"`      // issued-at UNIX timestamp, persisted with the grant
	V       int    `json:"v"`        // claim schema version; always 1
}

// DesktopKey derives the key desktop tokens are signed under. It is derived,
// not the session key itself, because Verify ignores unknown fields: a token
// signed under the session key that carried kind "desktop" would authenticate
// on session routes. Under DesktopKey, Verify and VerifyPort fail their HMAC,
// and VerifyDesktop fails a session capability's and a port token's. HKDF
// separation, not the Kind field, is what blocks cross-acceptance.
func DesktopKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 {
		return nil, errors.New("capability: empty session key")
	}
	return hkdf.Key(sha256.New, sessionKey, nil, desktopKeyInfo, 32)
}

// SignDesktop serialises c as JSON and returns "b64Payload.b64Sig" with an
// HMAC-SHA256 under DesktopKey(sessionKey), the token shape Sign produces.
func SignDesktop(c DesktopClaims, sessionKey []byte) (string, error) {
	key, err := DesktopKey(sessionKey)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyDesktop authenticates a desktop token against DesktopKey(sessionKey).
// Failures are Verify's (ErrMalformedToken, ErrInvalidSignature,
// ErrMalformedClaims) plus ErrNotDesktopToken when the signed claims are not
// Kind "desktop", Mode "always", V 1 with a non-empty grant ID.
func VerifyDesktop(token string, sessionKey []byte) (DesktopClaims, error) {
	key, err := DesktopKey(sessionKey)
	if err != nil {
		return DesktopClaims{}, err
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return DesktopClaims{}, fmt.Errorf("%w: segment count", ErrMalformedToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return DesktopClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return DesktopClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return DesktopClaims{}, ErrInvalidSignature
	}
	var c DesktopClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return DesktopClaims{}, fmt.Errorf("%w: %v", ErrMalformedClaims, err)
	}
	if c.Kind != DesktopTokenKind || c.Mode != DesktopModeAlways || c.V != 1 || c.GrantID == "" {
		return DesktopClaims{}, ErrNotDesktopToken
	}
	return c, nil
}
