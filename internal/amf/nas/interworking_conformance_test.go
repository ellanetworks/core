// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func movingFromEPCRequest() *fgs.RegistrationRequest {
	return &fgs.RegistrationRequest{
		GMMCapability: &fgs.GMMCapability{},
		UEStatus:      &fgs.UEStatus{S1ModeReg: true},
	}
}

func handoverFollowUpRequest() *fgs.RegistrationRequest {
	req := nativeIdleArrivalRequest()
	req.EPSNASMessageContainer = nil

	return req
}

func arrivedByHandover(ue *amf.UeContext) {
	ue.MarkArrivedFromEPSHandover()
	ue.Conn().EPSArrival = &amf.EPSArrival{}
}

// TS 24.501 §5.5.1.3.4
func TestInterworkingRegistrationFromEPCIsTreatedAsInitial(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := amfInstance.LookupUeBySupi(ue.Supi()); !ok {
		t.Fatal("the UE context was never committed, so no PDU session it names can be moved")
	}
}

func TestInterworkingRegistrationWithoutUEStatusStaysOnTheMobilityPath(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := &fgs.RegistrationRequest{GMMCapability: &fgs.GMMCapability{}}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	ue.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := ue.SmContextFindByPDUSessionID(1); !ok {
		t.Error("the UE's PDU session was torn down: an ordinary mobility update was routed through the initial-registration path")
	}
}

func acceptFromRegistration(t *testing.T, ue *amf.UeContext, ngapSender *fakeNGAPSender) *fgs.RegistrationAccept {
	t.Helper()

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("REGISTRATION ACCEPT count = %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("message type = %#x, want a REGISTRATION ACCEPT", nm[2])
	}

	accept, err := fgs.ParseRegistrationAccept(nm)
	if err != nil {
		t.Fatalf("parse the REGISTRATION ACCEPT: %v", err)
	}

	return accept
}

// TS 24.501 §5.5.1.3.4
func TestInterworkingRegistrationAcceptReportsSessionsTheAMFDoesNotHold(t *testing.T) {
	ue, ngapSender, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	req.PDUSessionStatus = &fgs.PSIBitmap{PSI: [16]bool{1: true}}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	for _, call := range smfStub.ReleaseSmContextCalls {
		t.Errorf("released SM context %q: the AMF holds none of this UE's sessions, so there is nothing to release", call.SmContextRef)
	}

	accept := acceptFromRegistration(t, ue, ngapSender)

	if accept.PDUSessionStatus == nil {
		t.Fatal("REGISTRATION ACCEPT omits the PDU session status IE: the UE named a session it believes survived the move from EPS and the AMF holds none, so it must answer with an all-zero status to make the UE release them and re-establish; unlike TS 24.301 §5.5.3.2.4 for 4G, TS 24.501 §5.5.1.3.4 excuses no answer when the network side holds nothing")
	}

	if accept.PDUSessionStatus.PSI[1] {
		t.Error("REGISTRATION ACCEPT reports PDU session 1 as active: the AMF holds no session for it, so the UE will keep sending on a session that reaches nothing")
	}
}

// TS 24.501 §5.5.1.3.4
func TestInterworkingRegistrationAcceptReportsSessionsThatSurvivedTheHandover(t *testing.T) {
	ue, ngapSender, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}
	arrivedByHandover(ue)

	req := movingFromEPCRequest()
	req.PDUSessionStatus = &fgs.PSIBitmap{PSI: [16]bool{1: true}}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	for _, call := range smfStub.ReleaseSmContextCalls {
		t.Errorf("released SM context %q: the UE reported the session active and the AMF holds it, so it survived the handover", call.SmContextRef)
	}

	accept := acceptFromRegistration(t, ue, ngapSender)

	if accept.PDUSessionStatus == nil {
		t.Fatal("REGISTRATION ACCEPT omits the PDU session status IE although the request carried one")
	}

	if !accept.PDUSessionStatus.PSI[1] {
		t.Error("REGISTRATION ACCEPT reports PDU session 1 as inactive: it moved with the handover and the AMF holds it, so the UE will tear down a session that is still up")
	}
}

// TS 24.501 §5.5.1.3.2
func TestInterworkingPeriodicUpdateWithUEStatusIsNotTreatedAsInitial(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypePeriodicUpdating

	ue.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := ue.SmContextFindByPDUSessionID(1); !ok {
		t.Error("the UE's PDU session was torn down: only an inter-system change from S1 mode carries the UE status IE, and a periodic update is never one, so this took the initial-registration path")
	}
}

func TestRegistrationAfterAnEPSHandoverKeepsTheMovedSessions(t *testing.T) {
	ue, _, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

	if err := ue.CreateSmContext(1, "moved-by-handover", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	arrivedByHandover(ue)

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	for _, call := range smfStub.ReleaseSmContextCalls {
		t.Errorf("released SM context %q: the handover moved it onto 5GS moments before", call.SmContextRef)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(1); !ok {
		t.Error("the AMF dropped the PDU session the handover moved")
	}
}

func openRegistration(t *testing.T, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest) {
	t.Helper()

	wire, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the registration request: %v", err)
	}

	if err := handleRegistrationRequestMessage(context.TODO(), amfInstance, ue, req, wire, true, false); err != nil {
		t.Fatalf("handleRegistrationRequestMessage: %v", err)
	}
}

func TestHandoverMarkOpensTheRegistrationThatFollowsIt(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.MarkArrivedFromEPSHandover()

	openRegistration(t, amfInstance, ue, handoverFollowUpRequest())

	if !ue.Conn().ArrivedFromEPS() {
		t.Error("the registration that follows a completed relocation was not taken as an arrival from EPS, so the UE's moved sessions are torn down")
	}
}

func TestHandoverMarkEndsWithTheRegistrationItServed(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.MarkArrivedFromEPSHandover()

	openRegistration(t, amfInstance, ue, handoverFollowUpRequest())
	ue.ClearRegistrationRequestData()

	openRegistration(t, amfInstance, ue, handoverFollowUpRequest())

	if ue.Conn().ArrivedFromEPS() {
		t.Error("a later registration on the same connection inherited the handover mark of one that had already completed")
	}
}

func TestIdleArrivalSupersedesALeftoverHandoverMark(t *testing.T) {
	ue, amfInstance, _, _ := idleArrivalUE(t)

	ue.MarkArrivedFromEPSHandover()

	req := idleArrivalRequest()
	openRegistration(t, amfInstance, ue, req)
	recoverContextFromEPS(context.TODO(), amfInstance, ue, req, false)

	conn := ue.Conn()

	if !conn.ArrivalNeedsSecurityModeControl() {
		t.Error("the idle arrival did not map the EPS context: the leftover mark stood in for a context the MME had to supply")
	}

	if conn.EPSArrival.ArrivingSessions() == nil {
		t.Error("the arrival carries no PDN connections, so the UE's sessions are left behind in EPS")
	}
}

func TestHandoverMarkOutlivesARegistrationThatDidNotComplete(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.MarkArrivedFromEPSHandover()

	openRegistration(t, amfInstance, ue, handoverFollowUpRequest())
	openRegistration(t, amfInstance, ue, handoverFollowUpRequest())

	if !ue.Conn().ArrivedFromEPS() {
		t.Error("the retry of a registration that did not complete lost the arrival, so the UE is put through an initial registration instead")
	}
}

func TestUnresolvedRegistrationBecomesServedByAuthenticating(t *testing.T) {
	for _, tc := range []struct {
		typ           fgs.RegistrationType
		supersedes    bool
		supersedesWhy string
	}{
		{fgs.RegistrationTypeInitial, true, "TS 24.501 §5.5.1.2.8 f): the old 5GMM context is deleted once authentication proves the UE genuine"},
		{fgs.RegistrationTypeMobilityUpdating, false, ""},
		{fgs.RegistrationTypePeriodicUpdating, false, ""},
	} {
		t.Run(tc.typ.String(), func(t *testing.T) {
			ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

			fresh := amf.NewUeContext()
			amfInstance.AttachUeConn(fresh, ue.Conn())
			fresh.Conn().RegistrationType5GS = tc.typ
			fresh.TransitionTo(amf.RegistrationInitiated)

			if _, ok := amfInstance.LookupUeBySupi(ue.Supi()); !ok {
				t.Fatal("fixture precondition: the incumbent context should be served")
			}

			supi := ue.Supi()
			fresh.SetSupi(supi)

			if isRegistrationUpdate(tc.typ) {
				amfInstance.CarrySubscriberSessions(fresh)
			}

			if err := amfInstance.CommitUEIdentity(t.Context(), fresh, amf.MintAuthProofForRegistrationCommit()); err != nil {
				t.Fatalf("CommitUEIdentity: %v", err)
			}

			held, ok := amfInstance.LookupUeBySupi(supi)
			if !ok {
				t.Fatal("the AMF serves no context for a subscriber it has just authenticated")
			}

			if tc.supersedes && held != fresh {
				t.Fatalf("LookupUeBySupi = %p, want the newly authenticated context %p: %s", held, fresh, tc.supersedesWhy)
			}
		})
	}
}

// TS 24.501 §5.5.1.2.8
func TestDeregisteredContextIsRetainedAsAHusk(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Deregister(t.Context())

	held, ok := amfInstance.LookupUeBySupi(ue.Supi())
	if !ok || held != ue {
		t.Fatal("the husk was dropped, so a re-registration cannot reuse its security context")
	}

	if !amfInstance.ServesUeContext(ue) {
		t.Error("ServesUeContext denies a context the AMF still holds")
	}
}

// TS 24.501 §5.5.1.3.2
func TestSupersedingRegistrationCarriesSessionsOnlyForAnUpdate(t *testing.T) {
	for _, tc := range []struct {
		typ     fgs.RegistrationType
		carried bool
	}{
		{fgs.RegistrationTypeMobilityUpdating, true},
		{fgs.RegistrationTypePeriodicUpdating, true},
		{fgs.RegistrationTypeInitial, false},
	} {
		t.Run(tc.typ.String(), func(t *testing.T) {
			incumbent, _, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

			if err := incumbent.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
				t.Fatalf("CreateSmContext: %v", err)
			}

			fresh := amf.NewUeContext()
			amfInstance.AttachUeConn(fresh, incumbent.Conn())
			fresh.Conn().RegistrationType5GS = tc.typ
			fresh.TransitionTo(amf.RegistrationInitiated)
			fresh.SetSupi(incumbent.Supi())

			if isRegistrationUpdate(tc.typ) {
				amfInstance.CarrySubscriberSessions(fresh)
			}

			if err := amfInstance.CommitUEIdentity(t.Context(), fresh,
				amf.MintAuthProofForRegistrationCommit()); err != nil {
				t.Fatalf("CommitUEIdentity: %v", err)
			}

			_, held := fresh.SmContextFindByPDUSessionID(1)
			if held != tc.carried {
				t.Fatalf("fresh context holds PDU session 1 = %v, want %v", held, tc.carried)
			}

			released := len(smfStub.ReleaseSmContextCalls) != 0
			if released == tc.carried {
				t.Fatalf("SMF release calls = %v, want released = %v", smfStub.ReleaseSmContextCalls, !tc.carried)
			}
		})
	}
}
