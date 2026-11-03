// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
)

type n26Policy bool

func (p n26Policy) Allowed(context.Context, string) (bool, error) { return bool(p), nil }

func (f *roamingMME) arrivalFrom5GS(t *testing.T) (*fakeFiveGSPeer, *mme.UeConn, *captureConn) {
	t.Helper()

	f.store.mu.Lock()
	f.store.visitors[roamerIMSI] = roamerPartnerID
	f.store.mu.Unlock()

	resp := arrivingEPSContext(t, interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES})

	supi, err := etsi.NewSUPIFromIMSI(roamerIMSI)
	if err != nil {
		t.Fatal(err)
	}

	resp.SUPI = supi

	peer := &fakeFiveGSPeer{Response: resp}
	f.m.FiveGS = peer
	f.m.N26 = n26Policy(true)

	cc := &captureConn{}
	conn := f.m.NewUeConn(cc, 7)
	conn.ServingTAI = servedAttachTAI

	return peer, conn, cc
}

func updateLocationRequests(t *testing.T, hss *fakehss.Node) []s6a.UpdateLocationRequest {
	t.Helper()

	var out []s6a.UpdateLocationRequest

	for _, m := range s6aRequests(t, hss, s6a.CommandUpdateLocation) {
		ulr, err := s6a.ParseUpdateLocationRequest(m)
		if err != nil {
			t.Fatalf("parse ULR: %v", err)
		}

		out = append(out, ulr)
	}

	return out
}

func lastTAUDownlink(t *testing.T, ue *mme.UeContext, cc *captureConn) []byte {
	t.Helper()

	cc.mu.Lock()
	sent := slices.Clone(cc.sent)
	cc.mu.Unlock()

	var wire []byte

	for i := len(sent) - 1; i >= 0 && wire == nil; i-- {
		msg, err := s1ap.Unmarshal(sent[i])
		if im, ok := msg.(*s1ap.InitiatingMessage); err == nil && ok && im.ProcedureCode == s1ap.ProcDownlinkNASTransport {
			wire = decodeDownlinkNAS(t, sent[i])
		}
	}

	if wire == nil {
		t.Fatal("no NAS message was sent to the UE")
	}

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect the downlink: %v", err)
	}

	return plain
}

func TestVisitorArrivingFrom5GSInIdleModeRegistersAtItsHomeHSS(t *testing.T) {
	f := newRoamingMME(t)
	peer, conn, _ := f.arrivalFrom5GS(t)

	release := f.hss.Hold(s6a.CommandUpdateLocation)
	HandleNAS(context.Background(), f.m, conn, interSystemTAU(t, nil))

	deadline := time.Now().Add(2 * time.Second)
	for len(s6aRequests(t, f.hss, s6a.CommandUpdateLocation)) == 0 {
		if time.Now().After(deadline) {
			release()
			t.Fatal("no Update Location was sent for the visitor that moved from 5GS")
		}

		time.Sleep(time.Millisecond)
	}

	if !peer.Acked {
		t.Error("the Update Location went out before the AMF was acknowledged (TS 23.401 §5.3.3.1 step 7 precedes step 12)")
	}

	release()
	f.settle(conn)

	ulrs := updateLocationRequests(t, f.hss)
	if len(ulrs) != 1 {
		t.Fatalf("ULRs = %d, want 1", len(ulrs))
	}

	if ulrs[0].Flags != s6a.ULRFlagS6aS6dIndicator {
		t.Errorf("ULR-Flags = %v, want S6a/S6d-Indicator alone: no Initial-Attach-Indicator, and no Dual-Registration-5G-Indicator so the HSS+UDM cancels the AMF", ulrs[0].Flags)
	}

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	v, ok := ue.Visitor()
	if !ok || !v.Registered || v.Subscription == nil {
		t.Fatalf("visitor = %+v, %v, want registered at its home HSS with the subscription cached", v, ok)
	}

	if len(conn.TauAcceptPlain) == 0 {
		t.Fatal("the update was not accepted")
	}

	if ue.EMMState() != mme.EMMRegistered {
		t.Errorf("EMM state = %v, want EMM-REGISTERED", ue.EMMState())
	}

	if p := f.m.LookupPDN(ue, 6); p == nil {
		t.Error("the session that moved from 5GS was not kept as a PDN connection")
	}

	if partner, ok := f.store.visitor(roamerIMSI); !ok || partner != roamerPartnerID {
		t.Error("the MME registration of the visitor was not recorded")
	}
}

func TestVisitorArrivingFrom5GSIsRejectedWhenItsHomeHSSRefuses(t *testing.T) {
	f := newRoamingMME(t)
	f.hss.FailULR(&s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)})
	_, conn, cc := f.arrivalFrom5GS(t)

	f.handle(conn, interSystemTAU(t, nil))

	ue := conn.UeContext()
	if ue == nil {
		t.Fatal("no context was bound")
	}

	if len(conn.TauAcceptPlain) != 0 {
		t.Fatal("the update was accepted although the home HSS refused the visitor")
	}

	reject, err := eps.ParseTrackingAreaUpdateReject(lastTAUDownlink(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Tracking Area Update Reject: %v", err)
	}

	if reject.Cause != eps.EMMCauseEPSAndNonEPSServicesNotAllowed {
		t.Errorf("cause = %v, want #8 for an unknown user (TS 29.272 Annex A)", reject.Cause)
	}

	if ue.EMMState() != mme.EMMDeregistered {
		t.Errorf("EMM state = %v, want EMM-DEREGISTERED", ue.EMMState())
	}

	if len(f.sessions.releaseCauses) == 0 {
		t.Error("the PDN connections of the refused visitor were kept")
	}
}

func TestVisitorOfAPartnerWithoutBothAgreementsGetsNoContextFrom5GS(t *testing.T) {
	f := newRoamingMME(t)
	peer, conn, _ := f.arrivalFrom5GS(t)
	peer.Err = interworking.ErrN26NotAllowed

	f.handle(conn, interSystemTAU(t, nil))

	if conn.UeContext() != nil {
		t.Fatal("a context was bound although the AMF handed none out")
	}

	if len(s6aRequests(t, f.hss, s6a.CommandUpdateLocation)) != 0 {
		t.Error("an Update Location was sent for a visitor the AMF did not hand over")
	}
}

func TestTAUAfterAHandoverFrom5GSRegistersTheVisitorAtItsHomeHSS(t *testing.T) {
	f := newRoamingMME(t)
	f.m.N26 = n26Policy(true)
	a := f.registeredRoamer(t)

	v, _ := a.ue.Visitor()
	v.Registered = false
	v.Subscription = nil
	a.ue.SetVisitorForTest(v)
	a.ue.MarkArrivedByHandoverFrom5GSForTest()

	f.handle(a.ue.Conn(), trackingAreaUpdateNAS(t, a.ue, nil))

	ulrs := updateLocationRequests(t, f.hss)
	if len(ulrs) != 2 {
		t.Fatalf("ULRs = %d, want the attach one and one for the TAU after the handover", len(ulrs))
	}

	if want := s6a.ULRFlagS6aS6dIndicator | s6a.ULRFlagIntraPLMNInterMMEHandover; ulrs[1].Flags != want {
		t.Errorf("ULR-Flags = %v, want %v", ulrs[1].Flags, want)
	}

	if got := mustVisitor(t, f); !got.Registered {
		t.Error("the visitor is not registered at its home HSS after the TAU")
	}

	if len(a.ue.Conn().TauAcceptPlain) == 0 {
		t.Error("the TAU after the handover was not accepted")
	}

	f.handle(a.ue.Conn(), trackingAreaUpdateNAS(t, a.ue, nil))

	if n := len(updateLocationRequests(t, f.hss)); n != 2 {
		t.Errorf("ULRs = %d after a later TAU, want no further registration", n)
	}
}

func TestCancelLocationDuringAMoveTo5GSKeepsTheSessions(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	a.ue.BeginIdleMobilityTo5GS(time.Minute)

	req, err := f.hss.CancelLocationRequest(s6a.CancelLocationRequest{IMSI: roamerIMSI, Type: s6a.CancellationMMEUpdateProcedure})
	ans := f.answer(t, f.m.CancelLocation, req, err)

	if r := resultOf(t, ans); r.Experimental || r.Code != diameter.ResultSuccess {
		t.Fatalf("CLA result %+v, want DIAMETER_SUCCESS", r)
	}

	if _, held := f.m.LookupUeByIMSI(roamerIMSI); !held {
		t.Fatal("the Cancel Location removed a context whose sessions are moving to 5GS")
	}

	if f.m.LookupPDN(a.ue, 5) == nil || len(f.sessions.releaseCauses) != 0 {
		t.Fatal("the Cancel Location released a PDN connection that is moving to 5GS")
	}

	if got := mustVisitor(t, f); got.Registered {
		t.Error("the visitor is still registered at its home HSS after the cancel")
	}

	f.m.RemoveUe(context.Background(), a.ue)
	f.m.SettleHomeNetworkForTest()

	if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 0 {
		t.Errorf("PURs = %d for a visitor that moved to 5GS, want none", len(purs))
	}
}

func TestVisitorPDNIsOfferedTo5GSOnlyWhenItsHomeSubscribesInterworking(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)
	a.ue.SetAccess(mme.Access{Allow4G: true, FiveGSInterworking: true})

	if got := interworkingPDUSessionID(f.m, a.ue, "internet", 5); got != 0 {
		t.Errorf("PDU session ID = %d for an APN without Interworking-5GS-Indicator, want none", got)
	}

	internet := fakehss.APN(1, "internet")
	internet.Interworking5GSIndicator = true
	sub := fakehss.Subscription(1, internet)

	v, _ := a.ue.Visitor()
	v.Subscription = &sub
	a.ue.SetVisitorForTest(v)

	if got := interworkingPDUSessionID(f.m, a.ue, "internet", 5); got != 5 {
		t.Errorf("PDU session ID = %d for an APN with Interworking-5GS-Indicator, want 5", got)
	}

	notAllowed := s6a.CNR5GCNotAllowed
	sub.CoreNetworkRestrictions = &notAllowed

	if got := interworkingPDUSessionID(f.m, a.ue, "internet", 5); got != 0 {
		t.Errorf("PDU session ID = %d with 5GC not allowed, want none", got)
	}
}

func TestEPSSNSSAIContainerOfAVisitorCarriesTheHomeSlice(t *testing.T) {
	container, err := snssaiPCOContainer(models.Snssai{Sst: 1, Sd: "000001"}, &models.Snssai{Sst: 1, Sd: "000001"}, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := fgs.ParseSNSSAI(container.Content[:len(container.Content)-3])
	if err != nil {
		t.Fatalf("decode the S-NSSAI: %v", err)
	}

	if got.MappedHPLMNSST == nil || *got.MappedHPLMNSST != 1 || got.MappedHPLMNSD == nil || *got.MappedHPLMNSD != [3]byte{0, 0, 1} {
		t.Errorf("S-NSSAI = %+v, want the mapped HPLMN S-NSSAI", got)
	}
}
