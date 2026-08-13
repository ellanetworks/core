// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"fmt"

	"github.com/ellanetworks/core/internal/fivegskeys"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// MapToEPSOnIdleMobility maps the UE's 5G security context to the EPS one the
// MME adopts on an idle-mode inter-system change (TS 33.501 §8.6.1). K'ASME
// comes from K_AMF and the uplink 5G NAS COUNT of the verified TRACKING AREA
// UPDATE REQUEST (Annex A.14.1), and both EPS NAS COUNTs are the 5G ones.
//
// No NH is derived: nothing carries one to the UE on this path, and the MME
// derives K_eNB from K'ASME and the same uplink count if the active flag brings
// up S1-U (TS 33.401 §7.2.6.2).
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

// MapTo5GSOnIdleMobility maps an EPS security context to the 5G one the AMF
// adopts on an idle-mode inter-system change (TS 33.501 §8.6.2). K'AMF comes
// from K_ASME and the uplink EPS NAS COUNT of the TRACKING AREA UPDATE REQUEST
// enclosed in the REGISTRATION REQUEST (Annex A.15.1), and both 5G NAS COUNTs
// are 0.
//
// The selected 5G algorithms reach the UE in a NAS SMC rather than a transparent
// container (§8.2), so no container MAC is computed here and no K_gNB is
// derived: the security mode procedure settles the context, and the AN key
// follows from it.
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
