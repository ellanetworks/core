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
	"github.com/ellanetworks/core/nas/eps"
)

var (
	ErrNoEPSNASAlgorithms      = errors.New("amf: UE holds no EPS NAS algorithms for mobility to EPS")
	ErrNoEPSSecurityCapability = errors.New("amf: UE has no EPS security capability")
	ErrNo5GSecurityContext     = errors.New("amf: UE has no current 5G NAS security context")
)

func (ue *UeContext) EPSNetworkCapability() (eps.UENetworkCapability, bool) {
	raw := ue.S1UENetworkCapability()
	if raw == nil {
		return eps.UENetworkCapability{}, false
	}

	netCap, err := eps.ParseUENetworkCapability(raw)
	if err != nil {
		return eps.UENetworkCapability{}, false
	}

	return netCap, true
}

// EPSSecurityCapability is the UE's EPS security capability (TS 24.301
// §9.9.3.36), from the S1 UE network capability the UE sent or from the MME over
// N26.
func (ue *UeContext) EPSSecurityCapability() (eps.UESecurityCapability, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsSecurityCapability == nil {
		return eps.UESecurityCapability{}, false
	}

	return *ue.epsSecurityCapability, true
}

// setEPSSecurityCapabilityLocked stores capability and drops any EPS NAS
// algorithm pair chosen from a different one.
func (ue *UeContext) setEPSSecurityCapabilityLocked(capability eps.UESecurityCapability) {
	if ue.epsSecurityCapability != nil && *ue.epsSecurityCapability != capability {
		ue.forgetEPSNASAlgorithmsLocked()
	}

	ue.epsSecurityCapability = &capability
}

func (ue *UeContext) forgetEPSNASAlgorithmsLocked() {
	ue.epsNASAlgorithmsOffered = nil
	ue.epsNASAlgorithms = nil
}

// NeedsEPSNASAlgorithms reports whether the AMF still owes this UE the EPS NAS
// algorithms it is to use after mobility to EPS (TS 24.501 §5.4.2.2).
func (ue *UeContext) NeedsEPSNASAlgorithms() bool {
	if !ue.SupportsS1Mode() {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.epsNASAlgorithms == nil
}

// EPSNASAlgorithmsInUse returns the EPS NAS algorithms the UE holds, reporting
// false until it has accepted the SECURITY MODE COMMAND that carried them.
func (ue *UeContext) EPSNASAlgorithmsInUse() (interworking.EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithms == nil {
		return interworking.EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithms, true
}

func (ue *UeContext) offeredEPSNASAlgorithms() (interworking.EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return interworking.EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithmsOffered, true
}

// MarkEPSNASAlgorithmsDelivered promotes the offered pair to the one in use, on
// the UE accepting the SECURITY MODE COMMAND that carried it (TS 24.501
// §5.4.2.4).
func (ue *UeContext) MarkEPSNASAlgorithmsDelivered() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return
	}

	delivered := *ue.epsNASAlgorithmsOffered
	ue.epsNASAlgorithms = &delivered
}

// SelectEPSNASAlgorithms picks the EPS NAS algorithms this UE is to use after
// mobility to EPS and offers them, so the SECURITY MODE COMMAND about to be
// built carries them (TS 24.501 §5.4.2.2, TS 33.501 §6.7.2).
func (ue *UeContext) SelectEPSNASAlgorithms(intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (interworking.EPSNASAlgorithms, bool) {
	netCap, ok := ue.EPSNetworkCapability()
	if !ok {
		return interworking.EPSNASAlgorithms{}, false
	}

	eea, eia, ok := eps.SelectNASAlgorithms(netCap, intOrder, encOrder)
	if !ok {
		return interworking.EPSNASAlgorithms{}, false
	}

	offered := interworking.EPSNASAlgorithms{Ciphering: eea, Integrity: eia}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.epsNASAlgorithmsOffered = &offered

	return offered, true
}

// MapSecurityContextToEPS derives the mapped EPS security context for a handover
// to EPS from this UE's current 5G one (TS 33.501 §8.3.2 step 2).
//
// It consumes the downlink 5G NAS COUNT the derivation binds K'ASME to, so the
// same context can never be mapped twice onto one COUNT.
func (ue *UeContext) MapSecurityContextToEPS() (interworking.FiveGToEPSHandover, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured || ue.sc == nil {
		return interworking.FiveGToEPSHandover{}, ErrNo5GSecurityContext
	}

	if ue.epsNASAlgorithms == nil {
		return interworking.FiveGToEPSHandover{}, ErrNoEPSNASAlgorithms
	}

	if ue.epsSecurityCapability == nil {
		return interworking.FiveGToEPSHandover{}, ErrNoEPSSecurityCapability
	}

	dl, err := ue.dlCount.Use()
	if err != nil {
		return interworking.FiveGToEPSHandover{}, fmt.Errorf("amf: downlink NAS COUNT: %w", err)
	}

	return interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:                   ue.kamf,
		NgKSI:                  ngKsi(ue.ngKsi),
		ULNASCount:             ue.ulCount.LastAccepted(),
		DLNASCount:             dl,
		Algorithms:             *ue.epsNASAlgorithms,
		UESecurityCapability:   *ue.epsSecurityCapability,
		UE5GSecurityCapability: ue.ueSecurityCapability,
	})
}

// InstallMappedSecurityContextFromEPS takes a mapped 5G security context into use
// as this UE's current one, on a handover from EPS (TS 33.501 §8.4.2 step 3).
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

	ue.setEPSSecurityCapabilityLocked(mapped.EPSSecurityCapability)

	offered, delivered := mapped.EPSAlgorithms, mapped.EPSAlgorithms
	ue.epsNASAlgorithmsOffered, ue.epsNASAlgorithms = &offered, &delivered

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
