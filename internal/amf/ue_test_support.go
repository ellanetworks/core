// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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

func (a *AMF) FireHandoverGuardForTest(ue *UeContext) bool { return a.abandonHandover(ue) }

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

func (ue *UeContext) SetSecuredForTest(b bool) {
	ue.secured = b

	// A secured UE has a NAS security context in production, so give one to a
	// test that marks it secured without running the security mode procedure.
	if b && ue.sc == nil {
		// The null algorithms, as a UE that has not run the security mode procedure
		// would have: the keys only have to be present, not secret.
		for i := range ue.knasInt {
			ue.knasInt[i], ue.knasEnc[i] = byte(i+1), byte(i+1)
		}

		ue.reinstallSecurityContextForTest()
	}
}
func (ue *UeContext) SecuredForTest() bool { return ue.secured }

// reinstallSecurityContextForTest rebuilds the NAS security context from the keys
// and algorithms currently held and hands it to the downlink sender, keeping the
// downlink NAS COUNT the sender has reached.
func (ue *UeContext) reinstallSecurityContextForTest() {
	if err := ue.installSecurityContextLocked(); err != nil {
		ue.dl.Clear()

		return
	}

	ue.dl.Install(ue.sc, nas.NewDownlinkCounter(ue.dl.Next()))
}

func (ue *UeContext) SetIntegrityAlgForTest(a nas.IntegrityAlgorithm) {
	ue.integrityAlg = a
	ue.reinstallSecurityContextForTest()
}
func (ue *UeContext) IntegrityAlgForTest() nas.IntegrityAlgorithm { return ue.integrityAlg }

func (ue *UeContext) SetCipheringAlgForTest(a nas.CipheringAlgorithm) {
	ue.cipheringAlg = a
	ue.reinstallSecurityContextForTest()
}
func (ue *UeContext) CipheringAlgForTest() nas.CipheringAlgorithm { return ue.cipheringAlg }

func (ue *UeContext) SetKnasIntForTest(k [16]uint8) {
	ue.knasInt = k
	ue.reinstallSecurityContextForTest()
}
func (ue *UeContext) KnasIntForTest() [16]uint8 { return ue.knasInt }

func (ue *UeContext) SetKnasEncForTest(k [16]uint8) {
	ue.knasEnc = k
	ue.reinstallSecurityContextForTest()
}
func (ue *UeContext) KnasEncForTest() [16]uint8 { return ue.knasEnc }

func (ue *UeContext) SetNgKsiForTest(n models.NgKsi) { ue.ngKsi = n }
func (ue *UeContext) NgKsiForTest() models.NgKsi     { return ue.ngKsi }

func (ue *UeContext) SetKamfForTest(k string) { ue.kamf, _ = hex.DecodeString(k) }
func (ue *UeContext) KamfForTest() []uint8    { return ue.kamf }

func (ue *UeContext) SetNHForTest(nh []uint8) { copy(ue.nh[:], nh) }
func (ue *UeContext) NHForTest() [32]uint8    { return ue.nh }

func (ue *UeContext) SetUESecurityCapabilityForTest(c *fgs.UESecurityCapability) {
	ue.ueSecurityCapability = c
}

func (ue *UeContext) UESecurityCapabilityForTest() *fgs.UESecurityCapability {
	return ue.ueSecurityCapability
}

// UESecCapForTest builds a UE security capability with the given 5G EA and IA
// algorithm numbers (0..7) enabled.
func UESecCapForTest(ea, ia []uint8) *fgs.UESecurityCapability {
	return &fgs.UESecurityCapability{
		EA: nas.Algorithms(ea...),
		IA: nas.Algorithms(ia...),
	}
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
		_ = ue.ulCount.Commit(nas.Count(c - 1))
	}
}

func (ue *UeContext) ULCountForTest() nas.UplinkCounter { return ue.ulCount }

func (ue *UeContext) SetDLCountForTest(c nas.Count) {
	ue.dl.Install(ue.sc, nas.NewDownlinkCounter(c))
}

func (ue *UeContext) DLCountForTest() nas.Count { return ue.dl.Next() }

func (ue *UeContext) NextDownlinkCountForTest() nas.Count { return ue.dl.Next() }

func (ue *UeContext) ForgetS1CapabilityForTest() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.s1UENetworkCapability = nil
	ue.epsSecurityCapability = nil
}
