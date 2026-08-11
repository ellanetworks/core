// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
)

var (
	ErrNoEPSNASAlgorithms      = errors.New("amf: UE holds no EPS NAS algorithms for mobility to EPS")
	ErrNoEPSSecurityCapability = errors.New("amf: UE has disclosed no EPS security capability")
)

// TS 33.501 §8.3.2 step 2
func (ue *UeContext) MapSecurityContextToEPS() (interworking.FiveGToEPSHandover, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured || ue.sc == nil {
		return interworking.FiveGToEPSHandover{}, errors.New("amf: no current 5G NAS security context to map")
	}

	if ue.epsNASAlgorithms == nil {
		return interworking.FiveGToEPSHandover{}, ErrNoEPSNASAlgorithms
	}

	capability, ok := ue.epsSecurityCapabilityLocked()
	if !ok {
		return interworking.FiveGToEPSHandover{}, ErrNoEPSSecurityCapability
	}

	dl, err := ue.dlCount.Use()
	if err != nil {
		return interworking.FiveGToEPSHandover{}, fmt.Errorf("amf: downlink NAS COUNT: %w", err)
	}

	return interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:                 ue.kamf,
		NgKSI:                ngKsi(ue.ngKsi),
		ULNASCount:           ue.ulCount.LastAccepted(),
		DLNASCount:           dl,
		Algorithms:           *ue.epsNASAlgorithms,
		UESecurityCapability: capability,
	})
}

// TS 33.501 §8.4.2 step 3
func (ue *UeContext) InstallMappedSecurityContextFromEPS(mapped interworking.Mapped5GSecurityContext, _ AuthProof) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.kamf = mapped.KAMF[:]
	ue.ngKsi = models.NgKsi{Ksi: int32(mapped.NgKSI.Value), Tsc: models.ScTypeMapped}
	ue.cipheringAlg, ue.integrityAlg = mapped.Ciphering, mapped.Integrity
	ue.knasEnc, ue.knasInt = mapped.KNASEnc, mapped.KNASInt

	ue.ulCount = nas.UplinkCounter{}
	ue.dlCount = nas.NewDownlinkCounter(mapped.DLNASCount)

	capability := mapped.UESecurityCapability
	ue.ueSecurityCapability = &capability

	epsAlgorithms := mapped.EPSAlgorithms
	ue.epsNASAlgorithmsOffered, ue.epsNASAlgorithms = &epsAlgorithms, &epsAlgorithms

	// {NCC=1, NH} is the whole of the AS key state the AMF keeps: the target gNB
	// derived its own K_gNB from the {NCC=0, NH} pair it was handed, by A.11 with
	// its cell parameters, so no K_gNB the AMF could name is the one in use
	// (TS 33.501 §8.4.2 steps 4 and 5). The temporary K_gNB is an output of the
	// procedure, not stored state.
	ue.kgnb = nil
	ue.nh = mapped.NH
	ue.ncc = mapped.NCC

	ue.secured = true

	if err := ue.installSecurityContextLocked(); err != nil {
		ue.secured = false

		return fmt.Errorf("amf: install mapped 5G NAS security context: %w", err)
	}

	return nil
}
