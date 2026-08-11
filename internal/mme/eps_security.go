// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 33.501 §8.3.2 step 4
func (ue *UeContext) InstallRelocatedSecurityContext(in interworking.EPSSecurityContext, _ AuthProof) error {
	knasEnc, err := epskeys.DeriveKNASEnc(in.KASME[:], in.Algorithms.Ciphering)
	if err != nil {
		return fmt.Errorf("mme: derive K_NASenc: %w", err)
	}

	knasInt, err := epskeys.DeriveKNASInt(in.KASME[:], in.Algorithms.Integrity)
	if err != nil {
		return fmt.Errorf("mme: derive K_NASint: %w", err)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.kasme = append([]byte(nil), in.KASME[:]...)
	ue.eksi = in.EKSI.Value
	ue.cipheringAlg, ue.integrityAlg = in.Algorithms.Ciphering, in.Algorithms.Integrity
	ue.knasEnc, ue.knasInt = knasEnc, knasInt

	ue.ulCount = nas.NewUplinkCounter(in.ULNASCount)
	ue.dlCount = nas.NewDownlinkCounter(in.DLNASCount)

	ue.nh, ue.ncc = in.NH, in.NCC

	ue.ueNetCap = relocatedNetworkCapability(in.UESecurityCapability)

	ue.secured = true

	if err := ue.installSecurityContextLocked(); err != nil {
		ue.secured = false

		return fmt.Errorf("mme: install relocated EPS NAS security context: %w", err)
	}

	return nil
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
