// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// securityContext builds the NAS security context for the algorithms and keys
// the UE derived, allowing null integrity: a test UE follows whatever the
// network selected, including NIA0.
func securityContext(nia, nea uint8, kNASint, kNASenc [16]byte) (*nas.SecurityContext, error) {
	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:          nas.IntegrityAlgorithm(nia),
		Ciphering:          nas.CipheringAlgorithm(nea),
		IntegrityKey:       kNASint,
		CipherKey:          kNASenc,
		AllowNullIntegrity: true,
	})
	if err != nil {
		return nil, fmt.Errorf("install NAS security context: %w", err)
	}

	return sc, nil
}

// 5G NAS security algorithm identifiers (TS 33.501 §5.11.1.2 / TS 24.501
// §9.11.3.34). Exported so scenarios can assert the algorithm the network selected.
const (
	AlgIntegrity128NIA0 uint8 = 0x00 // NULL
	AlgIntegrity128NIA1 uint8 = 0x01 // 128-SNOW3G
	AlgIntegrity128NIA2 uint8 = 0x02 // 128-AES
	AlgIntegrity128NIA3 uint8 = 0x03 // 128-ZUC

	AlgCiphering128NEA0 uint8 = 0x00 // NULL
	AlgCiphering128NEA1 uint8 = 0x01 // 128-SNOW3G
	AlgCiphering128NEA2 uint8 = 0x02 // 128-AES
	AlgCiphering128NEA3 uint8 = 0x03 // 128-ZUC
)

// Algorithm-type distinguishers for the NAS key derivation (TS 33.501 Annex A.8).
const (
	nnasEncAlg uint8 = 0x01
	nnasIntAlg uint8 = 0x02
)

// macInput is the integrity-protected span: the sequence-number octet followed by
// the (ciphered) NAS message payload (TS 24.501 §4.4.4.1).
func macInput(seq uint8, payload []byte) []byte {
	return append([]byte{seq}, payload...)
}
