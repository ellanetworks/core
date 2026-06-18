// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
)

// fcKASME is the EPS K_ASME key-derivation FC value (TS 33.401 §A.2).
const fcKASME = "10"

// EPSAV is an EPS-AKA authentication vector (TS 33.401 §6.1.1) — the HSS output
// the MME consumes.
type EPSAV struct {
	RAND  [16]byte
	AUTN  [16]byte
	XRES  []byte
	KASME []byte
}

// GenerateEPSVector creates an EPS-AKA vector for the subscriber, advancing and
// persisting the (shared) SQN. plmnID is the 3-octet serving-network identity
// used for K_ASME. For a re-synchronisation, pass the UE's AUTS and the RAND
// from the preceding challenge; otherwise pass empty strings.
func (s *Service) GenerateEPSVector(ctx context.Context, imsi string, plmnID []byte, resyncAuts, resyncRand string) (*EPSAV, error) {
	k, opc, sqn, err := s.advance(ctx, imsi, resyncAuts, resyncRand)
	if err != nil {
		return nil, err
	}

	amf, err := hex.DecodeString(authAMF)
	if err != nil {
		return nil, fmt.Errorf("amf decode error: %w", err)
	}

	var randArr [16]byte
	if _, err = rand.Read(randArr[:]); err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	ck, ik := make([]byte, 16), make([]byte, 16)
	res := make([]byte, 8)
	ak := make([]byte, 6)

	if err = F1(opc, k, randArr[:], sqn, amf, macA, macS); err != nil {
		return nil, fmt.Errorf("milenage F1: %w", err)
	}

	if err = F2345(opc, k, randArr[:], res, ck, ik, ak, nil); err != nil {
		return nil, fmt.Errorf("milenage F2345: %w", err)
	}

	sqnXorAK := make([]byte, 6)
	for i := range sqn {
		sqnXorAK[i] = sqn[i] ^ ak[i]
	}

	// AUTN = (SQN⊕AK) ‖ AMF ‖ MAC-A (TS 33.102 §6.3.2).
	var autn [16]byte

	copy(autn[0:6], sqnXorAK)
	copy(autn[6:8], amf)
	copy(autn[8:16], macA)

	kasme, err := deriveKASME(ck, ik, plmnID, sqnXorAK)
	if err != nil {
		return nil, err
	}

	return &EPSAV{RAND: randArr, AUTN: autn, XRES: res, KASME: kasme}, nil
}

// deriveKASME derives K_ASME (TS 33.401 §A.2) from CK‖IK, the serving-network
// PLMN identity (3 octets), and SQN⊕AK (6 octets).
func deriveKASME(ck, ik, plmnID, sqnXorAK []byte) ([]byte, error) {
	key := make([]byte, 0, len(ck)+len(ik))
	key = append(key, ck...)
	key = append(key, ik...)

	kasme, err := ueauth.GetKDFValue(key, fcKASME, plmnID, ueauth.KDFLen(plmnID), sqnXorAK, ueauth.KDFLen(sqnXorAK))
	if err != nil {
		return nil, fmt.Errorf("derive K_ASME: %w", err)
	}

	return kasme, nil
}
