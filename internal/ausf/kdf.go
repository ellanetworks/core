// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ausf

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/util/ueauth"
)

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
