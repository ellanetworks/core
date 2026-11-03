// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func (f *roamingMME) registeredRoamer(t *testing.T) *roamerAttach {
	t.Helper()

	a := f.attach(t, eps.AttachTypeEPS, "")
	a.accept(t)
	a.ue.ForceStateForTest(mme.EMMRegistered)

	return a
}

func (f *roamingMME) answer(t *testing.T, handler func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message, req *diameter.Message, err error) *diameter.Message {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}

	ans, err := fakehss.Answer(handler(context.Background(), f.hss.Local, req))
	if err != nil {
		t.Fatal(err)
	}

	f.m.SettleHomeNetworkForTest()

	return ans
}

func resultOf(t *testing.T, ans *diameter.Message) tgpp.Result {
	t.Helper()

	r, err := tgpp.ParseResult(ans)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func (a *roamerAttach) networkDetach(t *testing.T) *eps.DetachRequestNetwork {
	t.Helper()

	req, err := eps.ParseDetachRequestNetwork(a.protected(t, decodeDownlinkNAS(t, a.last(t))))
	if err != nil {
		t.Fatalf("expected a Detach Request: %v", err)
	}

	return req
}

func TestVisitorPurgeAtRemoval(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  s6a.PUAFlags
		frozen bool
	}{
		{name: "M-TMSI not frozen"},
		{name: "M-TMSI frozen", flags: s6a.PUAFlagFreezeMTMSI, frozen: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.hss.SetPUAFlags(tc.flags)
			f.m.SetMTMSIFreezeTimeForTest(time.Hour)

			a := f.registeredRoamer(t)
			tmsi := a.ue.Tmsi()

			a.ue.Conn().UpdateLocation(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x1234567}, s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 0x0102})

			f.m.RemoveUe(context.Background(), a.ue)
			f.m.SettleHomeNetworkForTest()

			purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE)
			if len(purs) != 1 {
				t.Fatalf("PURs = %d, want 1", len(purs))
			}

			if env := tgpp.ParseEnvelope(purs[0]); env.DestinationHost != roamerHSSHost || env.DestinationRealm != roamerHomeRealm {
				t.Fatalf("PUR envelope %+v, want the learnt HSS %s", env, roamerHSSHost)
			}

			pur, err := s6a.ParsePurgeUERequest(purs[0])
			if err != nil {
				t.Fatal(err)
			}

			if pur.IMSI != roamerIMSI || pur.Location == nil || pur.Location.MME == nil ||
				string(pur.Location.MME.ECGI) != string([]byte{0x00, 0xf1, 0x10, 0x01, 0x23, 0x45, 0x67}) ||
				string(pur.Location.MME.TAI) != string([]byte{0x00, 0xf1, 0x10, 0x01, 0x02}) || pur.Location.MME.Age == nil {
				t.Fatalf("PUR %+v, want the IMSI and the last known E-UTRAN location", pur)
			}

			if got := f.reg.purges(); len(got) != 1 || got[0] != roamerIMSI {
				t.Fatalf("purged = %v, want the registration purged after the PUA", got)
			}

			if f.m.MTMSIAllocatedForTest(tmsi) != tc.frozen {
				t.Fatalf("M-TMSI allocated = %v, want %v", !tc.frozen, tc.frozen)
			}
		})
	}
}

func TestVisitorPurgeFailureStillPurgesLocally(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)
	f.hss.SetDown(true)

	f.m.RemoveUe(context.Background(), a.ue)
	f.m.SettleHomeNetworkForTest()

	if got := f.reg.purges(); len(got) != 1 {
		t.Fatalf("purged = %v, want the local registration purged although the HSS is down", got)
	}
}

func TestNoPurgeWithoutAHomeRegistration(t *testing.T) {
	f := newRoamingMME(t)
	f.hss.FailULR(&s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed)})

	a := f.attach(t, eps.AttachTypeEPS, "")
	a.reject(t, true)
	f.m.RemoveUe(context.Background(), a.ue)
	f.m.SettleHomeNetworkForTest()

	if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 0 {
		t.Fatalf("PURs = %d after a refused ULR, want none", len(purs))
	}
}

func TestPurgeIsOrderedBeforeTheNextAttach(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	release := f.hss.Hold(s6a.CommandPurgeUE)
	f.m.RemoveUe(context.Background(), a.ue)

	next := f.startAttachAsync(t)

	time.Sleep(20 * time.Millisecond)

	if airs := s6aRequests(t, f.hss, s6a.CommandAuthenticationInformation); len(airs) != 1 {
		t.Fatalf("AIRs = %d while the PUR is outstanding, want the new AIR held behind it", len(airs))
	}

	release()
	f.settle(next.ue.Conn())

	if airs := s6aRequests(t, f.hss, s6a.CommandAuthenticationInformation); len(airs) != 2 {
		t.Fatalf("AIRs = %d, want the new attach's AIR after the PUR", len(airs))
	}
}

func (f *roamingMME) startAttachAsync(t *testing.T) *roamerAttach {
	t.Helper()

	cc := &captureConn{}
	ue := newAttachUe(f.m, cc, 8)
	req := attachRequestFor(t, roamerIMSI, eps.AttachTypeEPS, "")

	ue.Conn().Serialize(func() { HandleNAS(context.Background(), f.m, ue.Conn(), req) })

	return &roamerAttach{ue: ue, cc: cc}
}

func TestSlowHomeHSSDoesNotStallOtherUEs(t *testing.T) {
	f := newRoamingMME(t)
	release := f.hss.Hold(s6a.CommandAuthenticationInformation)

	defer release()

	cc := &captureConn{}
	roamer := newAttachUe(f.m, cc, 7)
	home := newAttachUe(f.m, cc, 8)

	done := make(chan struct{})

	go func() {
		defer close(done)

		roamer.Conn().Serialize(func() {
			HandleNAS(context.Background(), f.m, roamer.Conn(), attachRequestFor(t, roamerIMSI, eps.AttachTypeEPS, ""))
		})

		home.Conn().Serialize(func() {
			HandleNAS(context.Background(), f.m, home.Conn(), attachRequestFor(t, testSubscriber.IMSI, eps.AttachTypeEPS, ""))
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the eNB loop is blocked by the roamer's home HSS")
	}

	if cc.count() != 1 {
		t.Fatalf("downlinks = %d, want only the home subscriber's Authentication Request", cc.count())
	}

	if _, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected the home subscriber's Authentication Request: %v", err)
	}

	var order []string

	roamer.Conn().Serialize(func() { order = append(order, "queued") })

	if len(order) != 0 {
		t.Fatal("a NAS message of the roamer ran ahead of its outstanding AIR")
	}

	release()
	roamer.Conn().SettleForTest()

	if len(order) != 1 || cc.count() != 2 {
		t.Fatalf("order = %v, downlinks = %d, want the roamer's Authentication Request then its queued message", order, cc.count())
	}
}

func TestCancelLocation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		typ       s6a.CancellationType
		flags     s6a.CLRFlags
		detach    bool
		reattach  bool
		removed   bool
		purgeHome bool
	}{
		{name: "subscription withdrawal", typ: s6a.CancellationSubscriptionWithdrawal, detach: true},
		{name: "subscription withdrawal with re-attach", typ: s6a.CancellationSubscriptionWithdrawal, flags: s6a.CLRFlagReattachRequired, detach: true, reattach: true},
		{name: "MME update", typ: s6a.CancellationMMEUpdateProcedure, removed: true},
		{name: "initial attach", typ: s6a.CancellationInitialAttachProcedure, removed: true},
		{name: "SGSN update", typ: s6a.CancellationSGSNUpdateProcedure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			a := f.registeredRoamer(t)

			req, err := f.hss.CancelLocationRequest(s6a.CancelLocationRequest{IMSI: roamerIMSI, Type: tc.typ, Flags: tc.flags})
			ans := f.answer(t, f.m.CancelLocation, req, err)

			if r := resultOf(t, ans); r.Experimental || r.Code != diameter.ResultSuccess {
				t.Fatalf("CLA result %+v, want DIAMETER_SUCCESS", r)
			}

			if tc.detach {
				d := a.networkDetach(t)
				if (d.TypeOfDetach == eps.DetachTypeReattachRequired) != tc.reattach || d.Cause != nil {
					t.Fatalf("Detach Request %+v, want re-attach required = %v and no cause", d, tc.reattach)
				}

				f.m.RemoveUe(context.Background(), a.ue)
				f.m.SettleHomeNetworkForTest()
			}

			if tc.removed {
				parseUEContextReleaseCommand(t, a.last(t))
				f.m.RemoveUe(context.Background(), a.ue)
				f.m.SettleHomeNetworkForTest()
			}

			if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 0 {
				t.Fatalf("PURs = %d after a Cancel Location, want none", len(purs))
			}

			if _, held := f.m.LookupUeByIMSI(roamerIMSI); held == (tc.detach || tc.removed) {
				t.Fatalf("context held = %v", held)
			}
		})
	}
}

func TestCancelLocationIgnoresUnknownAndForeignRequests(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)
	sent := a.cc.count()

	req, err := f.hss.CancelLocationRequest(s6a.CancelLocationRequest{IMSI: "999700000000099", Type: s6a.CancellationSubscriptionWithdrawal})
	if r := resultOf(t, f.answer(t, f.m.CancelLocation, req, err)); r.Code != diameter.ResultSuccess {
		t.Fatalf("CLA for an unknown IMSI %+v, want DIAMETER_SUCCESS", r)
	}

	foreign := fakehss.New(f.hss.Local, diameter.Identity{OriginHost: "hss.epc.mnc001.mcc998.3gppnetwork.org", OriginRealm: "epc.mnc001.mcc998.3gppnetwork.org"})

	req, err = foreign.CancelLocationRequest(s6a.CancelLocationRequest{IMSI: roamerIMSI, Type: s6a.CancellationSubscriptionWithdrawal})
	if r := resultOf(t, f.answer(t, f.m.CancelLocation, req, err)); r.Code != diameter.ResultSuccess {
		t.Fatalf("CLA from another network %+v, want DIAMETER_SUCCESS", r)
	}

	req, err = f.hss.CancelLocationRequest(s6a.CancelLocationRequest{IMSI: testSubscriber.IMSI, Type: s6a.CancellationSubscriptionWithdrawal})
	if r := resultOf(t, f.answer(t, f.m.CancelLocation, req, err)); r.Code != diameter.ResultSuccess {
		t.Fatalf("CLA for a home subscriber %+v, want DIAMETER_SUCCESS", r)
	}

	if a.cc.count() != sent {
		t.Fatal("a Cancel Location from outside the roamer's home network detached it")
	}
}

func TestInsertSubscriberData(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	ambr := s6a.AMBR{UL: 10_000_000, DL: 20_000_000}
	update := s6a.SubscriptionData{
		AMBR: &ambr,
		APNConfigurationProfile: &s6a.APNConfigurationProfile{
			ContextIdentifier: 1,
			Included:          s6a.ModifiedAddedAPNConfigurationsIncluded,
			Configurations:    []s6a.APNConfiguration{fakehss.APN(1, "internet"), fakehss.APN(3, "iot")},
		},
	}

	req, err := f.hss.InsertSubscriberDataRequest(s6a.InsertSubscriberDataRequest{IMSI: roamerIMSI, Subscription: update, Flags: s6a.IDRFlagTADSDataRequest | s6a.IDRFlagEPSUserStateRequest | s6a.IDRFlagEPSLocationInformationRequest})
	ans := f.answer(t, f.m.InsertSubscriberData, req, err)

	ida, err := s6a.ParseInsertSubscriberDataAnswer(ans)
	if err != nil {
		t.Fatalf("IDA: %v", err)
	}

	if ida.IMSVoiceOverPSSupported == nil || *ida.IMSVoiceOverPSSupported != s6a.IMSVoiceOverPSNotSupported ||
		ida.RATType == nil || *ida.RATType != tgpp.RATEUTRAN ||
		ida.UserState == nil || *ida.UserState != s6a.UserStateConnectedReachableForPaging || ida.Location == nil {
		t.Fatalf("IDA %+v, want the T-ADS data, the user state and the location", ida)
	}

	sub, ok := f.m.HomeSubscription(roamerIMSI)
	if !ok || sub.AMBR.UL != ambr.UL {
		t.Fatalf("home subscription %+v, want the new AMBR", sub)
	}

	if _, ok := sub.APNConfigurationProfile.Configuration(3); !ok {
		t.Fatal("the added APN configuration was not stored")
	}

	if _, ok := sub.APNConfigurationProfile.Configuration(2); !ok {
		t.Fatal("an IDR without All-APN-Configurations-Included replaced the stored APNs")
	}

	if a.ue.Ambr == nil || a.ue.Ambr.Uplink.Bps() != 10_000_000 {
		t.Fatalf("UE-AMBR = %+v, want min(home 10 Mbps, local 1 Gbps)", a.ue.Ambr)
	}
}

func TestInsertSubscriberDataResults(t *testing.T) {
	for _, tc := range []struct {
		name  string
		imsi  string
		flags s6a.IDRFlags
		want  tgpp.Result
	}{
		{name: "unknown IMSI", imsi: "999700000000099", want: tgpp.Experimental(tgpp.ResultErrorUserUnknown)},
		{name: "home subscriber", imsi: testSubscriber.IMSI, want: tgpp.Experimental(tgpp.ResultErrorUserUnknown)},
		{name: "current location only", imsi: roamerIMSI, flags: s6a.IDRFlagCurrentLocationRequest, want: tgpp.Result{Code: diameter.ResultUnableToComply}},
		{name: "UE reachability", imsi: roamerIMSI, flags: s6a.IDRFlagUEReachabilityRequest, want: tgpp.Result{Code: diameter.ResultUnableToComply}},
		{name: "empty subscription data", imsi: roamerIMSI, want: tgpp.Result{Code: diameter.ResultSuccess}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.registeredRoamer(t)

			req, err := f.hss.InsertSubscriberDataRequest(s6a.InsertSubscriberDataRequest{IMSI: tc.imsi, Flags: tc.flags})
			if got := resultOf(t, f.answer(t, f.m.InsertSubscriberData, req, err)); got != tc.want {
				t.Fatalf("IDA result %+v, want %+v", got, tc.want)
			}

			v := mustVisitor(t, f)
			if tc.want.Code == diameter.ResultUnableToComply && v.Confirmed {
				t.Fatal("a failed IDR did not mark the subscriber to be restored")
			}
		})
	}
}

func mustVisitor(t *testing.T, f *roamingMME) visitorView {
	t.Helper()

	ue, held := f.m.LookupUeByIMSI(roamerIMSI)
	if !held {
		t.Fatal("the roamer's context is gone")
	}

	v, ok := ue.Visitor()
	if !ok {
		t.Fatal("the roamer's context is not a visitor")
	}

	return visitorView{Confirmed: v.Confirmed, Registered: v.Registered}
}

type visitorView struct {
	Confirmed  bool
	Registered bool
}

func TestInsertSubscriberDataBarringDetaches(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	barred := s6a.SubscriberStatusOperatorDeterminedBarring
	odb := s6a.ODBRoamerAccessVPLMNAP

	req, err := f.hss.InsertSubscriberDataRequest(s6a.InsertSubscriberDataRequest{IMSI: roamerIMSI, Subscription: s6a.SubscriptionData{SubscriberStatus: &barred, OperatorDeterminedBarring: &odb}})
	if r := resultOf(t, f.answer(t, f.m.InsertSubscriberData, req, err)); r.Code != diameter.ResultSuccess {
		t.Fatalf("IDA result %+v", r)
	}

	if d := a.networkDetach(t); d.Cause == nil || *d.Cause != eps.EMMCauseNoSuitableCellsInTrackingArea {
		t.Fatalf("Detach Request %+v, want EMM cause #15 for operator determined barring", d)
	}
}

func TestDeleteSubscriberData(t *testing.T) {
	for _, tc := range []struct {
		name    string
		imsi    string
		del     s6a.SubscriptionDataDeletion
		want    tgpp.Result
		removed uint32
	}{
		{name: "unknown IMSI", imsi: "999700000000099", del: s6a.SubscriptionDataDeletion{Flags: s6a.DSRFlagSTNSR}, want: tgpp.Experimental(tgpp.ResultErrorUserUnknown)},
		{name: "default APN", imsi: roamerIMSI, del: s6a.SubscriptionDataDeletion{Flags: s6a.DSRFlagPDNSubscriptionContextsWithdrawal, ContextIdentifiers: []uint32{1}}, want: tgpp.Result{Code: diameter.ResultUnableToComply}},
		{name: "complete APN profile", imsi: roamerIMSI, del: s6a.SubscriptionDataDeletion{Flags: s6a.DSRFlagCompleteAPNConfigurationProfileWithdrawal}, want: tgpp.Result{Code: diameter.ResultUnableToComply}},
		{name: "non-default APN", imsi: roamerIMSI, del: s6a.SubscriptionDataDeletion{Flags: s6a.DSRFlagPDNSubscriptionContextsWithdrawal, ContextIdentifiers: []uint32{2}}, want: tgpp.Result{Code: diameter.ResultSuccess}, removed: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.registeredRoamer(t)

			req, err := f.hss.DeleteSubscriberDataRequest(s6a.DeleteSubscriberDataRequest{IMSI: tc.imsi, SubscriptionDataDeletion: tc.del})
			if got := resultOf(t, f.answer(t, f.m.DeleteSubscriberData, req, err)); got != tc.want {
				t.Fatalf("DSA result %+v, want %+v", got, tc.want)
			}

			sub, _ := f.m.HomeSubscription(roamerIMSI)

			_, kept := sub.APNConfigurationProfile.Configuration(2)
			if kept == (tc.removed == 2) {
				t.Fatalf("APN context 2 kept = %v", kept)
			}
		})
	}
}

func TestResetMarksVisitorsForRestoration(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	other := fakehss.New(f.hss.Local, diameter.Identity{OriginHost: "hss2." + roamerHomeRealm, OriginRealm: roamerHomeRealm})

	req, err := other.ResetRequest(s6a.ResetRequest{})
	f.answer(t, f.m.Reset, req, err)

	if v := mustVisitor(t, f); !v.Confirmed {
		t.Fatal("a reset from another HSS unconfirmed the visitor")
	}

	req, err = f.hss.ResetRequest(s6a.ResetRequest{UserIDs: []string{"99801"}})
	f.answer(t, f.m.Reset, req, err)

	if v := mustVisitor(t, f); !v.Confirmed {
		t.Fatal("a reset for other User-Ids unconfirmed the visitor")
	}

	req, err = f.hss.ResetRequest(s6a.ResetRequest{UserIDs: []string{"99970"}})
	if r := resultOf(t, f.answer(t, f.m.Reset, req, err)); r.Code != diameter.ResultSuccess {
		t.Fatalf("RSA result %+v", r)
	}

	if v := mustVisitor(t, f); v.Confirmed {
		t.Fatal("the reset did not mark the visitor Location Not Confirmed")
	}

	before := len(s6aRequests(t, f.hss, s6a.CommandUpdateLocation))

	if err := f.m.ConfirmRegistration(context.Background(), a.ue); err != nil {
		t.Fatal(err)
	}

	f.m.SettleHomeNetworkForTest()

	ulrs := s6aRequests(t, f.hss, s6a.CommandUpdateLocation)
	if len(ulrs) != before+1 {
		t.Fatalf("ULRs = %d, want a restoration ULR at the next authenticated contact", len(ulrs)-before)
	}

	if env := tgpp.ParseEnvelope(ulrs[len(ulrs)-1]); env.DestinationHost != "" || env.DestinationRealm != roamerHomeRealm {
		t.Fatalf("restoration ULR envelope %+v, want the home realm without Destination-Host", env)
	}

	ulr, err := s6a.ParseUpdateLocationRequest(ulrs[len(ulrs)-1])
	if err != nil {
		t.Fatal(err)
	}

	if ulr.Flags != s6a.ULRFlagS6aS6dIndicator {
		t.Fatalf("restoration ULR-Flags = %s, want S6a/S6d-Indicator only", ulr.Flags)
	}

	if v := mustVisitor(t, f); !v.Confirmed {
		t.Fatal("a successful restoration did not confirm the visitor")
	}
}

func TestNotifyOnEquipmentChange(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		detach    bool
		confirmed bool
	}{
		{name: "success", confirmed: true},
		{name: "unknown serving node", err: &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownServingNode)}},
		{name: "user unknown", err: &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, detach: true, confirmed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			a := f.registeredRoamer(t)
			f.hss.FailNOR(tc.err)

			imei := mustIMEI(t, "3506382143658702")

			if !a.ue.MarkSecured(imei) {
				t.Fatal("a new IMEISV was not reported as a change")
			}

			f.m.NotifyEquipmentChange(context.Background(), a.ue)
			f.m.SettleHomeNetworkForTest()

			nors := s6aRequests(t, f.hss, s6a.CommandNotify)
			if len(nors) != 1 {
				t.Fatalf("NORs = %d, want 1", len(nors))
			}

			nor, err := s6a.ParseNotifyRequest(nors[0])
			if err != nil {
				t.Fatal(err)
			}

			if nor.Terminal == nil || nor.Terminal.IMEI != roamerTACAndSNR || nor.Terminal.SoftwareVersion != "02" {
				t.Fatalf("NOR Terminal-Information %+v, want the new IMEI and Software-Version", nor.Terminal)
			}

			if tc.detach {
				if _, err := eps.ParseDetachRequestNetwork(a.protected(t, decodeDownlinkNAS(t, a.last(t)))); err != nil {
					t.Fatalf("expected a Detach Request after DIAMETER_ERROR_USER_UNKNOWN: %v", err)
				}

				return
			}

			if v := mustVisitor(t, f); v.Confirmed != tc.confirmed {
				t.Fatalf("confirmed = %v, want %v", v.Confirmed, tc.confirmed)
			}
		})
	}
}

func TestNoNotifyWithoutAnIMEIChange(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	if a.ue.MarkSecured(mustIMEI(t, roamerIMEISV)) {
		t.Fatal("the same IMEISV was reported as a change")
	}
}

func TestRoamingWithdrawalDetachesTheVisitor(t *testing.T) {
	for _, tc := range []struct {
		name string
		why  ueregistration.Withdrawal
		want eps.EMMCause
	}{
		{name: "partner withdrawn", why: ueregistration.RoamingWithdrawn, want: eps.EMMCausePLMNNotAllowed},
		{name: "4G agreement withdrawn", why: ueregistration.RoamingAgreementWithdrawn, want: eps.EMMCauseNoSuitableCellsInTrackingArea},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			a := f.registeredRoamer(t)

			f.reg.withdraw = &tc.why
			f.m.ReconcileRegistrations(context.Background())

			d := a.networkDetach(t)
			if d.TypeOfDetach != eps.DetachTypeReattachNotRequired || d.Cause == nil || *d.Cause != tc.want {
				t.Fatalf("Detach Request %+v, want re-attach not required with cause %s", d, tc.want)
			}

			f.m.RemoveUe(context.Background(), a.ue)
			f.m.SettleHomeNetworkForTest()

			if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 1 {
				t.Fatalf("PURs = %d, want the withdrawn visitor purged at its home HSS", len(purs))
			}
		})
	}
}

func TestWithdrawnPartnerGate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		withdrawal db.RoamingWithdrawal
		want       eps.EMMCause
		extended   bool
	}{
		{name: "partner", withdrawal: db.RoamingWithdrawal{}, want: eps.EMMCausePLMNNotAllowed},
		{name: "PLMN", withdrawal: db.RoamingWithdrawal{Mcc: "999", Mnc: "70"}, want: eps.EMMCausePLMNNotAllowed},
		{name: "4G agreement", withdrawal: db.RoamingWithdrawal{Type: db.UERegistrationTypeMME}, want: eps.EMMCauseNoSuitableCellsInTrackingArea, extended: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			w := tc.withdrawal
			w.PartnerID = roamerPartnerID
			w.ExpiresAt = time.Now().Add(time.Minute).Unix()
			f.partners.partners[0].Withdrawals = []db.RoamingWithdrawal{w}

			a := f.startAttach(t, roamerIMSI, eps.AttachTypeEPS, "")

			reject := a.reject(t, false)
			if reject.Cause != tc.want || (reject.ExtendedEMMCause != nil && reject.ExtendedEMMCause.EUTRANNotAllowed) != tc.extended {
				t.Fatalf("Attach Reject %+v, want cause %s", reject, tc.want)
			}
		})
	}
}

func TestExpiredOrForeignWithdrawalDoesNotGate(t *testing.T) {
	f := newRoamingMME(t)
	f.partners.partners[0].Withdrawals = []db.RoamingWithdrawal{
		{PartnerID: roamerPartnerID, ExpiresAt: time.Now().Add(-time.Second).Unix()},
		{PartnerID: roamerPartnerID, Mcc: "999", Mnc: "71", ExpiresAt: time.Now().Add(time.Minute).Unix()},
		{PartnerID: roamerPartnerID, Type: db.UERegistrationTypeAMF3GPPAccess, ExpiresAt: time.Now().Add(time.Minute).Unix()},
	}

	a := f.attach(t, eps.AttachTypeEPS, "")
	a.accept(t)
}

func TestRegistrationRefusedDuringWithdrawal(t *testing.T) {
	f := newRoamingMME(t)
	f.m.Registrations = refusingRegistrar{f.reg}

	a := f.attach(t, eps.AttachTypeEPS, "")

	if reject := a.reject(t, true); reject.Cause != eps.EMMCausePLMNNotAllowed {
		t.Fatalf("Attach Reject cause %s, want #11", reject.Cause)
	}

	f.m.RemoveUe(context.Background(), a.ue)
	f.m.SettleHomeNetworkForTest()

	if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 1 {
		t.Fatalf("PURs = %d, want the HSS registration made by the ULR purged", len(purs))
	}
}

func mustIMEI(t *testing.T, imeisv string) etsi.IMEI {
	t.Helper()

	imei, err := etsi.NewIMEIFromPEI(imeisv)
	if err != nil {
		t.Fatal(err)
	}

	return imei
}

type refusingRegistrar struct{ *visitorRegistrar }

func (refusingRegistrar) RegisterVisitor(context.Context, string, string) (int64, error) {
	return 0, fmt.Errorf("%w: test", db.ErrRoamingRefused)
}

func TestVisitorDetachPurges(t *testing.T) {
	f := newRoamingMME(t)
	a := f.registeredRoamer(t)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff:         true,
		TypeOfDetach:      eps.DetachTypeEPS,
		EPSMobileIdentity: eps.IMSIIdentity(eps.IMSI(roamerIMSI)),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 1), nas.DirectionUplink,
		mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, a.knasInt, a.knasEnc))
	if err != nil {
		t.Fatal(err)
	}

	f.handle(a.ue.Conn(), wire)
	parseUEContextReleaseCommand(t, a.last(t))

	f.m.ReleaseUEContextLocally(context.Background(), a.ue, "UE Context Release Complete", models.ReleaseFor(models.ReleaseDeregistration))
	f.m.SettleHomeNetworkForTest()

	if _, held := f.m.LookupUeByIMSI(roamerIMSI); held {
		t.Fatal("the detached visitor's context is still held")
	}

	if purs := s6aRequests(t, f.hss, s6a.CommandPurgeUE); len(purs) != 1 {
		t.Fatalf("PURs = %d after the visitor detached, want 1", len(purs))
	}
}
