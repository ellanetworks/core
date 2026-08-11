// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type EPSNASAlgorithms = interworking.EPSNASAlgorithms

func (ue *UeContext) EPSNetworkCapability() (eps.UENetworkCapability, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.epsNetworkCapabilityLocked()
}

func (ue *UeContext) epsNetworkCapabilityLocked() (eps.UENetworkCapability, bool) {
	if ue.s1UENetworkCapability == nil {
		return eps.UENetworkCapability{}, false
	}

	netCap, err := eps.ParseUENetworkCapability(ue.s1UENetworkCapability)
	if err != nil {
		return eps.UENetworkCapability{}, false
	}

	return netCap, true
}

func (ue *UeContext) EPSSecurityCapability() (eps.UESecurityCapability, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.epsSecurityCapabilityLocked()
}

func (ue *UeContext) epsSecurityCapabilityLocked() (eps.UESecurityCapability, bool) {
	netCap, ok := ue.epsNetworkCapabilityLocked()
	if !ok {
		return eps.UESecurityCapability{}, false
	}

	return eps.ReplayedUESecurityCapability(netCap, nil), true
}

func (ue *UeContext) NeedsEPSNASAlgorithms() bool {
	if !ue.SupportsS1Mode() {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.epsNASAlgorithms == nil
}

func (ue *UeContext) EPSNASAlgorithmsInUse() (EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithms == nil {
		return EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithms, true
}

func (ue *UeContext) offeredEPSNASAlgorithms() (EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithmsOffered, true
}

func (ue *UeContext) MarkEPSNASAlgorithmsDelivered() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return
	}

	delivered := *ue.epsNASAlgorithmsOffered
	ue.epsNASAlgorithms = &delivered
}

func (ue *UeContext) SelectEPSNASAlgorithms(intOrder []nas.IntegrityAlgorithm, encOrder []nas.CipheringAlgorithm) (EPSNASAlgorithms, bool) {
	netCap, ok := ue.EPSNetworkCapability()
	if !ok {
		return EPSNASAlgorithms{}, false
	}

	eea, eia, ok := eps.SelectNASAlgorithms(netCap, intOrder, encOrder)
	if !ok {
		return EPSNASAlgorithms{}, false
	}

	offered := EPSNASAlgorithms{Ciphering: eea, Integrity: eia}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.epsNASAlgorithmsOffered = &offered

	return offered, true
}

func (ue *UeContext) forgetEPSNASAlgorithmsLocked() {
	ue.epsNASAlgorithmsOffered = nil
	ue.epsNASAlgorithms = nil
}
