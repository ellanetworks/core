// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"github.com/ellanetworks/core/internal/util/ueauth"
)

// DeriveXresStar computes XRES* per TS 33.501 §A.4.
// Returns the last 128 bits of KDF(CK||IK, FC=0x6B, SN, RAND, RES).
func DeriveXresStar(ck, ik []byte, snName string, rand, res []byte) ([]byte, error) {
	key := make([]byte, 0, len(ck)+len(ik))
	key = append(key, ck...)
	key = append(key, ik...)
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

// DeriveKausf computes Kausf per TS 33.501 §A.2.
func DeriveKausf(ck, ik []byte, snName string, sqnXorAK []byte) ([]byte, error) {
	key := make([]byte, 0, len(ck)+len(ik))
	key = append(key, ck...)
	key = append(key, ik...)
	P0 := []byte(snName)

	return ueauth.GetKDFValue(
		key,
		ueauth.FCForKausfDerivation,
		P0, ueauth.KDFLen(P0),
		sqnXorAK, ueauth.KDFLen(sqnXorAK),
	)
}
