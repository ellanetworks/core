// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// Chokepoint accessors for the EPS NAS security/identity state. The secret keys
// (kasme, knasInt, knasEnc) are never returned; the operations that use them are
// methods so the keys stay inside the UeContext (TS 33.401).

// IMSI returns the UE's IMSI.
func (ue *UeContext) IMSI() string {
	if ue == nil {
		return ""
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.imsi
}

// HasKASME reports whether K_ASME is present (the UE has authenticated), without
// exposing the key.
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

	return ue.eia
}

// EEA returns the selected NAS ciphering algorithm.
func (ue *UeContext) EEA() byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.eea
}

// ULCount returns the expected uplink NAS COUNT.
func (ue *UeContext) ULCount() uint32 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.ulCount
}

// Secured reports whether the NAS security context is established.
func (ue *UeContext) Secured() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.secured
}

// AdvanceULCount increments the expected uplink NAS COUNT past an accepted
// message (TS 24.301).
func (ue *UeContext) AdvanceULCount() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ulCount++
}

// CommitUplinkCount advances the expected uplink NAS COUNT past the verified
// message, so a replay estimates to a stale count whose MAC fails to verify
// (TS 24.301).
func (ue *UeContext) CommitUplinkCount(count uint32) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ulCount = count + 1
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

	overflow := uint16(ue.ulCount >> 8)
	if recvSeq < uint8(ue.ulCount) {
		overflow++
	}

	count = nascommon.NASCount(overflow, recvSeq)

	p, err := eps.Unprotect(nas, count, nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, IntegrityAlg(ue.eia), CipherAlg(ue.eea))
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

	count := ue.dlCount
	ue.dlCount++

	return eps.Protect(plain, sht, nascommon.NASCount(0, uint8(count)),
		nascommon.DirectionDownlink, ue.knasInt, ue.knasEnc, IntegrityAlg(ue.eia), CipherAlg(ue.eea))
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

	ue.eea, ue.eia = eea, eia
	ue.knasEnc, ue.knasInt = knasEnc, knasInt

	return nil
}

// VerifyServiceRequestShortMAC recomputes the Service Request short-MAC over the
// supplied NAS header and compares it (and the truncated sequence number)
// against the values the UE sent (TS 24.301 §5.6.1). It returns the diagnostics
// for logging on failure; the keys never leave the kernel.
func (ue *UeContext) VerifyServiceRequestShortMAC(head []byte, gotMAC [2]byte, gotSeq uint8) (ok bool, want [2]byte, expSeq uint8, ul uint32) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ul = ue.ulCount
	expSeq = uint8(ue.ulCount) & 0x1f

	want, err := eps.ServiceRequestShortMAC(head, ue.knasInt, ue.ulCount, nascommon.DirectionUplink, IntegrityAlg(ue.eia))
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

	// K_eNB is derived from the uplink NAS COUNT of the most recently received
	// uplink NAS message (the Security Mode Complete on attach, the Service
	// Request on reconnect), i.e. one less than the next-expected count
	// (TS 33.401).
	kenbCount = ue.ulCount
	if kenbCount > 0 {
		kenbCount--
	}

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
func (c *S1Conn) Conn() NasWriter { return c.conn }

// SetPDNEnbFTEID records the eNB S1-U endpoint on a PDN connection under the UE lock.
func (m *MME) SetPDNEnbFTEID(ue *UeContext, p *PdnConnection, f models.FTEID) {
	ue.mu.Lock()
	p.EnbFTEID = f
	ue.mu.Unlock()
}
