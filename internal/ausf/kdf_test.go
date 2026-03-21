// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestDeriveXresStar(t *testing.T) {
	ck, _ := hex.DecodeString("b7a3f8e24c1d5600f9e2a87c4b0e3d12")
	ik, _ := hex.DecodeString("a1b2c3d4e5f60718293a4b5c6d7e8f90")
	rand, _ := hex.DecodeString("deadbeef12345678abcdef0123456789")
	res, _ := hex.DecodeString("0102030405060708")
	snName := "5G:mnc001.mcc001.3gppnetwork.org"

	xresStar, err := deriveXresStar(ck, ik, snName, rand, res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(xresStar) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(xresStar))
	}

	// Calling again with same inputs must produce the same output (deterministic).
	xresStar2, err := deriveXresStar(ck, ik, snName, rand, res)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if hex.EncodeToString(xresStar) != hex.EncodeToString(xresStar2) {
		t.Fatalf("expected deterministic output")
	}
}

func TestDeriveXresStar_DifferentInputsProduceDifferentOutputs(t *testing.T) {
	ck, _ := hex.DecodeString("b7a3f8e24c1d5600f9e2a87c4b0e3d12")
	ik, _ := hex.DecodeString("a1b2c3d4e5f60718293a4b5c6d7e8f90")
	rand, _ := hex.DecodeString("deadbeef12345678abcdef0123456789")
	res1, _ := hex.DecodeString("0102030405060708")
	res2, _ := hex.DecodeString("0807060504030201")
	snName := "5G:mnc001.mcc001.3gppnetwork.org"

	out1, err := deriveXresStar(ck, ik, snName, rand, res1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out2, err := deriveXresStar(ck, ik, snName, rand, res2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hex.EncodeToString(out1) == hex.EncodeToString(out2) {
		t.Fatalf("different RES should produce different XRES*")
	}
}

func TestDeriveKausf(t *testing.T) {
	ck, _ := hex.DecodeString("b7a3f8e24c1d5600f9e2a87c4b0e3d12")
	ik, _ := hex.DecodeString("a1b2c3d4e5f60718293a4b5c6d7e8f90")
	sqnXorAK, _ := hex.DecodeString("aabbccddeeff")
	snName := "5G:mnc001.mcc001.3gppnetwork.org"

	kausf, err := deriveKausf(ck, ik, snName, sqnXorAK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kausf) != 32 {
		t.Fatalf("expected 32 bytes (256 bits), got %d", len(kausf))
	}

	// Deterministic
	kausf2, err := deriveKausf(ck, ik, snName, sqnXorAK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hex.EncodeToString(kausf) != hex.EncodeToString(kausf2) {
		t.Fatalf("expected deterministic output")
	}
}

func TestDeriveHxresStar(t *testing.T) {
	randHex := "deadbeef12345678abcdef0123456789"
	xresStarHex := "0102030405060708090a0b0c0d0e0f10"

	hxresStar, err := deriveHxresStar(randHex, xresStarHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually verify: SHA-256(RAND || XRES*), take last 16 bytes
	concat, _ := hex.DecodeString(randHex + xresStarHex)
	hash := sha256.Sum256(concat)
	expected := hex.EncodeToString(hash[16:])

	if hxresStar != expected {
		t.Fatalf("expected %s, got %s", expected, hxresStar)
	}
}

func TestDeriveHxresStar_InvalidHex(t *testing.T) {
	_, err := deriveHxresStar("not-hex", "0102030405060708090a0b0c0d0e0f10")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestDeriveKseaf(t *testing.T) {
	kausf, _ := hex.DecodeString("aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344")
	snName := "5G:mnc001.mcc001.3gppnetwork.org"

	kseaf, err := deriveKseaf(kausf, snName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kseaf) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(kseaf))
	}

	// Deterministic
	kseaf2, err := deriveKseaf(kausf, snName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hex.EncodeToString(kseaf) != hex.EncodeToString(kseaf2) {
		t.Fatalf("expected deterministic output")
	}
}

func TestDeriveKseaf_DifferentSN(t *testing.T) {
	kausf, _ := hex.DecodeString("aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344")

	kseaf1, err := deriveKseaf(kausf, "5G:mnc001.mcc001.3gppnetwork.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kseaf2, err := deriveKseaf(kausf, "5G:mnc002.mcc002.3gppnetwork.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hex.EncodeToString(kseaf1) == hex.EncodeToString(kseaf2) {
		t.Fatalf("different SN names should produce different Kseaf")
	}
}
