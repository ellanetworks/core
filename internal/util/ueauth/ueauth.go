// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package ueauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const (
	FCForCkPrimeIkPrimeDerivation  = "20"
	FCForKseafDerivation           = "6C"
	FCForResStarXresStarDerivation = "6B"
	FCForKausfDerivation           = "6A"
	FCForKamfDerivation            = "6D"
	FCForKgnbKn3iwfDerivation      = "6E"
	FCForNhDerivation              = "6F"
	FCForAlgorithmKeyDerivation    = "69"
)

func KDFLen(input []byte) []byte {
	r := make([]byte, 2)
	binary.BigEndian.PutUint16(r, uint16(len(input)))
	return r
}

// This function implements the KDF defined in TS.33220 cluase B.2.0.
//
// For P0-Pn, the ones that will be used directly as a string (e.g. "WLAN") should be type-casted by []byte(),
// and the ones originally in hex (e.g. "bb52e91c747a") should be converted by using hex.DecodeString().
//
// For L0-Ln, use KDFLen() function to calculate them (e.g. KDFLen(P0)).
func GetKDFValue(key []byte, FC string, param ...[]byte) ([]byte, error) {
	kdf := hmac.New(sha256.New, key)

	var S []byte
	if STmp, err := hex.DecodeString(FC); err != nil {
		return nil, fmt.Errorf("GetKDFValue FC decode failed: %+v", err)
	} else {
		S = STmp
	}

	for _, p := range param {
		S = append(S, p...)
	}

	if _, err := kdf.Write(S); err != nil {
		return nil, fmt.Errorf("GetKDFValue KDF write failed: %+v", err)
	}
	sum := kdf.Sum(nil)
	return sum, nil
}
