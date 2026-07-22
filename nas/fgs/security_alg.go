// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// NAS security algorithm identifiers (TS 33.501 §9.4.2.3). NEA3/NIA3 (128-ZUC)
// are defined for completeness but not implemented — Ella does not offer ZUC.
const (
	AlgCiphering128NEA0 uint8 = 0x00 // NULL
	AlgCiphering128NEA1 uint8 = 0x01 // 128-SNOW3G
	AlgCiphering128NEA2 uint8 = 0x02 // 128-AES
	AlgCiphering128NEA3 uint8 = 0x03 // 128-ZUC

	AlgIntegrity128NIA0 uint8 = 0x00 // NULL
	AlgIntegrity128NIA1 uint8 = 0x01 // 128-SNOW3G
	AlgIntegrity128NIA2 uint8 = 0x02 // 128-AES
	AlgIntegrity128NIA3 uint8 = 0x03 // 128-ZUC
)

// Algorithm type distinguishers P0 for NAS key derivation with FC=0x69
// (TS 33.501 Annex A.8, table A.8-1).
const (
	NNASEncAlg uint8 = 0x01
	NNASIntAlg uint8 = 0x02
)

// Direction inputs to the NAS integrity/cipher algorithms (TS 33.501 Annex D).
const (
	DirectionUplink   = common.DirectionUplink
	DirectionDownlink = common.DirectionDownlink
)

// Bearer3GPP is the BEARER input to the NAS algorithms: the NAS connection
// identifier for 3GPP access (TS 33.501 §6.9.2.1).
const Bearer3GPP = nasBearer

// AccessType3GPP is the access-type distinguisher used in KgNB/KN3IWF derivation
// (TS 33.501 Annex A.9).
const AccessType3GPP uint8 = 0x01

func cipherByID(id uint8) (common.Cipher, error) {
	switch id {
	case AlgCiphering128NEA0:
		return common.NullCipher{}, nil
	case AlgCiphering128NEA1:
		return common.SNOW3GCipher{}, nil
	case AlgCiphering128NEA2:
		return common.AESCTRCipher{}, nil
	}

	return nil, fmt.Errorf("nas/fgs: unsupported NAS ciphering algorithm %#x", id)
}

func integrityByID(id uint8) (common.Integrity, error) {
	switch id {
	case AlgIntegrity128NIA0:
		return common.NullIntegrity{}, nil
	case AlgIntegrity128NIA1:
		return common.SNOW3GIntegrity{}, nil
	case AlgIntegrity128NIA2:
		return common.AESCMACIntegrity{}, nil
	}

	return nil, fmt.Errorf("nas/fgs: unsupported NAS integrity algorithm %#x", id)
}

// NASMacCalculate computes the 4-octet NAS-MAC over msg with algorithm algID
// (TS 33.501 §6.4.3, Annex D.3).
func NASMacCalculate(algID uint8, key [16]byte, count uint32, bearer, direction uint8, msg []byte) ([]byte, error) {
	integ, err := integrityByID(algID)
	if err != nil {
		return nil, err
	}

	mac, err := integ.MAC(key, count, bearer, direction, msg)
	if err != nil {
		return nil, err
	}

	return mac[:], nil
}

// NASEncrypt enciphers or deciphers data in place with algorithm algID; the
// operation is a keystream XOR, so the same call runs in both directions
// (TS 33.501 §6.4.4, Annex D.2).
func NASEncrypt(algID uint8, key [16]byte, count uint32, bearer, direction uint8, data []byte) error {
	ciph, err := cipherByID(algID)
	if err != nil {
		return err
	}

	out, err := ciph.Apply(key, count, bearer, direction, data)
	if err != nil {
		return err
	}

	copy(data, out)

	return nil
}
