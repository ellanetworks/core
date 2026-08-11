// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package fivegskeys is the 5G key hierarchy of TS 33.501 Annex A: the
// derivations that hang off K_AMF.

package fivegskeys

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
)

const (
	nasEncAlgDistinguisher uint8 = 0x01
	nasIntAlgDistinguisher uint8 = 0x02
)

const accessType3GPP uint8 = 0x01

func deriveNASKey(kamf []byte, distinguisher, algID uint8) ([16]byte, error) {
	var k [16]byte

	out, err := ueauth.GetKDFValue(kamf, ueauth.FCForAlgorithmKeyDerivation,
		[]byte{distinguisher}, ueauth.KDFLen([]byte{distinguisher}),
		[]byte{algID}, ueauth.KDFLen([]byte{algID}))
	if err != nil {
		return k, fmt.Errorf("derive NAS key: %w", err)
	}

	copy(k[:], out[16:32])

	return k, nil
}

func DeriveKNASEnc(kamf []byte, alg nas.CipheringAlgorithm) (nas.CipherKey, error) {
	k, err := deriveNASKey(kamf, nasEncAlgDistinguisher, uint8(alg))

	return nas.CipherKey(k), err
}

func DeriveKNASInt(kamf []byte, alg nas.IntegrityAlgorithm) (nas.IntegrityKey, error) {
	k, err := deriveNASKey(kamf, nasIntAlgDistinguisher, uint8(alg))

	return nas.IntegrityKey(k), err
}

func DeriveKgNB(kamf []byte, ulNASCount uint32) ([32]byte, error) {
	var k [32]byte

	p0 := make([]byte, 4)
	binary.BigEndian.PutUint32(p0, ulNASCount)

	out, err := ueauth.GetKDFValue(kamf, ueauth.FCForKgnbKn3iwfDerivation,
		p0, ueauth.KDFLen(p0),
		[]byte{accessType3GPP}, ueauth.KDFLen([]byte{accessType3GPP}))
	if err != nil {
		return k, fmt.Errorf("derive K_gNB: %w", err)
	}

	if len(out) != len(k) {
		return k, fmt.Errorf("unexpected K_gNB length %d, want %d", len(out), len(k))
	}

	copy(k[:], out)

	return k, nil
}

func DeriveNH(kamf, syncInput []byte) ([32]byte, error) {
	var nh [32]byte

	out, err := ueauth.GetKDFValue(kamf, ueauth.FCForNhDerivation, syncInput, ueauth.KDFLen(syncInput))
	if err != nil {
		return nh, fmt.Errorf("derive NH: %w", err)
	}

	if len(out) != len(nh) {
		return nh, fmt.Errorf("unexpected NH length %d, want %d", len(out), len(nh))
	}

	copy(nh[:], out)

	return nh, nil
}
