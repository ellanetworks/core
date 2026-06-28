// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/free5gc/nas/nasType"
)

// SmContextRef is a snapshot of one PDU session's SM context reference, taken
// under the UE lock so callers can release or deactivate it without holding the
// lock.
type SmContextRef struct {
	Ref          string
	PduSessionID uint8
	Inactive     bool
}

// SmContextRefs returns a locked snapshot of the UE's PDU session SM context
// references.
func (ue *UeContext) SmContextRefs() []SmContextRef {
	if ue == nil {
		return nil
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	refs := make([]SmContextRef, 0, len(ue.SmContextList))
	for id, sc := range ue.SmContextList {
		refs = append(refs, SmContextRef{Ref: sc.Ref, PduSessionID: id, Inactive: sc.PduSessionInactive})
	}

	return refs
}

// NextHopNCC returns the AS security next hop and its chaining count for the
// transport layer to derive the target gNB key at handover/path switch
// (TS 33.501).
func (ue *UeContext) NextHopNCC() ([]uint8, uint8) {
	if ue == nil {
		return nil, 0
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	return ue.NH, ue.NCC
}

// HasSecurityContext reports whether a 5G NAS security context is available.
func (ue *UeContext) HasSecurityContext() bool {
	if ue == nil {
		return false
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	return ue.SecurityContextAvailable
}

// SupiValue returns the UE's SUPI.
func (ue *UeContext) SupiValue() etsi.SUPI {
	if ue == nil {
		return etsi.SUPI{}
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	return ue.supi
}

// UESecCap returns the UE's 5G security capabilities.
func (ue *UeContext) UESecCap() *nasType.UESecurityCapability {
	if ue == nil {
		return nil
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	return ue.ueSecurityCapability
}
