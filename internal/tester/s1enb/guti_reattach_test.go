// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func securedTestUE(t *testing.T) *UE {
	t.Helper()

	ue := &UE{IMSI: "001010000000001", netCapEEA: 0xf0, netCapEIA: 0x70, kasme: make([]byte, 32), eea: 2, eia: 2, eksi: 3, ulCount: 7}
	for i := range ue.kasme {
		ue.kasme[i] = byte(i + 1)
	}

	if err := ue.deriveNASKeys(); err != nil {
		t.Fatalf("derive NAS keys: %v", err)
	}

	return ue
}

func testGUTI() *eps.EPSMobileIdentity {
	return new(eps.GUTIIdentity(eps.GUTI{
		PLMN: nas.PLMN{MCC: "999", MNC: "70"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{1, 2, 3, 4},
	}))
}

func TestProtectedGUTIAttachRequest(t *testing.T) {
	ue := securedTestUE(t)
	ue.DeferESMInformation()

	wire, err := ue.buildProtectedGUTIAttachRequest(testGUTI())
	if err != nil {
		t.Fatal(err)
	}

	if sht, err := eps.PeekSecurityHeaderType(wire); err != nil || sht != eps.SHTIntegrityProtected {
		t.Fatalf("security header type = %v (%v), want integrity protected", sht, err)
	}

	if ue.ulCount != 8 {
		t.Fatalf("uplink NAS COUNT = %d after the attach, want 8", ue.ulCount)
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, 7), nas.DirectionUplink, ue.sc))
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}

	req, err := eps.ParseAttachRequest(plain)
	if err != nil {
		t.Fatalf("parse attach request: %v", err)
	}

	if req.EPSMobileIdentity.GUTI == nil || req.EPSMobileIdentity.GUTI.TMSI != [4]byte{1, 2, 3, 4} {
		t.Fatalf("identity = %+v, want the GUTI", req.EPSMobileIdentity)
	}

	if req.NASKeySetIdentifier.Value != 3 || req.NASKeySetIdentifier.Mapped {
		t.Fatalf("KSI = %+v, want native 3", req.NASKeySetIdentifier)
	}

	pc, err := eps.ParsePDNConnectivityRequest(req.ESMMessageContainer)
	if err != nil {
		t.Fatalf("parse PDN connectivity request: %v", err)
	}

	if pc.ESMInformationTransferFlag == nil || !*pc.ESMInformationTransferFlag {
		t.Fatalf("ESM information transfer flag not set")
	}
}

func TestProtectedGUTIAttachRequestNeedsASecurityContext(t *testing.T) {
	ue := &UE{IMSI: "001010000000001"}

	if _, err := ue.buildProtectedGUTIAttachRequest(testGUTI()); err == nil {
		t.Fatal("built a GUTI re-attach with no security context")
	}
}

func TestSwitchOffDetachIdentifiedByGUTI(t *testing.T) {
	ue := securedTestUE(t)
	ue.DetachWithGUTI(testGUTI())

	wire, err := ue.buildDetachRequestWith(true)
	if err != nil {
		t.Fatal(err)
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, 7), nas.DirectionUplink, ue.sc))
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}

	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		t.Fatalf("parse detach request: %v", err)
	}

	if !req.SwitchOff || req.EPSMobileIdentity.GUTI == nil || req.NASKeySetIdentifier.Value != 3 {
		t.Fatalf("detach request %+v, want a switch-off identified by the GUTI with KSI 3", req)
	}
}
