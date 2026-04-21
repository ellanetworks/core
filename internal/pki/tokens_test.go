// Copyright 2026 Ella Networks

package pki_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

func TestToken_RoundTrip(t *testing.T) {
	key, err := pki.NewHMACKey()
	if err != nil {
		t.Fatal(err)
	}

	id, _ := pki.NewTokenID()

	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        2,
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(15 * time.Minute).Unix(),
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	}

	tok, err := pki.MintJoinToken(key, claims)
	if err != nil {
		t.Fatal(err)
	}

	got, err := pki.VerifyJoinToken(key, time.Now(), tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if got.TokenID != claims.TokenID || got.NodeID != claims.NodeID {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, claims)
	}
}

func TestToken_WrongKey(t *testing.T) {
	k1, _ := pki.NewHMACKey()
	k2, _ := pki.NewHMACKey()

	id, _ := pki.NewTokenID()
	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        2,
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	}

	tok, _ := pki.MintJoinToken(k1, claims)

	if _, err := pki.VerifyJoinToken(k2, time.Now(), tok); err == nil {
		t.Fatal("verify must fail with wrong key")
	}
}

func TestToken_Tampered(t *testing.T) {
	k, _ := pki.NewHMACKey()

	id, _ := pki.NewTokenID()
	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        2,
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	}

	tok, _ := pki.MintJoinToken(k, claims)

	// Flip a byte in the middle.
	b := []byte(tok)
	b[len(b)/2] ^= 0x01

	if _, err := pki.VerifyJoinToken(k, time.Now(), string(b)); err == nil {
		t.Fatal("verify must fail for tampered token")
	}
}

func TestToken_Expired(t *testing.T) {
	k, _ := pki.NewHMACKey()

	id, _ := pki.NewTokenID()
	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        2,
		IssuedAt:      time.Now().Add(-2 * time.Hour).Unix(),
		ExpiresAt:     time.Now().Add(-time.Hour).Unix(),
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	}

	tok, _ := pki.MintJoinToken(k, claims)

	if _, err := pki.VerifyJoinToken(k, time.Now(), tok); err == nil {
		t.Fatal("expired token must not verify")
	}
}

func TestToken_FutureIssued(t *testing.T) {
	k, _ := pki.NewHMACKey()

	id, _ := pki.NewTokenID()
	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        2,
		IssuedAt:      time.Now().Add(time.Hour).Unix(),
		ExpiresAt:     time.Now().Add(2 * time.Hour).Unix(),
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	}

	tok, _ := pki.MintJoinToken(k, claims)

	if _, err := pki.VerifyJoinToken(k, time.Now(), tok); err == nil {
		t.Fatal("future-dated token must not verify")
	}
}

func TestToken_BadInput(t *testing.T) {
	k, _ := pki.NewHMACKey()

	if _, err := pki.VerifyJoinToken(k, time.Now(), "not-base64-!!!"); err == nil {
		t.Fatal("should reject non-base64url input")
	}

	if _, err := pki.VerifyJoinToken(k, time.Now(), ""); err == nil {
		t.Fatal("should reject empty token")
	}
}

func TestExtractClaimsUnverified_ReadsFingerprint(t *testing.T) {
	k, _ := pki.NewHMACKey()

	id, _ := pki.NewTokenID()

	claims := pki.JoinClaims{
		TokenID:       id,
		NodeID:        7,
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		CAFingerprint: "sha256:deadbeef",
		ClusterID:     "test-cluster",
	}

	tok, err := pki.MintJoinToken(k, claims)
	if err != nil {
		t.Fatal(err)
	}

	// Anyone can read the claims without the HMAC key.
	got, err := pki.ExtractClaimsUnverified(tok)
	if err != nil {
		t.Fatalf("ExtractClaimsUnverified: %v", err)
	}

	if got.CAFingerprint != "sha256:deadbeef" {
		t.Fatalf("fingerprint = %q", got.CAFingerprint)
	}

	if got.NodeID != 7 {
		t.Fatalf("nodeID = %d", got.NodeID)
	}
}

func TestMint_MissingFingerprint(t *testing.T) {
	k, _ := pki.NewHMACKey()

	_, err := pki.MintJoinToken(k, pki.JoinClaims{
		TokenID:   "x",
		NodeID:    1,
		IssuedAt:  1,
		ExpiresAt: 10,
	})
	if err == nil {
		t.Fatal("mint without CAFingerprint must be rejected")
	}
}

func TestExtractClaimsUnverified_BadInput(t *testing.T) {
	if _, err := pki.ExtractClaimsUnverified("not-b64!!!"); err == nil {
		t.Fatal("invalid base64 must be rejected")
	}

	if _, err := pki.ExtractClaimsUnverified(""); err == nil {
		t.Fatal("empty token must be rejected")
	}
}

func TestMint_ShortKey(t *testing.T) {
	short := []byte("abc")

	_, err := pki.MintJoinToken(short, pki.JoinClaims{
		TokenID:       "x",
		NodeID:        1,
		IssuedAt:      1,
		ExpiresAt:     10,
		CAFingerprint: "sha256:abc",
		ClusterID:     "test-cluster",
	})
	if err == nil {
		t.Fatal("short key must be rejected")
	}
}
