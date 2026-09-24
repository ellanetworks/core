// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// SmContextRef is a snapshot of one PDU session's SM context reference, taken
// under the UE lock so callers can release or deactivate it without holding the
// lock.
type SmContextRef struct {
	Ref          string
	PduSessionID uint8
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
		refs = append(refs, SmContextRef{Ref: sc.Ref, PduSessionID: id})
	}

	return refs
}

func (ue *UeContext) Suci() string {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.suci
}

func (ue *UeContext) SetSUCI(suci *fgs.SUCI) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.suci = suci.String()
	if suci.Format == fgs.SUPIFormatIMSI {
		ue.plmnID = PlmnIDStringToModels(suci.PLMN.MCC + suci.PLMN.MNC)
	}
}

func (ue *UeContext) PlmnID() models.PlmnID {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.plmnID
}

func (ue *UeContext) Imei() etsi.IMEI {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.imei
}

func (ue *UeContext) SetImei(imei etsi.IMEI) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.imei = imei
}

func (ue *UeContext) AllowedNssai() []models.Snssai {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return append([]models.Snssai(nil), ue.allowedNssai...)
}

func (ue *UeContext) SetAllowedNssai(nssai []models.Snssai) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.allowedNssai = append([]models.Snssai(nil), nssai...)
}

func (ue *UeContext) RegistrationArea() []models.Tai {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return append([]models.Tai(nil), ue.registrationArea...)
}

func (ue *UeContext) RadioCapability() []byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.radioCapability
}

func (ue *UeContext) SetRadioCapability(capability []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.radioCapability = capability
}

func (ue *UeContext) SetAmbr(ambr *models.Ambr) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ambr = ambr
}

func (ue *UeContext) Ambr() *models.Ambr {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ambr
}

func (ue *UeContext) AmbrRates() (uplink, downlink models.BitRate, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.ambr == nil {
		return models.BitRate{}, models.BitRate{}, false
	}

	return ue.ambr.Uplink, ue.ambr.Downlink, true
}

func (ue *UeContext) RetainForEPS(d time.Duration) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.exportableToEPSUntil = time.Now().Add(d)
}

func (ue *UeContext) ExportableToEPS() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.state == Registered || time.Now().Before(ue.exportableToEPSUntil)
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

func (ue *UeContext) UESecCap() *fgs.UESecurityCapability {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ueSecurityCapability
}

func (ue *UeContext) StoredNgKsi() int32 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if len(ue.kamf) == 0 {
		return int32(nas.NoKeyAvailable)
	}

	return ue.ngKsi.Ksi
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

func (ue *UeContext) NEA() nas.CipheringAlgorithm {
	if ue == nil {
		return 0
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.cipheringAlg
}

func (ue *UeContext) NIA() nas.IntegrityAlgorithm {
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
	ue.supi = supi
	ue.mu.Unlock()

	ue.active.Load().bindSupi(supi)
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

// ULCount returns the NAS COUNT the next uplink message must carry.
func (ue *UeContext) ULCount() uint32 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ulCount.NextExpected().Value()
}

// DecryptUplinkContents deciphers an uplink NAS container in place against the
// UE's ciphering key. The container rides the message it arrived in, so it is
// ciphered with that message's NAS COUNT (TS 33.501).
func (ue *UeContext) DecryptUplinkContents(contents []byte) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out, err := ue.sc.Cipher(contents, ue.ulCount.LastAccepted(), nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		return err
	}

	copy(contents, out)

	return nil
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
