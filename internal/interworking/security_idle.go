// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"fmt"

	"github.com/ellanetworks/core/internal/fivegskeys"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func MapToEPSOnIdleMobility(in FiveGToEPSInput) (EPSSecurityContext, error) {
	if len(in.KAMF) != keyLen {
		return EPSSecurityContext{}, fmt.Errorf("interworking: K_AMF is %d octets, want %d", len(in.KAMF), keyLen)
	}

	kasme, err := deriveKey(in.KAMF, fcKASMEPrimeIdle, count32(in.ULNASCount.Value()))
	if err != nil {
		return EPSSecurityContext{}, fmt.Errorf("derive K'ASME: %w", err)
	}

	return EPSSecurityContext{
		KASME:                  kasme,
		EKSI:                   mappedKSI(in.NgKSI),
		ULNASCount:             in.ULNASCount,
		DLNASCount:             in.DLNASCount,
		Algorithms:             in.Algorithms,
		UESecurityCapability:   in.UESecurityCapability,
		UE5GSecurityCapability: in.UE5GSecurityCapability,
	}, nil
}

func MapTo5GSOnIdleMobility(in EPSSecurityContext, intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (Mapped5GSecurityContext, error) {
	kamf, err := deriveKey(in.KASME[:], fcKAMFPrimeIdle, count32(in.ULNASCount.Value()))
	if err != nil {
		return Mapped5GSecurityContext{}, fmt.Errorf("derive K'AMF: %w", err)
	}

	capability := mapped5GSecurityCapability(in)

	nea, nia, ok := fgs.SelectNASAlgorithms(capability, intOrder, encOrder)
	if !ok {
		return Mapped5GSecurityContext{}, fmt.Errorf("interworking: no 5G NAS algorithm common to the UE and the operator policy")
	}

	knasEnc, err := fivegskeys.DeriveKNASEnc(kamf[:], nea)
	if err != nil {
		return Mapped5GSecurityContext{}, fmt.Errorf("derive K_NASenc: %w", err)
	}

	knasInt, err := fivegskeys.DeriveKNASInt(kamf[:], nia)
	if err != nil {
		return Mapped5GSecurityContext{}, fmt.Errorf("derive K_NASint: %w", err)
	}

	return Mapped5GSecurityContext{
		KAMF:                  kamf,
		NgKSI:                 mappedKSI(in.EKSI),
		DLNASCount:            0,
		Ciphering:             nea,
		Integrity:             nia,
		KNASEnc:               knasEnc,
		KNASInt:               knasInt,
		UESecurityCapability:  capability,
		EPSAlgorithms:         in.Algorithms,
		EPSSecurityCapability: in.UESecurityCapability,
	}, nil
}
