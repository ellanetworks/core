// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/free5gc/nas/nasType"
)

// Test-support accessors for the unexported NAS security/identity state. They
// let external test packages (amf_test, ngap_test) construct and inspect a UE in
// a given security state without exporting the fields themselves.

// BindAMFForTest wires a test-constructed Radio to an AMF so the UEs registered on
// it (via NewRanUeForTest / the handlers) live in the AMF's ranUEs index. Any UEs
// already registered on this radio (e.g. via NewRanUeForTest before the AMF was
// known) migrate to a, so binding order does not matter in tests.
func (r *Radio) BindAMFForTest(a *AMF) {
	old := r.amf
	r.amf = a

	if old == nil || old == a {
		return
	}

	old.mu.Lock()
	moved := make(map[int64]*RanUe)

	for id, ranUe := range old.ranUEs {
		if ranUe.radio == r {
			moved[id] = ranUe
			delete(old.ranUEs, id)
		}
	}

	old.mu.Unlock()

	a.mu.Lock()
	for id, ranUe := range moved {
		a.ranUEs[id] = ranUe
	}
	a.mu.Unlock()
}

// NumUEsForTest counts the UE-associated NGAP connections currently on this radio.
func (r *Radio) NumUEsForTest() int {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	n := 0

	for _, ranUe := range r.amf.ranUEs {
		if ranUe.radio == r {
			n++
		}
	}

	return n
}

// SetHandoverGuardTimeoutForTest overrides the N2 handover supervision timeout so
// tests can drive the guard quickly.
func (a *AMF) SetHandoverGuardTimeoutForTest(d time.Duration) { a.handoverGuardTimeout = d }

func (ue *UeContext) SetSupiForTest(s etsi.SUPI) { ue.supi = s }
func (ue *UeContext) SupiForTest() etsi.SUPI     { return ue.supi }

func (ue *UeContext) SetGutiForTest(g etsi.GUTI) { ue.guti = g }
func (ue *UeContext) GutiForTest() etsi.GUTI     { return ue.guti }

// AssignGutiForTest assigns guti to ue and indexes it for resolution, as
// ReAllocateGuti does in production.
func (a *AMF) AssignGutiForTest(ue *UeContext, guti etsi.GUTI) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ue.guti = guti
	if guti != etsi.InvalidGUTI {
		a.uesByGuti[guti] = ue
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

func (ue *UeContext) SetUESecurityCapabilityForTest(c *nasType.UESecurityCapability) {
	ue.ueSecurityCapability = c
}

func (ue *UeContext) UESecurityCapabilityForTest() *nasType.UESecurityCapability {
	return ue.ueSecurityCapability
}

func (ue *UeContext) SetKgnbForTest(k []uint8) { ue.kgnb = k }
func (ue *UeContext) KgnbForTest() []uint8     { return ue.kgnb }

func (ue *UeContext) SetNCCForTest(n uint8) { ue.ncc = n }
func (ue *UeContext) NCCForTest() uint8     { return ue.ncc }

func (ue *UeContext) SetABBAForTest(a []uint8) { ue.abba = a }
func (ue *UeContext) ABBAForTest() []uint8     { return ue.abba }

func (ue *UeContext) SetULCountForTest(c nascommon.Count) { ue.ulCount = c }
func (ue *UeContext) ULCountForTest() *nascommon.Count    { return &ue.ulCount }

func (ue *UeContext) SetDLCountForTest(c nascommon.Count) { ue.dlCount = c }
func (ue *UeContext) DLCountForTest() *nascommon.Count    { return &ue.dlCount }
