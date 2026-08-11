// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// EPSNASAlgorithms is the pair of EPS NAS algorithms the AMF selects for a UE to
// use after mobility to EPS over N26 (TS 33.501 §6.7.2). They are signalled to the
// UE in the Selected EPS NAS security algorithms IE of a SECURITY MODE COMMAND and
// later handed to the MME as part of the mapped EPS security context (§8.6.1).
type EPSNASAlgorithms struct {
	Ciphering nas.CipheringAlgorithm // EEA
	Integrity nas.IntegrityAlgorithm // EIA
}

// EPSNetworkCapability decodes the UE's stored S1 UE network capability
// (TS 24.501 §9.11.3.48, which is the TS 24.301 §9.9.3.34 element). It reports
// false when the UE has sent none — it is not a cleartext IE, so it only arrives
// once NAS security is up — or when the octets do not decode.
//
// This is the AMF's only source of EPS algorithm support. The 5GS UE security
// capability's octets 5 and 6 look similar but describe AS security over E-UTRA
// connected to 5GC (TS 24.501 §9.11.3.54), which says nothing about what the UE
// would accept in EPS.
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

// EPSSecurityCapability is the UE's EPS security capability
// (TS 24.301 §9.9.3.36), derived from the S1 UE network capability it sent. The
// AMF replays it beside the selected EPS NAS algorithms (TS 24.501 §8.2.25.8) and
// forwards it to the MME with the mapped EPS security context (TS 33.501 §8.6.1).
// There is no GERAN octet: 5GS carries no MS network capability to take one from.
func (ue *UeContext) EPSSecurityCapability() (eps.UESecurityCapability, bool) {
	netCap, ok := ue.EPSNetworkCapability()
	if !ok {
		return eps.UESecurityCapability{}, false
	}

	return eps.ReplayedUESecurityCapability(netCap, nil), true
}

// NeedsEPSNASAlgorithms reports whether the AMF still owes this UE the EPS NAS
// algorithms it is to use after mobility to EPS: the UE supports S1 mode and holds
// no pair yet (TS 24.501 §5.4.2.2). Whether the AMF can actually select one is a
// separate question, answered by SelectEPSNASAlgorithms.
func (ue *UeContext) NeedsEPSNASAlgorithms() bool {
	if !ue.SupportsS1Mode() {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.epsNASAlgorithms == nil
}

// EPSNASAlgorithmsInUse returns the EPS NAS algorithms the UE holds, reporting
// false until it has accepted the SECURITY MODE COMMAND that carried them. A pair
// the UE never received is one the two sides do not agree on, and a mapped EPS
// security context built from it would not be usable (TS 33.501 §8.6.1).
func (ue *UeContext) EPSNASAlgorithmsInUse() (EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithms == nil {
		return EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithms, true
}

// offeredEPSNASAlgorithms returns the pair the SECURITY MODE COMMAND being built,
// or in flight, carries.
func (ue *UeContext) offeredEPSNASAlgorithms() (EPSNASAlgorithms, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return EPSNASAlgorithms{}, false
	}

	return *ue.epsNASAlgorithmsOffered, true
}

// MarkEPSNASAlgorithmsDelivered promotes the offered pair to the one in use, on
// the UE accepting the SECURITY MODE COMMAND that carried it (TS 24.501 §5.4.2.4).
// A security mode procedure that offered nothing leaves the UE's pair alone: until
// a replacement is accepted, the UE still holds whatever it was last given.
func (ue *UeContext) MarkEPSNASAlgorithmsDelivered() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.epsNASAlgorithmsOffered == nil {
		return
	}

	delivered := *ue.epsNASAlgorithmsOffered
	ue.epsNASAlgorithms = &delivered
}

// SelectEPSNASAlgorithms picks the EPS NAS ciphering and integrity algorithms this
// UE is to use after mobility to EPS and offers them, so the SECURITY MODE COMMAND
// about to be built carries them (TS 24.501 §5.4.2.2, TS 33.501 §6.7.2). The
// operator's preference order is the same one the MME applies, so an inter-system
// handover needs no algorithm change in the target.
//
// It reports false — and offers nothing — when the UE's EPS capability is unknown
// or shares no algorithm with the operator policy. That only costs the UE N26
// mobility, so it is never a reason to fail the 5GS registration.
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

// forgetEPSNASAlgorithmsLocked drops both the offered and the delivered pair. The
// selection is a function of the UE's S1 UE network capability, so a UE that
// re-registers because that capability changed — which TS 24.501 §5.5.1.3.2 case g
// requires it to do — must be given a pair chosen from the new one. Caller holds
// ue.mu.
func (ue *UeContext) forgetEPSNASAlgorithmsLocked() {
	ue.epsNASAlgorithmsOffered = nil
	ue.epsNASAlgorithms = nil
}
