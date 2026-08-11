// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package epskeys is the EPS key hierarchy of TS 33.401 Annex A: the
// derivations that hang off K_ASME.
package epskeys

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
)

const (
	fcKeNB            = "11" // A.3
	fcNextHop         = "12" // A.4: NH, the handover key chain
	fcEPSAlgorithmKey = "15" // A.7: NAS/RRC/UP algorithm keys
)

const (
	nasEncAlgDistinguisher byte = 0x01
	nasIntAlgDistinguisher byte = 0x02
)

func deriveNASKey(kasme []byte, distinguisher, algID byte) ([16]byte, error) {
	var k [16]byte

	out, err := ueauth.GetKDFValue(kasme, fcEPSAlgorithmKey,
		[]byte{distinguisher}, ueauth.KDFLen([]byte{distinguisher}),
		[]byte{algID}, ueauth.KDFLen([]byte{algID}))
	if err != nil {
		return k, fmt.Errorf("derive NAS key: %w", err)
	}

	copy(k[:], out[16:32])

	return k, nil
}

func DeriveKNASEnc(kasme []byte, alg nas.CipheringAlgorithm) ([16]byte, error) {
	return deriveNASKey(kasme, nasEncAlgDistinguisher, uint8(alg))
}

// DeriveKNASInt derives the NAS integrity key for the given EIA algorithm id.
func DeriveKNASInt(kasme []byte, alg nas.IntegrityAlgorithm) ([16]byte, error) {
	return deriveNASKey(kasme, nasIntAlgDistinguisher, uint8(alg))
}

func DeriveKeNB(kasme []byte, ulNASCount uint32) ([32]byte, error) {
	var k [32]byte

	p0 := make([]byte, 4)
	binary.BigEndian.PutUint32(p0, ulNASCount)

	out, err := ueauth.GetKDFValue(kasme, fcKeNB, p0, ueauth.KDFLen(p0))
	if err != nil {
		return k, fmt.Errorf("derive K_eNB: %w", err)
	}

	copy(k[:], out)

	return k, nil
}

func DeriveNH(kasme, syncInput []byte) ([32]byte, error) {
	var nh [32]byte

	out, err := ueauth.GetKDFValue(kasme, fcNextHop, syncInput, ueauth.KDFLen(syncInput))
	if err != nil {
		return nh, fmt.Errorf("derive NH: %w", err)
	}

	copy(nh[:], out)

	return nh, nil
}
