// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
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

	ue.mu.RLock()
	defer ue.mu.RUnlock()

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

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.nh, ue.ncc
}

// HasSecurityContext reports whether a 5G NAS security context is available.
func (ue *UeContext) HasSecurityContext() bool {
	if ue == nil {
		return false
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.securityContextAvailable
}

// SupiValue returns the UE's SUPI.
func (ue *UeContext) SupiValue() etsi.SUPI {
	if ue == nil {
		return etsi.SUPI{}
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.supi
}

// UESecCap returns the UE's 5G security capabilities.
func (ue *UeContext) UESecCap() *nasType.UESecurityCapability {
	if ue == nil {
		return nil
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.ueSecurityCapability
}

// Guti returns the UE's GUTI.
func (ue *UeContext) Guti() etsi.GUTI {
	if ue == nil {
		return etsi.GUTI{}
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.guti
}

// NgKsi returns the UE's NAS key set identifier.
func (ue *UeContext) NgKsi() models.NgKsi {
	if ue == nil {
		return models.NgKsi{}
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.ngKsi
}

// Abba returns the UE's ABBA parameter (TS 33.501).
func (ue *UeContext) Abba() []uint8 {
	if ue == nil {
		return nil
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.abba
}

// CipheringAlg returns the selected NAS ciphering algorithm.
func (ue *UeContext) CipheringAlg() uint8 {
	if ue == nil {
		return 0
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.cipheringAlg
}

// IntegrityAlg returns the selected NAS integrity algorithm.
func (ue *UeContext) IntegrityAlg() uint8 {
	if ue == nil {
		return 0
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.integrityAlg
}

// Kgnb returns the AS root key handed to the transport layer for the gNB.
func (ue *UeContext) Kgnb() []uint8 {
	if ue == nil {
		return nil
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.kgnb
}

// SetSupi records the UE's SUPI.
func (ue *UeContext) SetSupi(supi etsi.SUPI) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.supi = supi
}

// SetNgKsi records the UE's NAS key set identifier.
func (ue *UeContext) SetNgKsi(ngKsi models.NgKsi) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ngKsi = ngKsi
}

// SetAbba records the UE's ABBA parameter (TS 33.501).
func (ue *UeContext) SetAbba(abba []uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.abba = abba
}

// ClearSecurityContext marks the 5G NAS security context as unavailable.
func (ue *UeContext) ClearSecurityContext() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.securityContextAvailable = false
}

// MarkSecurityContextAvailable marks the 5G NAS security context as available.
func (ue *UeContext) MarkSecurityContextAvailable() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.securityContextAvailable = true
}

// DecryptUplinkContents deciphers an uplink NAS container in place against the
// UE's ciphering key and current uplink count (TS 33.501).
func (ue *UeContext) DecryptUplinkContents(contents []byte) error {
	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return security.NASEncrypt(ue.cipheringAlg, ue.knasEnc, ue.ulCount.Get(), security.Bearer3GPP, security.DirectionUplink, contents)
}

// SmContextSnapshot returns a locked shallow copy of the UE's PDU session SM
// contexts for safe concurrent iteration.
func (ue *UeContext) SmContextSnapshot() map[uint8]*SmContext {
	ue.mu.RLock()
	defer ue.mu.RUnlock()

	snapshot := make(map[uint8]*SmContext, len(ue.SmContextList))
	for id, sc := range ue.SmContextList {
		snapshot[id] = sc
	}

	return snapshot
}
