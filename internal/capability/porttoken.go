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

// PortTokenKind is the Kind of every port token.
const PortTokenKind = "port"

// portKeyInfo is the HKDF info string that separates the port key from the
// session key.
const portKeyInfo = "portty/port-token/v1"

// ErrWrongKind: the signature verified but the claims are not a v1 port token.
var ErrWrongKind = errors.New("capability: not a port token")

// PortClaims is the port token payload. Field declaration order is load-bearing
// for the same reason as Claims: the HMAC signs the marshalled bytes. There is
// no expiry: a port token lives as long as its grant and the signing key
// (stopping the share ends the grant, regenerating the key kills the token).
type PortClaims struct {
	SID     string `json:"sid"`      // session the grant belongs to
	Port    int    `json:"port"`     // the one loopback port it reaches
	IAT     int64  `json:"iat"`      // issued-at UNIX timestamp
	GrantID string `json:"grant_id"` // 128-bit random ID, names the grant
	Kind    string `json:"kind"`     // always PortTokenKind
	V       int    `json:"v"`        // claim schema version; always 1
}

// PortKey derives the key port tokens are signed under. It is derived, not the
// session key itself, because Verify ignores unknown fields and the session
// routes' verifyCapability never reads perms: a token signed under the session
// key that carried sid and grant_id would authenticate on session routes. Under
// PortKey, Verify fails its HMAC, and VerifyPort fails a session capability's.
func PortKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 {
		return nil, errors.New("capability: empty session key")
	}
	return hkdf.Key(sha256.New, sessionKey, nil, portKeyInfo, 32)
}

// SignPort serialises c as JSON and returns "b64Payload.b64Sig" with an
// HMAC-SHA256 under PortKey(sessionKey), the token shape Sign produces.
func SignPort(c PortClaims, sessionKey []byte) (string, error) {
	key, err := PortKey(sessionKey)
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

// VerifyPort authenticates a port token against PortKey(sessionKey). Failures
// are Verify's (ErrMalformedToken, ErrInvalidSignature, ErrMalformedClaims)
// plus ErrWrongKind when the signed claims are not Kind "port" and V 1.
func VerifyPort(token string, sessionKey []byte) (PortClaims, error) {
	key, err := PortKey(sessionKey)
	if err != nil {
		return PortClaims{}, err
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return PortClaims{}, fmt.Errorf("%w: segment count", ErrMalformedToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return PortClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return PortClaims{}, fmt.Errorf("%w: %v", ErrMalformedToken, err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return PortClaims{}, ErrInvalidSignature
	}
	var c PortClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return PortClaims{}, fmt.Errorf("%w: %v", ErrMalformedClaims, err)
	}
	if c.Kind != PortTokenKind || c.V != 1 {
		return PortClaims{}, ErrWrongKind
	}
	return c, nil
}
