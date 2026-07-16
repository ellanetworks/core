// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// Chokepoint accessors for the EPS NAS security/identity state. The secret keys
// (kasme, knasInt, knasEnc) are never returned; the operations that use them are
// methods so the keys stay inside the UeContext (TS 33.401).

// Supi returns the UE's SUPI (IMSI-derived), the key it is stored under.
func (ue *UeContext) Supi() etsi.SUPI { return ue.supi }

// Tmsi returns the UE's current M-TMSI (0 = none).
func (ue *UeContext) Tmsi() etsi.TMSI { return ue.tmsi }

// OldTmsi returns the M-TMSI being replaced during a GUTI reallocation (0 = none).
func (ue *UeContext) OldTmsi() etsi.TMSI { return ue.oldTmsi }

// IMSI returns the UE's IMSI, or "" when the identity is unset.
func (ue *UeContext) IMSI() string {
	if ue == nil {
		return ""
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.imsiOrEmpty()
}

// imsiOrEmpty returns the bare IMSI, or "" when the identity is unset; unlike
// etsi.SUPI.IMSI() it does not panic on an unset SUPI.
func (ue *UeContext) imsiOrEmpty() string {
	if !ue.supi.IsIMSI() {
		return ""
	}

	return ue.supi.IMSI()
}

// AmbrStrings returns the UE-AMBR uplink/downlink bit-rate strings, empty when the
// UE-AMBR has not been set.
func (ue *UeContext) AmbrStrings() (uplink, downlink string) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.Ambr == nil {
		return "", ""
	}

	return ue.Ambr.Uplink, ue.Ambr.Downlink
}

// HasKASME reports whether K_ASME is present (the UE has authenticated).
func (ue *UeContext) HasKASME() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.kasme) > 0
}

// SetKASME installs K_ASME derived from the EPS authentication vector (TS 33.401).
func (ue *UeContext) SetKASME(kasme []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.kasme = kasme
}

// EIA returns the selected NAS integrity algorithm.
func (ue *UeContext) EIA() byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.integrityAlg
}

// EEA returns the selected NAS ciphering algorithm.
func (ue *UeContext) EEA() byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.cipheringAlg
}

// ULCount returns the NAS COUNT the next uplink message must carry.
func (ue *UeContext) ULCount() uint32 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ulCount.NextExpected().Value()
}

// Secured reports whether the NAS security context is established.
func (ue *UeContext) Secured() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.secured
}

// AdvanceULCount records the expected uplink NAS COUNT as accepted. A SERVICE
// REQUEST is verified against that count by its short-MAC rather than by
// TryUnprotectUplink, so its acceptance is committed here (TS 24.301 §5.6.1).
func (ue *UeContext) AdvanceULCount() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ulCount.Commit(ue.ulCount.NextExpected())
}

// CommitUplinkCount records count as accepted, so a replay of its message
// estimates to a different count whose MAC fails to verify (TS 24.301 §4.4.3).
func (ue *UeContext) CommitUplinkCount(count uint32) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ulCount.Commit(nascommon.Count(count))
}

// TryUnprotectUplink verifies and deciphers a protected uplink NAS message
// against the UE's security context, returning the plain message and the full
// NAS COUNT it estimated. It does not mutate the UE, so a caller resolving a UE
// by S-TMSI can authenticate the message before binding the context. The keys
// never leave the kernel (TS 33.401).
func (ue *UeContext) TryUnprotectUplink(nas []byte) (plain []byte, count uint32, err error) {
	if len(nas) < 6 {
		return nil, 0, fmt.Errorf("nas message too short")
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	recvSeq := nas[5]

	count = ue.ulCount.Estimate(recvSeq).Value()

	p, err := eps.Unprotect(nas, count, nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, IntegrityAlg(ue.integrityAlg), CipherAlg(ue.cipheringAlg))
	if err != nil {
		return nil, 0, err
	}

	return p, count, nil
}

// ProtectDownlink reserves the next downlink NAS COUNT and integrity-protects
// (and ciphers, per the security header type) an already-marshalled NAS message
// with the UE's security context. The keys never leave the kernel (TS 24.301).
func (ue *UeContext) ProtectDownlink(plain []byte, sht eps.SecurityHeaderType) ([]byte, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	// Protect with the current NAS COUNT and advance only once the message is
	// protected, so a protection failure does not consume a downlink COUNT
	// (TS 24.301 §4.4.3.1).
	wire, err := eps.Protect(plain, sht, ue.dlCount.Value(),
		nascommon.DirectionDownlink, ue.knasInt, ue.knasEnc, IntegrityAlg(ue.integrityAlg), CipherAlg(ue.cipheringAlg))
	if err != nil {
		return nil, err
	}

	ue.dlCount = ue.dlCount.Next()

	return wire, nil
}

// InstallNASSecurityContext derives the NAS keys from K_ASME for the negotiated
// algorithms and installs the EPS NAS security context (TS 33.401). The
// AuthProof witnesses that EPS-AKA authentication has succeeded.
func (ue *UeContext) InstallNASSecurityContext(eea, eia byte, _ AuthProof) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	knasEnc, err := DeriveKNASEnc(ue.kasme, eea)
	if err != nil {
		return err
	}

	knasInt, err := DeriveKNASInt(ue.kasme, eia)
	if err != nil {
		return err
	}

	ue.cipheringAlg, ue.integrityAlg = eea, eia
	ue.knasEnc, ue.knasInt = knasEnc, knasInt

	// A new EPS security context starts both NAS COUNTs at zero, so the initial
	// SECURITY MODE COMMAND rides downlink COUNT 0 (TS 24.301 §4.4.3.1).
	ue.ulCount.Reset()
	ue.dlCount = 0

	return nil
}

// AllocateRegistrationArea assigns the UE's registered tracking area. Ella Core is a
// single registration area, so every UE is registered in the network's served TAIs.
func (ue *UeContext) AllocateRegistrationArea(servedTais []models.Tai) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.registrationArea = append(ue.registrationArea[:0:0], servedTais...)
}

// RegistrationArea returns a copy of the UE's registered tracking area.
func (ue *UeContext) RegistrationArea() []models.Tai {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return append([]models.Tai(nil), ue.registrationArea...)
}

// Eksi returns the eKSI assigned to the current EPS security context.
func (ue *UeContext) Eksi() uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.eksi
}

// SetEksi records the eKSI assigned to the current EPS security context.
func (ue *UeContext) SetEksi(v uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.eksi = v
}

// SetUESecurityCapability stores the UE and MS network capabilities. The AuthProof
// keeps every write on one audited path so a downgrade cannot enter (TS 24.301 §5.4.3.2).
func (ue *UeContext) SetUESecurityCapability(ueNetCap, msNetCap []byte, _ AuthProof) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ueNetCap = ueNetCap
	ue.msNetCap = msNetCap
}

// UeNetCap returns the stored raw UE network capability.
func (ue *UeContext) UeNetCap() []byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ueNetCap
}

// MsNetCap returns the stored raw MS network capability.
func (ue *UeContext) MsNetCap() []byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.msNetCap
}

// VerifyServiceRequestShortMAC recomputes the Service Request short-MAC over the
// supplied NAS header and compares it (and the truncated sequence number)
// against the values the UE sent (TS 24.301 §5.6.1). It returns the diagnostics
// for logging on failure; the keys never leave the kernel.
func (ue *UeContext) VerifyServiceRequestShortMAC(head []byte, gotMAC [2]byte, gotSeq uint8) (ok bool, want [2]byte, expSeq uint8, ul uint32) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	expected := ue.ulCount.NextExpected()

	// Only 5 of the 8 sequence number bits ride a SERVICE REQUEST, so the message
	// is bound to the expected count rather than to an estimate from the received
	// sequence number (TS 24.301 §4.4.3.1).
	ul = expected.Value()
	expSeq = expected.SQN() & 0x1f

	want, err := eps.ServiceRequestShortMAC(head, ue.knasInt, expected.Value(), nascommon.DirectionUplink, IntegrityAlg(ue.integrityAlg))
	if err != nil {
		return false, [2]byte{}, expSeq, ul
	}

	return want == gotMAC && expSeq == gotSeq, want, expSeq, ul
}

// DeriveInitialKeNB derives K_eNB from K_ASME and the last uplink NAS COUNT and
// seeds the X2-handover key chain (NH for NCC=1) for the first path switch
// (TS 33.401). It returns K_eNB for delivery to the eNB in the Initial Context
// Setup, plus the NAS COUNT it used (for diagnostics). K_ASME never leaves the
// kernel.
func (ue *UeContext) DeriveInitialKeNB() (kenb [32]byte, kenbCount uint32, err error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	// K_eNB is derived from the uplink NAS COUNT of the most recently accepted
	// uplink NAS message: the Security Mode Complete on attach, the Service
	// Request on reconnect (TS 33.401 §A.3).
	kenbCount = ue.ulCount.LastAccepted().Value()

	kenb, err = DeriveKeNB(ue.kasme, kenbCount)
	if err != nil {
		return [32]byte{}, kenbCount, err
	}

	nh, err := deriveNH(ue.kasme, kenb[:])
	if err != nil {
		return [32]byte{}, kenbCount, err
	}

	ue.nh = nh
	ue.ncc = 1

	return kenb, kenbCount, nil
}

// Conn returns the S1 association's writer (the eNB SCTP connection).
func (c *UeConn) Conn() S1APWriter { return c.conn }

// SetPDNEnbFTEID records the eNB S1-U endpoint on a PDN connection under the UE lock.
func (m *MME) SetPDNEnbFTEID(ue *UeContext, p *PdnConnection, f models.FTEID) {
	ue.mu.Lock()
	p.EnbFTEID = f
	ue.mu.Unlock()
}
