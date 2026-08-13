// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 33.501 §8.3.2 step 4
func (ue *UeContext) InstallRelocatedSecurityContext(in interworking.EPSSecurityContext, _ AuthProof) error {
	sc, err := ue.installRelocatedKeys(in)
	if err != nil {
		ue.downlink().Clear()

		return err
	}

	ue.downlink().Install(sc, nas.NewDownlinkCounter(in.DLNASCount))

	return nil
}

func (ue *UeContext) installRelocatedKeys(in interworking.EPSSecurityContext) (*nas.SecurityContext, error) {
	knasEnc, err := epskeys.DeriveKNASEnc(in.KASME[:], in.Algorithms.Ciphering)
	if err != nil {
		return nil, fmt.Errorf("mme: derive K_NASenc: %w", err)
	}

	knasInt, err := epskeys.DeriveKNASInt(in.KASME[:], in.Algorithms.Integrity)
	if err != nil {
		return nil, fmt.Errorf("mme: derive K_NASint: %w", err)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.kasme = append([]byte(nil), in.KASME[:]...)
	ue.eksi = in.EKSI
	ue.cipheringAlg, ue.integrityAlg = in.Algorithms.Ciphering, in.Algorithms.Integrity
	ue.knasEnc, ue.knasInt = knasEnc, knasInt

	ue.ulCount = nas.NewUplinkCounter(in.ULNASCount)

	ue.nh, ue.ncc = in.NH, in.NCC

	ue.ueNetCap = relocatedNetworkCapability(in.UESecurityCapability)
	ue.ue5GSecurityCapability = in.UE5GSecurityCapability

	ue.secured = true

	sc, err := ue.installSecurityContextLocked()
	if err != nil {
		ue.secured = false

		return nil, fmt.Errorf("mme: install relocated EPS NAS security context: %w", err)
	}

	return sc, nil
}

func (ue *UeContext) BeginIdleMobilityFrom5GS() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.idleMobilityFrom5GS = true
}

func (ue *UeContext) EndIdleMobilityFrom5GS() {
	if ue == nil {
		return
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.idleMobilityFrom5GS = false
}

func (ue *UeContext) IdleMobilityFrom5GSPending() bool {
	if ue == nil {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.idleMobilityFrom5GS
}

var ErrNoEPSSecurityContext = errors.New("mme: UE has no current EPS NAS security context")

func (ue *UeContext) EPSSecurityContextForRelocation() (interworking.EPSSecurityContext, error) {
	dlCount := ue.downlink().Next()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured || len(ue.kasme) != len(interworking.EPSSecurityContext{}.KASME) {
		return interworking.EPSSecurityContext{}, ErrNoEPSSecurityContext
	}

	var kasme [32]byte

	copy(kasme[:], ue.kasme)

	return interworking.EPSSecurityContext{
		KASME:                  kasme,
		EKSI:                   ue.eksi,
		ULNASCount:             ue.ulCount.LastAccepted(),
		DLNASCount:             dlCount,
		Algorithms:             interworking.EPSNASAlgorithms{Ciphering: ue.cipheringAlg, Integrity: ue.integrityAlg},
		UESecurityCapability:   relocatedSecurityCapability(ue.ueNetCap),
		NH:                     ue.nh,
		NCC:                    ue.ncc,
		UE5GSecurityCapability: ue.ue5GSecurityCapability,
	}, nil
}

func relocatedSecurityCapability(c eps.UENetworkCapability) eps.UESecurityCapability {
	return eps.UESecurityCapability{
		EEA:     c.EEA,
		EIA:     c.EIA,
		HasUMTS: c.HasUMTS,
		UEA:     c.UEA,
		UIA:     c.UIA,
	}
}

func relocatedNetworkCapability(c eps.UESecurityCapability) eps.UENetworkCapability {
	return eps.UENetworkCapability{
		EEA:     c.EEA,
		EIA:     c.EIA,
		HasUMTS: c.HasUMTS,
		UEA:     c.UEA,
		UIA:     c.UIA,
	}
}
