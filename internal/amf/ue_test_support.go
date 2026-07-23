// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
)

// AddUeContextToPoolForTest indexes a UE in the AMF's SUPI-keyed pool, as a completed
// registration would (CommitUEIdentity), for external test setup.
func (amf *AMF) AddUeContextToPoolForTest(ue *UeContext) error {
	if !ue.supi.IsValid() {
		return fmt.Errorf("supi is empty")
	}

	amf.mu.Lock()
	amf.UEs[ue.supi] = ue
	amf.mu.Unlock()

	return nil
}

// ForceStateForTest sets the UE state unconditionally, bypassing transition
// validation, for test precondition setup. Production code must use TransitionTo.
func (ue *UeContext) ForceStateForTest(s StateType) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.setStateLocked(s)
}

func (ue *UeContext) ForceRegStepForTest(step RegStep) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.state = RegistrationInitiated
	ue.regStep = step
}

// BindAMFForTest wires a test-constructed Radio to an AMF. UEs already registered
// on this radio migrate to a, so binding order does not matter in tests.
func (r *Radio) BindAMFForTest(a *AMF) {
	old := r.amf
	r.amf = a

	// Register the radio under its conn so radioFor(ueConn.conn) resolves it, as a
	// connected gNB is registered in prod.
	if r.Conn != nil {
		a.mu.Lock()
		a.radios[r.Conn] = r
		a.mu.Unlock()
	}

	if old == nil || old == a {
		return
	}

	old.mu.Lock()
	moved := make(map[int64]*UeConn)

	for id, ueConn := range old.conns {
		if ueConn.conn == r.Conn {
			moved[id] = ueConn
			delete(old.conns, id)
		}
	}

	old.mu.Unlock()

	a.mu.Lock()
	for id, ueConn := range moved {
		ueConn.amf = a
		a.conns[id] = ueConn
	}
	a.mu.Unlock()
}

func (r *Radio) NumUEsForTest() int {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	n := 0

	for _, ueConn := range r.amf.conns {
		if ueConn.conn == r.Conn {
			n++
		}
	}

	return n
}

func (a *AMF) SetHandoverGuardTimeoutForTest(d time.Duration) { a.handoverGuardTimeout = d }

func (ue *UeContext) ArmPagingForTest(d time.Duration, maxRetransmit int32) {
	ue.pagingTimer.Arm(d, maxRetransmit, func(int32) {}, func() {})
}

func (ue *UeContext) PagingActiveForTest() bool {
	return ue.pagingTimer.Active()
}

func (ue *UeContext) MobileReachableActiveForTest() bool {
	return ue.mobileReachableTimer.Active()
}

func (ue *UeContext) SetTmsiForTest(t etsi.TMSI)    { ue.tmsi = t }
func (ue *UeContext) SetOldTmsiForTest(t etsi.TMSI) { ue.oldTmsi = t }

func (ue *UeContext) SetSupiForTest(s etsi.SUPI) { ue.supi = s }
func (ue *UeContext) SupiForTest() etsi.SUPI     { return ue.supi }

// SetGutiForTest stores the GUTI's 5G-TMSI; the AMF keeps only the TMSI and
// rebuilds the GUTI from the GUAMI on demand.
func (ue *UeContext) SetGutiForTest(g etsi.GUTI5G) { ue.tmsi = g.Tmsi }
func (ue *UeContext) TmsiForTest() etsi.TMSI       { return ue.tmsi }

// TmsiInUseForTest reports whether the allocator still holds t.
func (a *AMF) TmsiInUseForTest(t etsi.TMSI) bool { return a.tmsi.IsAllocated(t) }

// AssignGutiForTest stores the GUTI's 5G-TMSI on ue and indexes it for resolution.
func (a *AMF) AssignGutiForTest(ue *UeContext, guti etsi.GUTI5G) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ue.tmsi = guti.Tmsi
	if guti != etsi.InvalidGUTI5G {
		a.uesByTmsi[guti.Tmsi] = ue
	}
}

func (ue *UeContext) SetSecuredForTest(b bool) { ue.secured = b }
func (ue *UeContext) SecuredForTest() bool     { return ue.secured }

func (ue *UeContext) SetIntegrityAlgForTest(a uint8) { ue.integrityAlg = a }
func (ue *UeContext) IntegrityAlgForTest() uint8     { return ue.integrityAlg }

func (ue *UeContext) SetCipheringAlgForTest(a uint8) { ue.cipheringAlg = a }
func (ue *UeContext) CipheringAlgForTest() uint8     { return ue.cipheringAlg }

func (ue *UeContext) SetKnasIntForTest(k [16]uint8) { ue.knasInt = k }
func (ue *UeContext) KnasIntForTest() [16]uint8     { return ue.knasInt }

func (ue *UeContext) SetKnasEncForTest(k [16]uint8) { ue.knasEnc = k }
func (ue *UeContext) KnasEncForTest() [16]uint8     { return ue.knasEnc }

func (ue *UeContext) SetNgKsiForTest(n models.NgKsi) { ue.ngKsi = n }
func (ue *UeContext) NgKsiForTest() models.NgKsi     { return ue.ngKsi }

func (ue *UeContext) SetKamfForTest(k string) { ue.kamf, _ = hex.DecodeString(k) }
func (ue *UeContext) KamfForTest() []uint8    { return ue.kamf }

func (ue *UeContext) SetNHForTest(nh []uint8) { copy(ue.nh[:], nh) }
func (ue *UeContext) NHForTest() [32]uint8    { return ue.nh }

func (ue *UeContext) SetUESecurityCapabilityForTest(c []byte) {
	ue.ueSecurityCapability = c
}

func (ue *UeContext) UESecurityCapabilityForTest() []byte {
	return ue.ueSecurityCapability
}

// UESecCapBytesForTest builds a UE security capability IE value (octet 1 = 5G-EA,
// octet 2 = 5G-IA) with the given 5G EA and IA algorithm numbers (0..7) enabled.
func UESecCapBytesForTest(ea, ia []uint8) []byte {
	var b [2]byte

	for _, n := range ea {
		b[0] |= 1 << (7 - n)
	}

	for _, n := range ia {
		b[1] |= 1 << (7 - n)
	}

	return b[:]
}

func (ue *UeContext) SetKgnbForTest(k []uint8) { ue.kgnb = k }
func (ue *UeContext) KgnbForTest() []uint8     { return ue.kgnb }

func (ue *UeContext) SetNCCForTest(n uint8) { ue.ncc = n }
func (ue *UeContext) NCCForTest() uint8     { return ue.ncc }

func (ue *UeContext) SetABBAForTest(a []uint8) { ue.abba = a }
func (ue *UeContext) ABBAForTest() []uint8     { return ue.abba }

// SetULCountForTest seeds the uplink counter to expect c as the NAS COUNT of the
// next uplink message.
func (ue *UeContext) SetULCountForTest(c uint32) {
	ue.ulCount.Reset()

	if c > 0 {
		ue.ulCount.Commit(nascommon.Count(c - 1))
	}
}

func (ue *UeContext) ULCountForTest() nascommon.UplinkCounter { return ue.ulCount }

func (ue *UeContext) SetDLCountForTest(c nascommon.Count) { ue.dlCount = c }
func (ue *UeContext) DLCountForTest() *nascommon.Count    { return &ue.dlCount }
