// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehplmn"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

type dlCounter struct {
	mu                        sync.Mutex
	downlinkNasTransportCalls int
}

func (d *dlCounter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		return 0, err
	}

	if m, ok := pdu.(*ngap.InitiatingMessage); ok && m.ProcedureCode == ngap.ProcDownlinkNASTransport {
		d.mu.Lock()
		d.downlinkNasTransportCalls++
		d.mu.Unlock()
	}

	return len(b), nil
}

func (f *roamingFixture) capturingVisitorUE(t *testing.T) (*UeContext, *UeConn, *dlCounter) {
	t.Helper()

	ue := NewUeContext()
	ue.mu.Lock()
	ue.suci = roamerSUCI
	ue.mu.Unlock()

	sender := &dlCounter{}
	radio := &Radio{Conn: sender}
	radio.BindAMFForTest(f.amf)

	conn := NewUeConnForTest(radio, 1, 10)
	f.amf.AttachUeConn(t.Context(), ue, conn)

	if err := f.amf.AdmitVisitor(t.Context(), ue); err != nil {
		t.Fatalf("AdmitVisitor: %v", err)
	}

	return ue, conn, sender
}

func (f *roamingFixture) connectedVisitor(t *testing.T) (*UeContext, *UeConn, *dlCounter) {
	t.Helper()

	ue, conn, sender := f.capturingVisitorUE(t)
	f.authenticate(t, ue, conn)
	f.registerAtHPLMN(t, ue, conn)

	if err := f.amf.RegisterUE(t.Context(), ue); err != nil {
		t.Fatal(err)
	}

	ue.ForceStateForTest(Registered)

	return ue, conn, sender
}

func TestHPLMNUDMGroupIsSelectedBySUPIAndPinned(t *testing.T) {
	for _, ranges := range []bool{false, true} {
		f := newRoamingFixture(t)
		f.home.UDMGroup("g1", ranges, "001010000000099")
		f.home.UDMGroup("g2", ranges, roamerIMSI)

		ue, conn := f.visitorUE(t)
		f.authenticate(t, ue, conn)
		f.registerAtHPLMN(t, ue, conn)

		v, _ := ue.Visitor()
		if v.HPLMN.UDMGroupID != "g2" || v.HPLMN.UDMInstanceID == "" || v.HPLMN.Supi != "imsi-"+roamerIMSI {
			t.Fatalf("ranges=%v: home target %+v", ranges, v.HPLMN)
		}

		udm := f.home.UDMHost("g2")

		for _, r := range f.home.Requests() {
			switch {
			case strings.HasPrefix(r.Path, "/nudm-"):
				if r.Host != udm {
					t.Errorf("ranges=%v: %s %s sent to %s, want %s", ranges, r.Method, r.Path, r.Host, udm)
				}
			case strings.HasPrefix(r.Path, "/nnrf-disc") && strings.Contains(r.Query, "target-nf-type=UDM"):
				q, _ := url.ParseQuery(r.Query)
				if q.Get("supi") != "imsi-"+roamerIMSI {
					t.Errorf("ranges=%v: UDM discovery without the SUPI: %s", ranges, r.Query)
				}
			}
		}

		f.amf.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))
		f.amf.SettleHPLMN()

		if !f.home.Purged("imsi-" + roamerIMSI) {
			t.Fatalf("ranges=%v: the purge did not reach the pinned UDM", ranges)
		}
	}
}

func TestHPLMNOAuth2TokensComeFromTheHPLMNNRF(t *testing.T) {
	f := newRoamingFixture(t)
	f.home.RequireOAuth2()

	ue, conn := f.visitorUE(t)
	f.authenticate(t, ue, conn)
	f.registerAtHPLMN(t, ue, conn)

	var tokens []fakehplmn.Request

	for _, r := range f.home.Requests() {
		if r.Path == wire.AccessTokenPath() {
			tokens = append(tokens, r)
		}
	}

	if len(tokens) != 3 {
		t.Fatalf("%d access token requests, want one per service (nausf-auth, nudm-uecm, nudm-sdm)", len(tokens))
	}

	form, _ := url.ParseQuery(string(tokens[0].Body))
	if form.Get("nfType") != "AMF" || form.Get("targetNfType") != "AUSF" || form.Get("scope") != "nausf-auth" ||
		form.Get("requesterPlmn") != `{"mcc":"999","mnc":"70"}` || form.Get("targetPlmn") != `{"mcc":"001","mnc":"01"}` {
		t.Fatalf("access token request %v", form)
	}

	f.home.RevokeTokens()

	v, _ := ue.Visitor()
	if _, err := f.amf.HPLMN.SubscriptionData(t.Context(), v.HPLMN, "imsi-"+roamerIMSI); err != nil {
		t.Fatalf("a revoked token was not refreshed: %v", err)
	}
}

func TestHPLMNAnswersWithoutVerdictAreTransient(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)
	f.authenticate(t, ue, conn)

	f.home.Fail(http.MethodPut, "amf-3gpp-access", wire.NewProblem(http.StatusForbidden, "", "N32-f policy"))

	done := make(chan error, 1)

	f.amf.RequestHPLMNRegistration(t.Context(), ue, conn, true, func(_ context.Context, err error) { done <- err })

	err := waitDone(t, done)
	if r, ok := HPLMNRegistrationRejection(err); ok {
		t.Fatalf("a 403 without cause became %+v", r)
	}

	f.home.Heal()
	f.home.Fail(http.MethodGet, "imsi-"+roamerIMSI, wire.NewProblem(http.StatusNotFound, wire.CauseDataNotFound, ""))

	f.amf.RequestHPLMNRegistration(t.Context(), ue, conn, true, func(_ context.Context, err error) { done <- err })

	r, ok := HPLMNRegistrationRejection(waitDone(t, done))
	if !ok || r.Cause != fgs.GMMCauseN1ModeNotAllowed {
		t.Fatalf("SDM DATA_NOT_FOUND: %+v, want #27", r)
	}
}

func sorNotification(t *testing.T, v roaming.Visitor5G, info wire.SorInfo) wire.ModificationNotification {
	t.Helper()

	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}

	path := "/sorInfo"

	return wire.ModificationNotification{
		SubscriptionID: v.SDM.ID,
		NotifyItems: []wire.NotifyItem{{
			ResourceID: "http://udm.example" + wire.SDMAmDataPath("imsi-"+roamerIMSI),
			Changes:    []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &path, NewValue: raw}},
		}},
	}
}

func TestSoRAfterRegistrationIsDeliveredAndAcknowledged(t *testing.T) {
	f := newRoamingFixture(t)
	ue, _, sender := f.connectedVisitor(t)
	v, _ := ue.Visitor()

	before := len(f.home.Requests())
	ack := true

	n := sorNotification(t, v, wire.SorInfo{AckInd: &ack, SorMacIausf: "00112233445566778899aabbccddeeff", Countersor: "0002", ProvisioningTime: "2026-11-04T10:00:00Z"})
	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, n); err != nil {
		t.Fatal(err)
	}

	f.amf.SettleHPLMN()

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("%d DL NAS Transports, want 1", sender.downlinkNasTransportCalls)
	}

	if len(f.home.Requests()) != before {
		t.Fatalf("a SoR-only change re-read the subscription: %v", f.home.Requests()[before:])
	}

	if f.amf.SoRContainer(ue) != nil {
		t.Fatal("delivered SoR information would be repeated in the next Registration Accept")
	}

	f.amf.AcknowledgeSoR(t.Context(), ue, nil)

	ackContainer := append([]byte{0x01}, make([]byte, 16)...)
	ackContainer[1] = 0xcd

	f.amf.AcknowledgeSoRTransport(t.Context(), ue, ackContainer)
	f.amf.SettleHPLMN()

	got, ok := f.home.SoRAck("imsi-" + roamerIMSI)
	if !ok || got.SorMacIue != "cd000000000000000000000000000000" || got.ProvisioningTime != "2026-11-04T10:00:00Z" {
		t.Fatalf("SoR acknowledgement %+v", got)
	}
}

func TestSoRForAnIdleVisitorPagesItAndIsDeliveredOnceConnected(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn, sender := f.connectedVisitor(t)
	v, _ := ue.Visitor()

	tmsi, err := etsi.NewTMSI(0x01020304)
	if err != nil {
		t.Fatal(err)
	}

	ue.SetTmsiForTest(tmsi)
	ue.AllocateRegistrationArea([]models.Tai{{PlmnID: &models.PlmnID{Mcc: "999", Mnc: "70"}, Tac: "000001"}})
	ue.active.Store(nil)

	ack := false

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, sorNotification(t, v, wire.SorInfo{AckInd: &ack, SorMacIausf: "00112233445566778899aabbccddeeff", Countersor: "0003", ProvisioningTime: "2026-11-04T11:00:00Z"})); err != nil {
		t.Fatal(err)
	}

	f.amf.SettleHPLMN()

	if sender.downlinkNasTransportCalls != 0 || f.amf.SoRContainer(ue) == nil {
		t.Fatalf("idle visitor: %d DL NAS Transports, pending %x", sender.downlinkNasTransportCalls, f.amf.SoRContainer(ue))
	}

	pending := ue.PagingPending()
	if ue.PagingState() != PagingAttempting || !pending.SignallingOnly() || pending.Req.N1Class != models.N1Class5GMM {
		t.Fatalf("paging state %s, pending %+v", ue.PagingState(), pending)
	}

	ue.active.Store(conn)
	ue.PagingAnswered()
	f.amf.ResumeHPLMNChanges(t.Context(), ue)
	ue.PagingDelivered(t.Context())

	if sender.downlinkNasTransportCalls != 1 || f.amf.SoRContainer(ue) != nil {
		t.Fatalf("after reconnection: %d DL NAS Transports", sender.downlinkNasTransportCalls)
	}

	if ue.PagingState() != PagingIdle {
		t.Fatalf("paging state %s after delivery", ue.PagingState())
	}
}

func TestSoRForAnUnpageableVisitorWaitsForItsNextConnection(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn, sender := f.connectedVisitor(t)
	v, _ := ue.Visitor()

	ue.active.Store(nil)
	ue.ForceStateForTest(RegistrationInitiated)

	ack := false

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, sorNotification(t, v, wire.SorInfo{AckInd: &ack, SorMacIausf: "00112233445566778899aabbccddeeff", Countersor: "0004", ProvisioningTime: "2026-11-04T12:00:00Z"})); err != nil {
		t.Fatal(err)
	}

	f.amf.SettleHPLMN()

	if sender.downlinkNasTransportCalls != 0 || f.amf.SoRContainer(ue) == nil || ue.PagingState() != PagingIdle {
		t.Fatalf("unpageable visitor: %d DL NAS Transports, pending %x, paging %s", sender.downlinkNasTransportCalls, f.amf.SoRContainer(ue), ue.PagingState())
	}

	ue.ForceStateForTest(Registered)
	ue.active.Store(conn)
	f.amf.ResumeHPLMNChanges(t.Context(), ue)

	if sender.downlinkNasTransportCalls != 1 || f.amf.SoRContainer(ue) != nil {
		t.Fatalf("after reconnection: %d DL NAS Transports", sender.downlinkNasTransportCalls)
	}
}

func TestSliceChangeRequestsRegistrationInsteadOfDeregistering(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn, sender := f.connectedVisitor(t)
	ue.SetAllowedNssai([]models.Snssai{{Sst: 1}})

	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) {
		s.AM.Nssai = &wire.Nssai{DefaultSingleNssais: []wire.Snssai{wire.NewSnssai(2, "")}}
	})

	v, _ := ue.Visitor()
	path := "/nssai"

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, wire.ModificationNotification{NotifyItems: []wire.NotifyItem{{
		ResourceID: wire.SDMAmDataPath("imsi-" + roamerIMSI),
		Changes:    []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &path}},
	}}}); err != nil {
		t.Fatal(err)
	}

	f.amf.SettleHPLMN()

	if ue.State() != Registered || !f.amf.HoldsUE(roamerIMSI) {
		t.Fatal("a slice change deregistered the visitor")
	}

	if v, _ = ue.Visitor(); !v.Reregister || !v.ReregisterSent || sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("Configuration Update Command not sent: %+v, %d DL NAS Transports", v, sender.downlinkNasTransportCalls)
	}

	f.amf.ConfigurationUpdateCompleted(t.Context(), ue, conn)

	if v, _ = ue.Visitor(); v.Reregister || v.ReregisterSent {
		t.Fatal("the registration request is still pending after the UE completed it")
	}
}

func TestVisitorMSISDNComesFromTheHomeGPSI(t *testing.T) {
	f := newRoamingFixture(t)
	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) { s.AM.Gpsis = []string{"extid-x@example.org", "msisdn-15551230000"} })

	ue, _, _ := f.connectedVisitor(t)

	if got := f.db.visitors[roamerIMSI].Msisdn; got != "15551230000" {
		t.Fatalf("visitor MSISDN %q", got)
	}

	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) { s.AM.Gpsis = []string{"msisdn-15559990000"} })

	v, _ := ue.Visitor()
	path := "/gpsis"

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, wire.ModificationNotification{NotifyItems: []wire.NotifyItem{{
		ResourceID: wire.SDMAmDataPath("imsi-" + roamerIMSI),
		Changes:    []wire.ChangeItem{{Op: wire.ChangeReplace, Path: &path}},
	}}}); err != nil {
		t.Fatal(err)
	}

	f.amf.SettleHPLMN()

	f.db.mu.Lock()
	got := f.db.visitors[roamerIMSI].Msisdn
	f.db.mu.Unlock()

	if got != "15559990000" {
		t.Fatalf("visitor MSISDN after the change %q", got)
	}
}
