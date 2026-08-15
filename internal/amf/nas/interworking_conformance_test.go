// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// Conformance tests for 5GS interworking without N26
func movingFromEPCRequest() *fgs.RegistrationRequest {
	return &fgs.RegistrationRequest{
		GMMCapability: &fgs.GMMCapability{},
		UEStatus:      &fgs.UEStatus{S1ModeReg: true},
	}
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
	ue.MarkArrivedFromEPSHandover()

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

	ue.MarkArrivedFromEPSHandover()

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

// TS 23.502 §4.11.2.3
func TestIdleArrivalSpendsALeftoverHandoverMark(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.MarkArrivedFromEPSHandover()

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	conn.ArrivingFromEPS = &interworking.ArrivingSessions{}

	contextSetup(context.TODO(), amfInstance, ue, movingFromEPCRequest(), nil)

	conn.ArrivingFromEPS = nil
	conn.ArrivedFromEPS = false

	contextSetup(context.TODO(), amfInstance, ue, movingFromEPCRequest(), nil)

	if conn.ArrivedFromEPS {
		t.Error("an update the AMF holds no EPS arrival for was taken as one: the handover mark outlived the idle arrival that short-circuited it")
	}
}

// TS 24.501 §4.4.4.3
func TestUnresolvedRegistrationBecomesServedByAuthenticating(t *testing.T) {
	for _, typ := range []fgs.RegistrationType{
		fgs.RegistrationTypeInitial,
		fgs.RegistrationTypeMobilityUpdating,
		fgs.RegistrationTypePeriodicUpdating,
	} {
		t.Run(typ.String(), func(t *testing.T) {
			ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

			fresh := amf.NewUeContext()
			amfInstance.AttachUeConn(fresh, ue.Conn())
			fresh.Conn().RegistrationType5GS = typ
			fresh.TransitionTo(amf.RegistrationInitiated)

			if _, ok := amfInstance.LookupUeBySupi(ue.Supi()); !ok {
				t.Fatal("fixture precondition: the incumbent context should be served")
			}

			supi := ue.Supi()
			if err := amfInstance.AdoptAuthenticatedSupi(t.Context(), fresh, supi,
				amf.MintAuthProofForRegistrationCommit()); err != nil {
				t.Fatalf("AdoptAuthenticatedSupi: %v", err)
			}

			held, ok := amfInstance.LookupUeBySupi(supi)
			if !ok || held != fresh {
				t.Fatalf("LookupUeBySupi = (%p, %v), want the newly authenticated context %p", held, ok, fresh)
			}
		})
	}
}

// A deregistered context stays resolvable: the AMF keeps it as a husk with its
// security context so a later registration can reuse it and skip authentication,
// mirroring the MME's implicit-detach husk (TS 24.301 §4.4.2 / annex C). Callers that
// need a registered UE check the 5GMM state themselves.
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
