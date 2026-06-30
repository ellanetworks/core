// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"time"

	"github.com/ellanetworks/core/s1ap"
)

// Test-support accessors for the unexported EPS security/identity state. They let
// external test packages (mme/nas) construct and inspect a UE in a given security
// state without exporting the fields themselves; production code mutates this
// state only through the chokepoint methods.

func (ue *UeContext) SetKnasIntForTest(k [16]byte) { ue.knasInt = k }
func (ue *UeContext) SetKnasEncForTest(k [16]byte) { ue.knasEnc = k }

func (ue *UeContext) SetKASMEForTest(k []byte) { ue.kasme = k }

func (ue *UeContext) SetULCountForTest(c uint32) { ue.ulCount = c }
func (ue *UeContext) SetDLCountForTest(c uint32) { ue.dlCount = c }
func (ue *UeContext) DLCountForTest() uint32     { return ue.dlCount }

func (ue *UeContext) SetEIAForTest(a byte) { ue.eia = a }
func (ue *UeContext) SetEEAForTest(a byte) { ue.eea = a }

func (ue *UeContext) SetNHForTest(nh [32]byte) { ue.nh = nh }
func (ue *UeContext) NHForTest() [32]byte      { return ue.nh }
func (ue *UeContext) SetNCCForTest(n uint8)    { ue.ncc = n }
func (ue *UeContext) NCCForTest() uint8        { return ue.ncc }

func (ue *UeContext) SetMtmsiForTest(m uint32) { ue.mtmsi = m }
func (ue *UeContext) MtmsiForTest() uint32     { return ue.mtmsi }

func (ue *UeContext) SetOldMTMSIForTest(m uint32) { ue.oldMTMSI = m }
func (ue *UeContext) OldMTMSIForTest() uint32     { return ue.oldMTMSI }

func (ue *UeContext) SetSecuredForTest(v bool) { ue.secured = v }
func (ue *UeContext) SecuredForTest() bool     { return ue.secured }

func (c *S1Conn) SetSecureExchangeEstablishedForTest(v bool) { c.secureExchangeEstablished = v }

func (ue *UeContext) KnasIntForTest() [16]byte { return ue.knasInt }
func (ue *UeContext) KnasEncForTest() [16]byte { return ue.knasEnc }

// SetSecurityContextForTest installs a NAS security context (deriving K_NASint/enc
// from kasme) and marks the UE secured, for external test setup.
func (ue *UeContext) SetSecurityContextForTest(kasme []byte, eea, eia byte) error {
	ke, err := DeriveKNASEnc(kasme, eea)
	if err != nil {
		return err
	}

	ki, err := DeriveKNASInt(kasme, eia)
	if err != nil {
		return err
	}

	ue.kasme = kasme
	ue.eea, ue.eia = eea, eia
	ue.knasEnc, ue.knasInt = ke, ki
	ue.secured = true

	return nil
}

// RegisterUEForTest indexes a UE under imsi in the persistent registry, as a
// completed attach would, for external test setup.
func (m *MME) RegisterUEForTest(ue *UeContext, imsi string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.imsi != "" && m.ues[ue.imsi] == ue {
		delete(m.ues, ue.imsi)
	}

	ue.imsi = imsi
	m.ues[imsi] = ue
}

func (ue *UeContext) SetIMSIForTest(imsi string) { ue.imsi = imsi }

func (ue *UeContext) KASMEForTest() []byte { return ue.kasme }

func (c *S1Conn) ConnForTest() NasWriter { return c.conn }

func (c *S1Conn) ReleasingForTest() bool { return c.releasing }

func (m *MME) ConnCountForTest() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.conns)
}

func (ue *UeContext) HasHandoverForTest() bool { return ue.handover != nil }

func (ue *UeContext) HandoverGenForTest() uint64 { return ue.handoverGen }

func (ue *UeContext) SetKeyChainBusyForTest(v bool) { ue.keyChainBusy = v }

// ForceHandoverCommittingForTest sets up the commit/cancel race.
func (ue *UeContext) ForceHandoverCommittingForTest() {
	if ue.handover != nil {
		ue.handover.state = hoCommitting
	}
}

// DeriveNextNHForTest computes the next {NH} from the UE's current kasme and NH,
// so a test can assert the committed key chain (TS 33.401 §7.2.8).
func (ue *UeContext) DeriveNextNHForTest() ([32]byte, error) {
	return deriveNH(ue.kasme, ue.nh[:])
}

func (m *MME) RegisterENBByIDForTest(g s1ap.GlobalENBID, conn NasWriter) {
	m.mu.Lock()
	m.enbByID[ENBID(g)] = conn
	m.mu.Unlock()
}

func (m *MME) SetHandoverGuardTimeoutForTest(d time.Duration) { m.handoverGuardTimeout = d }

func (m *MME) FireHandoverGuardForTest(ue *UeContext, gen uint64) { m.onHandoverGuardExpiry(ue, gen) }

func (m *MME) ReclaimUEsOnConnLossForTest(conn NasWriter) { m.reclaimUEsOnConnLoss(conn) }

func (ue *UeContext) MobileReachableArmedForTest() bool { return ue.mobileReachableTimer.Active() }
