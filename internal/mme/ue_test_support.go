// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/mme/procedure"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/s1ap"
)

// Test-support accessors for the unexported EPS security/identity state. They let
// external test packages (mme/nas) construct and inspect a UE in a given security
// state without exporting the fields themselves; production code mutates this
// state only through the chokepoint methods.

func (ue *UeContext) SetKnasIntForTest(k [16]byte) { ue.knasInt = k }
func (ue *UeContext) SetKnasEncForTest(k [16]byte) { ue.knasEnc = k }

func (ue *UeContext) SetKASMEForTest(k []byte) { ue.kasme = k }

func (ue *UeContext) SetULCountForTest(c uint32) { ue.ulCount = nascommon.Count(c) }
func (ue *UeContext) SetDLCountForTest(c uint32) { ue.dlCount = nascommon.Count(c) }
func (ue *UeContext) DLCountForTest() uint32     { return ue.dlCount.Value() }

func (ue *UeContext) SetIntegrityAlgForTest(a byte) { ue.integrityAlg = a }
func (ue *UeContext) SetCipheringAlgForTest(a byte) { ue.cipheringAlg = a }

func (ue *UeContext) SetNHForTest(nh [32]byte) { ue.nh = nh }
func (ue *UeContext) NHForTest() [32]byte      { return ue.nh }
func (ue *UeContext) SetNCCForTest(n uint8)    { ue.ncc = n }
func (ue *UeContext) NCCForTest() uint8        { return ue.ncc }

func (ue *UeContext) SetTmsiForTest(m uint32) { ue.tmsi, _ = etsi.NewTMSI(m) }
func (ue *UeContext) TmsiForTest() uint32     { return ue.tmsi.Uint32() }

func (ue *UeContext) SetOldTmsiForTest(m uint32) { ue.oldTmsi, _ = etsi.NewTMSI(m) }
func (ue *UeContext) OldTmsiForTest() uint32     { return ue.oldTmsi.Uint32() }

func (ue *UeContext) SetSecuredForTest(v bool) { ue.secured = v }
func (ue *UeContext) SecuredForTest() bool     { return ue.secured }

func (c *UeConn) SetSecureExchangeEstablishedForTest(v bool) { c.secureExchangeEstablished = v }

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
	ue.cipheringAlg, ue.integrityAlg = eea, eia
	ue.knasEnc, ue.knasInt = ke, ki
	ue.secured = true

	return nil
}

// RegisterUEForTest indexes a UE under imsi in the persistent registry, as a
// completed attach would, for external test setup.
func (m *MME) RegisterUEForTest(ue *UeContext, imsi string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.supi.IsIMSI() && m.UEs[ue.supi] == ue {
		delete(m.UEs, ue.supi)
	}

	ue.supi, _ = etsi.NewSUPIFromIMSI(imsi)
	m.UEs[ue.supi] = ue
}

func (ue *UeContext) SetIMSIForTest(imsi string) { ue.supi, _ = etsi.NewSUPIFromIMSI(imsi) }

func (ue *UeContext) KASMEForTest() []byte { return ue.kasme }

func (c *UeConn) ConnForTest() S1APWriter { return c.conn }

func (c *UeConn) ReleasingForTest() bool { return c.releasing }

// NASGuardActiveForTest reports whether the UE's EMM common-procedure guard is armed.
func (ue *UeContext) NASGuardActiveForTest() bool {
	conn := ue.Conn()
	if conn == nil {
		return false
	}

	return conn.nasGuard.Active()
}

func (m *MME) ConnCountForTest() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.conns)
}

func (ue *UeContext) HasHandoverForTest() bool { return ue.handover != nil }

// ActiveProceduresForTest returns the types of the UE's in-flight key-changing
// procedures (the ongoing-procedures view).
func (ue *UeContext) ActiveProceduresForTest() []string {
	if ue.procedures == nil {
		return nil
	}

	return ue.procedures.ActiveTypes()
}

// SetKeyChainBusyForTest simulates a key-changing procedure holding the {NH, NCC}
// chain (v true, as a Path Switch) or releasing it (v false).
func (ue *UeContext) SetKeyChainBusyForTest(v bool) {
	if v {
		ue.BeginKeyChainProc(procedure.PathSwitch)
	} else {
		ue.clearKeyChainProc()
	}
}

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

func (m *MME) RegisterENBByIDForTest(g s1ap.GlobalENBID, conn S1APWriter) {
	m.mu.Lock()
	m.radiosByID[ENBID(g)] = &Radio{Conn: conn, id: ENBID(g)}
	m.mu.Unlock()
}

func (m *MME) SetHandoverGuardTimeoutForTest(d time.Duration) { m.handoverGuardTimeout = d }

func (m *MME) FireHandoverGuardForTest(ue *UeContext) { m.abandonHandover(ue) }

func (m *MME) ReclaimUEsOnConnLossForTest(conn S1APWriter) { m.reclaimUEsOnConnLoss(conn) }

func (ue *UeContext) MobileReachableArmedForTest() bool { return ue.mobileReachableTimer.Active() }

// ForceRegStepForTest sets the attach sub-phase directly, for tests that invoke a
// handler without driving the UE through the attach flow.
func (ue *UeContext) ForceRegStepForTest(step RegStep) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.regStep = step
}

// ForceStateForTest sets the EMM state directly, bypassing transition validation,
// for test precondition setup.
func (ue *UeContext) ForceStateForTest(s EMMState) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.emmState = s
}
