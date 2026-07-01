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

	ue.mu.Lock()
	defer ue.mu.Unlock()

	refs := make([]SmContextRef, 0, len(ue.SmContextList))
	for id, sc := range ue.SmContextList {
		refs = append(refs, SmContextRef{Ref: sc.Ref, PduSessionID: id, Inactive: sc.PduSessionInactive})
	}

	return refs
}

func (ue *UeContext) Secured() bool {
	if ue == nil {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.secured
}

func (ue *UeContext) Supi() etsi.SUPI {
	if ue == nil {
		return etsi.SUPI{}
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.supi
}

func (ue *UeContext) UESecCap() *nasType.UESecurityCapability {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ueSecurityCapability
}

func (ue *UeContext) NgKsi() models.NgKsi {
	if ue == nil {
		return models.NgKsi{}
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ngKsi
}

// Abba returns the UE's ABBA parameter (TS 33.501).
func (ue *UeContext) Abba() []uint8 {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.abba
}

func (ue *UeContext) CipheringAlg() uint8 {
	if ue == nil {
		return 0
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.cipheringAlg
}

func (ue *UeContext) IntegrityAlg() uint8 {
	if ue == nil {
		return 0
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.integrityAlg
}

// Kgnb returns the AS root key handed to the transport layer for the gNB.
func (ue *UeContext) Kgnb() []uint8 {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.kgnb
}

func (ue *UeContext) SetSupi(supi etsi.SUPI) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.supi = supi
}

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

func (ue *UeContext) ClearSecured() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.secured = false
}

func (ue *UeContext) MarkSecured() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.secured = true
}

// DecryptUplinkContents deciphers an uplink NAS container in place against the
// UE's ciphering key and current uplink count (TS 33.501).
func (ue *UeContext) DecryptUplinkContents(contents []byte) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return security.NASEncrypt(ue.cipheringAlg, ue.knasEnc, ue.ulCount.Value(), security.Bearer3GPP, security.DirectionUplink, contents)
}

// SmContextSnapshot returns a locked shallow copy of the UE's PDU session SM
// contexts for safe concurrent iteration.
func (ue *UeContext) SmContextSnapshot() map[uint8]*SmContext {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	snapshot := make(map[uint8]*SmContext, len(ue.SmContextList))
	for id, sc := range ue.SmContextList {
		snapshot[id] = sc
	}

	return snapshot
}
