// Copyright 2026 Ella Networks

package pki

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// JoinClaims are the authenticated claims carried in a join token. They
// bind the token to a specific node-id, expiry, cluster root CA, and
// cluster identity so a leaked token cannot be replayed against an
// arbitrary node-id, a joining node can pin the server's TLS handshake
// to the root named in the claims, and a first-boot joiner can build
// a CSR with the correct SPIFFE URI without separately discovering the
// clusterID.
type JoinClaims struct {
	TokenID       string `json:"id"`
	NodeID        int    `json:"node_id"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
	CAFingerprint string `json:"caf"`
	ClusterID     string `json:"cid"`
}

// joinTokenVersion is the first byte of every serialized token; bumped if
// the format ever changes.
const joinTokenVersion byte = 1

// joinTokenMaxPayload caps the JSON claims payload to defend the parser
// against crafted headers claiming a huge length (the wire-encoded claims
// are a few hundred bytes; 64 KiB is generous).
const joinTokenMaxPayload uint32 = 64 * 1024

// MintJoinToken returns an opaque bearer string binding claims to
// hmacKey. The token encodes claims as JSON followed by an HMAC-SHA256,
// base64url-encoded. Single-use tracking is the caller's responsibility
// (see cluster_join_tokens table in migration_v9).
func MintJoinToken(hmacKey []byte, claims JoinClaims) (string, error) {
	if len(hmacKey) < 16 {
		return "", fmt.Errorf("hmac key too short (need ≥16 bytes, got %d)", len(hmacKey))
	}

	if claims.TokenID == "" {
		return "", fmt.Errorf("claims.TokenID must be set")
	}

	if claims.NodeID < MinNodeID || claims.NodeID > MaxNodeID {
		return "", fmt.Errorf("claims.NodeID %d outside [%d, %d]", claims.NodeID, MinNodeID, MaxNodeID)
	}

	if claims.ExpiresAt <= claims.IssuedAt {
		return "", fmt.Errorf("claims.ExpiresAt %d must exceed IssuedAt %d", claims.ExpiresAt, claims.IssuedAt)
	}

	if claims.CAFingerprint == "" {
		return "", fmt.Errorf("claims.CAFingerprint must be set")
	}

	if claims.ClusterID == "" {
		return "", fmt.Errorf("claims.ClusterID must be set")
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	mac := hmac.New(sha256.New, hmacKey)
	_, _ = mac.Write([]byte{joinTokenVersion})
	_, _ = mac.Write(payload)

	tag := mac.Sum(nil)

	// Format: base64url( version(1) | payloadLen(4 BE) | payload | tag(32) )
	var buf []byte

	buf = append(buf, joinTokenVersion)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(payload)))
	buf = append(buf, payload...)
	buf = append(buf, tag...)

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// NewTokenID returns a random token identifier suitable for
// JoinClaims.TokenID. 128-bit entropy, hex-encoded.
func NewTokenID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("random token id: %w", err)
	}

	return fmt.Sprintf("%x", b), nil
}

// VerifyJoinToken decodes and authenticates a token string against
// hmacKey and returns the claims. Returns an error if HMAC verification
// fails, the payload does not parse, or the token is expired relative to
// now. Single-use enforcement is the caller's job.
func VerifyJoinToken(hmacKey []byte, now time.Time, token string) (*JoinClaims, error) {
	if len(hmacKey) < 16 {
		return nil, fmt.Errorf("hmac key too short (need \u226516 bytes, got %d)", len(hmacKey))
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	const minLen = 1 + 4 + 0 + 32
	if len(raw) < minLen {
		return nil, fmt.Errorf("token too short (%d bytes)", len(raw))
	}

	if raw[0] != joinTokenVersion {
		return nil, fmt.Errorf("unknown token version %d", raw[0])
	}

	payloadLen32 := binary.BigEndian.Uint32(raw[1:5])
	if payloadLen32 > joinTokenMaxPayload {
		return nil, fmt.Errorf("token payload length %d exceeds max %d", payloadLen32, joinTokenMaxPayload)
	}

	payloadLen := int(payloadLen32)
	if 1+4+payloadLen+32 != len(raw) {
		return nil, fmt.Errorf("token length mismatch: header says %d, got %d", 1+4+payloadLen+32, len(raw))
	}

	payload := raw[5 : 5+payloadLen]
	tag := raw[5+payloadLen:]

	mac := hmac.New(sha256.New, hmacKey)
	_, _ = mac.Write(raw[0:1])
	_, _ = mac.Write(payload)

	expect := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expect, tag) != 1 {
		return nil, fmt.Errorf("token authentication failed")
	}

	var claims JoinClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	if now.Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired at %d, now %d", claims.ExpiresAt, now.Unix())
	}

	if now.Unix() < claims.IssuedAt {
		return nil, fmt.Errorf("token issued in the future (iat=%d, now=%d)", claims.IssuedAt, now.Unix())
	}

	return &claims, nil
}

// NewHMACKey returns 32 bytes of cryptographically-secure randomness for
// use as the cluster's join-token HMAC key.
func NewHMACKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("random hmac key: %w", err)
	}

	return b, nil
}

// ExtractClaimsUnverified parses a join token and returns the embedded
// claims WITHOUT verifying the HMAC. The joining node uses this to
// read the CA fingerprint from a token it has not yet authenticated, so
// it can pin the TLS handshake to the cluster root before dialling.
// The HMAC is still validated server-side when the token is presented.
func ExtractClaimsUnverified(token string) (*JoinClaims, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	const minLen = 1 + 4 + 32
	if len(raw) < minLen {
		return nil, fmt.Errorf("token too short (%d bytes)", len(raw))
	}

	if raw[0] != joinTokenVersion {
		return nil, fmt.Errorf("unknown token version %d", raw[0])
	}

	payloadLen32 := binary.BigEndian.Uint32(raw[1:5])
	if payloadLen32 > joinTokenMaxPayload {
		return nil, fmt.Errorf("token payload length %d exceeds max %d", payloadLen32, joinTokenMaxPayload)
	}

	payloadLen := int(payloadLen32)
	if 1+4+payloadLen+32 != len(raw) {
		return nil, fmt.Errorf("token length mismatch")
	}

	var claims JoinClaims
	if err := json.Unmarshal(raw[5:5+payloadLen], &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	return &claims, nil
}
