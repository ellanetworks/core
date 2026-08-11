// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package fivegskeys is the 5G key hierarchy of TS 33.501 Annex A: the
// derivations that hang off K_AMF.
//
// It is shared rather than owned by the AMF because an inter-system handover
// from EPS has the mapped 5G security context built from a K_AMF' that no AMF
// procedure produced, and its NAS keys and AS key chain must come out identical
// to a native context's (TS 33.501 §8.4.2). Both callers derive through here, so
// there is one implementation.
//
// Nothing here reads an EPS parameter. The counterpart for the EPS hierarchy is
// internal/epskeys; the two stay apart because the specifications that define
// them do, and neither generation's derivation may be read off the other.
package fivegskeys

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
)

// Algorithm type distinguishers P0 for NAS key derivation with FC=0x69
// (TS 33.501 Annex A.8, table A.8-1).
const (
	nasEncAlgDistinguisher uint8 = 0x01
	nasIntAlgDistinguisher uint8 = 0x02
)

// accessType3GPP is the access type distinguisher for 3GPP access in the
// K_gNB derivation (TS 33.501 Annex A.9, table A.9-1).
const accessType3GPP uint8 = 0x01

// deriveNASKey derives a 128-bit NAS key from K_AMF for the given algorithm type
// distinguisher and algorithm identity (TS 33.501 Annex A.8); the key is the 128
// least-significant bits of the KDF output.
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

// DeriveKNASEnc derives the NAS ciphering key for the given NEA algorithm id.
func DeriveKNASEnc(kamf []byte, alg nas.CipheringAlgorithm) (nas.CipherKey, error) {
	k, err := deriveNASKey(kamf, nasEncAlgDistinguisher, uint8(alg))

	return nas.CipherKey(k), err
}

// DeriveKNASInt derives the NAS integrity key for the given NIA algorithm id.
func DeriveKNASInt(kamf []byte, alg nas.IntegrityAlgorithm) (nas.IntegrityKey, error) {
	k, err := deriveNASKey(kamf, nasIntAlgDistinguisher, uint8(alg))

	return nas.IntegrityKey(k), err
}

// DeriveKgNB derives K_gNB from K_AMF and the uplink NAS COUNT for 3GPP access
// (TS 33.501 Annex A.9).
//
// ulNASCount is a full 32-bit value rather than a nas.Count because a mapped 5G
// security context derives its temporary K_gNB at 2³²−1 — deliberately outside
// the 24-bit NAS COUNT range, so the value can never collide with one a real
// message used (TS 33.501 §8.4.2 NOTE 3).
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

// DeriveNH derives a Next Hop value from K_AMF and a synchronisation input
// (TS 33.501 Annex A.10): the newly derived K_gNB for the first NH, then the
// previous NH for each subsequent one.
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
