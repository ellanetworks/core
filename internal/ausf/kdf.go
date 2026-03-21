// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/util/ueauth"
)

// deriveXresStar computes XRES* per TS 33.501 §A.4.
// Returns the last 128 bits of KDF(CK||IK, FC=0x6B, SN, RAND, RES).
func deriveXresStar(ck, ik []byte, snName string, rand, res []byte) ([]byte, error) {
	key := append(ck, ik...)
	P0 := []byte(snName)

	kdfVal, err := ueauth.GetKDFValue(
		key,
		ueauth.FCForResStarXresStarDerivation,
		P0, ueauth.KDFLen(P0),
		rand, ueauth.KDFLen(rand),
		res, ueauth.KDFLen(res),
	)
	if err != nil {
		return nil, err
	}

	return kdfVal[len(kdfVal)/2:], nil
}

// deriveKausf computes Kausf per TS 33.501 §A.2.
func deriveKausf(ck, ik []byte, snName string, sqnXorAK []byte) ([]byte, error) {
	key := append(ck, ik...)
	P0 := []byte(snName)

	return ueauth.GetKDFValue(
		key,
		ueauth.FCForKausfDerivation,
		P0, ueauth.KDFLen(P0),
		sqnXorAK, ueauth.KDFLen(sqnXorAK),
	)
}

// deriveHxresStar computes HXRES* = SHA-256(RAND || XRES*)[16:] (last 128 bits).
func deriveHxresStar(randHex, xresStarHex string) (string, error) {
	concat, err := hex.DecodeString(randHex + xresStarHex)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(concat)

	return hex.EncodeToString(hash[16:]), nil
}

// deriveKseaf computes Kseaf per TS 33.501 §A.6.
func deriveKseaf(kausf []byte, snName string) ([]byte, error) {
	P0 := []byte(snName)

	return ueauth.GetKDFValue(
		kausf,
		ueauth.FCForKseafDerivation,
		P0, ueauth.KDFLen(P0),
	)
}
