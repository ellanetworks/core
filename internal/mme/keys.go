// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
)

// EPS key-derivation FC values (TS 33.401), as hex strings for
// ueauth.GetKDFValue.
const (
	fcKeNB            = "11" // K_eNB
	fcNextHop         = "12" // NH (next hop), the X2-handover key chain
	fcEPSAlgorithmKey = "15" // NAS/RRC/UP algorithm keys
)

// NAS algorithm type distinguishers (TS 33.401).
const (
	nasEncAlgDistinguisher byte = 0x01
	nasIntAlgDistinguisher byte = 0x02
)

// deriveNASKey derives a 128-bit NAS key from K_ASME for the given algorithm
// type distinguisher and algorithm identity (TS 33.401); the key is the
// 128 least-significant bits of the KDF output.
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

// DeriveKNASEnc derives the NAS ciphering key for the given EEA algorithm id.
func DeriveKNASEnc(kasme []byte, algID byte) ([16]byte, error) {
	return deriveNASKey(kasme, nasEncAlgDistinguisher, algID)
}

// DeriveKNASInt derives the NAS integrity key for the given EIA algorithm id.
func DeriveKNASInt(kasme []byte, algID byte) ([16]byte, error) {
	return deriveNASKey(kasme, nasIntAlgDistinguisher, algID)
}

// DeriveKeNB derives K_eNB from K_ASME and the uplink NAS COUNT (TS 33.401).
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

// deriveNH derives a Next Hop value from K_ASME and a synchronisation input
// (TS 33.401): the initial K_eNB for the first NH (NCC=1), then the previous
// NH for each subsequent one. The MME advances this chain on every X2 handover
// and hands {NH, NCC} to the target eNB in the Path Switch Acknowledge so it can
// derive a fresh K_eNB with forward security (TS 33.401).
func deriveNH(kasme, syncInput []byte) ([32]byte, error) {
	var nh [32]byte

	out, err := ueauth.GetKDFValue(kasme, fcNextHop, syncInput, ueauth.KDFLen(syncInput))
	if err != nil {
		return nh, fmt.Errorf("derive NH: %w", err)
	}

	copy(nh[:], out)

	return nh, nil
}
